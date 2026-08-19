package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type deviceOptions struct {
	Config       *qtraceConfig
	WorkDir      string
	DiagMDLog    string
	Settle       time.Duration
	Drain        time.Duration
	Window       time.Duration
	Poll         time.Duration
	StopGrace    time.Duration
	KeepCaptures bool
	FileSizeMB   int
	FileCount    int
	Reports      *reportWriter

	lockFile *os.File
	mu       sync.Mutex
	active   *exec.Cmd
}

type captureOutcome struct {
	Metrics              captureMetrics
	FirstHitLatency      *time.Duration
	FirstHitIsUpperBound bool
	CaptureDuration      time.Duration
	CaptureDirectory     string
	DiagExit             string
}

func (options *deviceOptions) acquire() error {
	if options.Config == nil || options.Reports == nil {
		return errors.New("device runner is not initialized")
	}
	if options.DiagMDLog == "" {
		options.DiagMDLog = "diag_mdlog"
	}
	if options.FileSizeMB <= 0 {
		options.FileSizeMB = 4
	}
	if options.FileCount <= 0 {
		options.FileCount = 32
	}
	if options.Poll <= 0 {
		options.Poll = 250 * time.Millisecond
	}
	if options.StopGrace <= 0 {
		options.StopGrace = 2 * time.Second
	}
	if err := os.MkdirAll(options.WorkDir, 0700); err != nil {
		return err
	}
	lockPath := filepath.Join(options.WorkDir, "reducer.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		owner, _ := os.ReadFile(lockPath)
		_ = lock.Close()
		return fmt.Errorf("another reducer owns %s (%s): %w", lockPath, strings.TrimSpace(string(owner)), err)
	}
	if err := lock.Truncate(0); err != nil {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
		return err
	}
	if _, err := lock.Seek(0, 0); err != nil {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
		return err
	}
	options.lockFile = lock
	_, _ = fmt.Fprintf(lock, "pid=%d\nstarted=%s\n", os.Getpid(), time.Now().Format(time.RFC3339Nano))
	_ = lock.Sync()
	if err := options.ensureNoDiagMDLog(); err != nil {
		options.release()
		return err
	}
	return nil
}

func (options *deviceOptions) release() {
	options.stopActive()
	if options.lockFile != nil {
		_ = syscall.Flock(int(options.lockFile.Fd()), syscall.LOCK_UN)
		_ = options.lockFile.Close()
		options.lockFile = nil
	}
}

func (options *deviceOptions) ensureNoDiagMDLog() error {
	out, err := exec.Command("pidof", "diag_mdlog").Output()
	if err != nil && len(out) == 0 {
		return nil
	}
	var active []string
	for _, field := range strings.Fields(string(out)) {
		pid, parseErr := strconv.Atoi(field)
		if parseErr != nil || pid == os.Getpid() {
			continue
		}
		cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		active = append(active, fmt.Sprintf("pid=%d cmd=%s", pid, strings.TrimSpace(strings.ReplaceAll(string(cmdline), "\x00", " "))))
	}
	if len(active) != 0 {
		return fmt.Errorf("diag_mdlog is already running; stop it explicitly before reducer: %s", strings.Join(active, "; "))
	}
	return nil
}

func (options *deviceOptions) stopActive() {
	options.mu.Lock()
	command := options.active
	options.mu.Unlock()
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Signal(syscall.SIGTERM)
}

func (options *deviceOptions) cleanup(ctx context.Context, label string) error {
	zero, err := options.Config.zeroMessageFrames()
	if err != nil {
		return err
	}
	_, err = options.runDiagCapture(ctx, "cleanup-"+label, zero, targetCombined, 0, options.Drain, nil)
	return err
}

func (options *deviceOptions) testCandidate(
	ctx context.Context,
	stage, control string,
	target targetKind,
	frameIDs []int,
	zeroMasks bool,
	private privateMode,
	privateState string,
	attempt int,
	positiveOK bool,
	window time.Duration,
) (runRecord, error) {
	if err := options.ensureNoDiagMDLog(); err != nil {
		return runRecord{}, err
	}
	if err := options.cleanup(ctx, "before"); err != nil {
		return runRecord{}, fmt.Errorf("pre-run zero-mask cleanup failed: %w", err)
	}
	frames, err := options.Config.selectFrames(frameIDs, zeroMasks, private)
	if err != nil {
		return runRecord{}, err
	}
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), options.Drain+options.StopGrace+5*time.Second)
			_ = options.cleanup(cleanupCtx, "deferred")
			cancel()
		}
	}()
	prefix := stage
	if control != "" {
		prefix += "-" + control
	}
	runID := options.Reports.nextID(prefix)
	servingCtx, servingCancel := context.WithTimeout(ctx, 4*time.Second)
	serving := readServingIdentities(servingCtx)
	servingCancel()
	outcome, captureErr := options.runDiagCapture(ctx, runID, frames, target, options.Settle, window, serving)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), options.Drain+options.StopGrace+5*time.Second)
	cleanupErr := options.cleanup(cleanupCtx, "after-"+runID)
	cancel()
	cleanupNeeded = cleanupErr != nil
	if privateState == "" {
		privateState = string(private)
	}
	record := runRecord{
		RunID: runID, Timestamp: time.Now().UTC(), Stage: stage, ControlGroup: control,
		Target: target, Attempt: attempt, CandidateFrameIDs: append([]int(nil), frameIDs...),
		CandidateSSIDs: options.Config.allSSIDs(frameIDs), CandidateMaskBits: []int{},
		PrivateCommandState: privateState, PositiveControlStatus: positiveOK,
		CaptureDurationMS:    outcome.CaptureDuration.Milliseconds(),
		FirstHitIsUpperBound: outcome.FirstHitIsUpperBound,
		QSHFrameCount:        outcome.Metrics.QSHFrameCount, QSHTotalBytes: outcome.Metrics.QSHTotalBytes,
		TargetHashCount: outcome.Metrics.TargetHashCount, ParseSuccessCount: outcome.Metrics.ParseSuccessCount,
		ParseErrorCount: outcome.Metrics.ParseErrorCount, NRCellCount: outcome.Metrics.NRCellCount,
		LTECellCount: outcome.Metrics.LTECellCount, CapturedBytes: outcome.Metrics.CapturedBytes,
		MalformedFrames: outcome.Metrics.MalformedFrames, Cells: outcome.Metrics.Cells,
		CaptureDirectory: outcome.CaptureDirectory, DiagExit: outcome.DiagExit,
		MaskApplyReference: "end of configured settle interval",
	}
	if outcome.FirstHitLatency != nil {
		latency := outcome.FirstHitLatency.Milliseconds()
		record.FirstHitLatencyMS = &latency
	}
	record.Result = captureErr == nil && cleanupErr == nil && outcome.Metrics.matches(target)
	switch {
	case captureErr != nil:
		record.FailureReason = captureErr.Error()
	case cleanupErr != nil:
		record.FailureReason = "post-run zero-mask cleanup failed: " + cleanupErr.Error()
	case !outcome.Metrics.matches(target):
		record.FailureReason = "no complete, valid target cell in this capture window"
	}
	if err := options.Reports.Write(record); err != nil {
		return record, err
	}
	return record, errors.Join(captureErr, cleanupErr)
}

func (options *deviceOptions) runDiagCapture(
	ctx context.Context,
	name string,
	frames []diagFrame,
	target targetKind,
	settle, window time.Duration,
	serving []servingIdentity,
) (captureOutcome, error) {
	if err := options.ensureNoDiagMDLog(); err != nil {
		return captureOutcome{}, err
	}
	runDir := filepath.Join(options.WorkDir, "captures", sanitizeName(name))
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return captureOutcome{}, err
	}
	configPath := filepath.Join(runDir, "candidate.cfg")
	configData := encodeConfig(frames)
	if _, err := parseQTraceConfigBytes(configData); err != nil {
		return captureOutcome{}, fmt.Errorf("generated candidate failed HDLC/CRC validation: %w", err)
	}
	if err := writeFileAtomic(configPath, configData, 0600); err != nil {
		return captureOutcome{}, err
	}
	logFile, err := os.OpenFile(filepath.Join(runDir, "diag-mdlog.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return captureOutcome{}, err
	}
	command := exec.Command(options.DiagMDLog, options.diagArguments(configPath, runDir)...)
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return captureOutcome{}, err
	}
	options.mu.Lock()
	options.active = command
	options.mu.Unlock()
	exitCh := make(chan error, 1)
	go func() { exitCh <- command.Wait() }()
	settleTimer := time.NewTimer(settle)
	select {
	case <-ctx.Done():
		settleTimer.Stop()
		options.terminateCommand(command, exitCh)
		_ = logFile.Close()
		return captureOutcome{}, ctx.Err()
	case err := <-exitCh:
		settleTimer.Stop()
		options.clearActive(command)
		_ = logFile.Close()
		return captureOutcome{}, fmt.Errorf("diag_mdlog exited during settle: %s", formatExit(err))
	case <-settleTimer.C:
	}
	captureStart := time.Now()
	baseline := snapshotQMDLSizes(runDir)
	deadline := captureStart.Add(window)
	poll := time.NewTicker(options.Poll)
	defer poll.Stop()
	var firstHit *time.Duration
	var latest captureMetrics
	var diagExit string
	running := true
	exitedEarly := false
	for running && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			options.terminateCommand(command, exitCh)
			options.clearActive(command)
			_ = logFile.Close()
			return captureOutcome{}, ctx.Err()
		case err := <-exitCh:
			running = false
			exitedEarly = true
			diagExit = formatExit(err)
		case <-poll.C:
			metrics, parseErr := analyzeQMDLWindow(ctx, findQMDLFiles(runDir), baseline, serving, target)
			if parseErr == nil {
				latest = metrics
				if firstHit == nil && metrics.matches(target) {
					elapsed := time.Since(captureStart)
					firstHit = &elapsed
				}
			}
		}
	}
	if running {
		diagExit = options.terminateCommand(command, exitCh)
	}
	options.clearActive(command)
	_ = logFile.Close()
	finalCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	final, parseErr := analyzeQMDLWindow(finalCtx, findQMDLFiles(runDir), baseline, serving, target)
	cancel()
	if parseErr == nil {
		latest = final
	}
	upperBound := false
	if firstHit == nil && latest.matches(target) {
		elapsed := window
		firstHit = &elapsed
		upperBound = true
	}
	outcome := captureOutcome{
		Metrics: latest, FirstHitLatency: firstHit, FirstHitIsUpperBound: upperBound,
		CaptureDuration: window, CaptureDirectory: runDir, DiagExit: diagExit,
	}
	if !options.KeepCaptures {
		_ = os.RemoveAll(runDir)
		outcome.CaptureDirectory = ""
	}
	if parseErr != nil {
		return outcome, parseErr
	}
	if exitedEarly {
		return outcome, fmt.Errorf("diag_mdlog exited before capture window ended: %s", diagExit)
	}
	return outcome, nil
}

func (options *deviceOptions) diagArguments(configPath, runDir string) []string {
	return []string{
		"-f", configPath,
		"-o", runDir,
		"-s", strconv.Itoa(options.FileSizeMB),
		"-n", strconv.Itoa(options.FileCount),
		"-d",
	}
}

func (options *deviceOptions) terminateCommand(command *exec.Cmd, exitCh <-chan error) string {
	if command == nil || command.Process == nil {
		return "not-started"
	}
	_ = command.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(options.StopGrace)
	defer timer.Stop()
	select {
	case err := <-exitCh:
		return formatExit(err)
	case <-timer.C:
		_ = command.Process.Kill()
		err := <-exitCh
		return "forced-kill: " + formatExit(err)
	}
}

func (options *deviceOptions) clearActive(command *exec.Cmd) {
	options.mu.Lock()
	if options.active == command {
		options.active = nil
	}
	options.mu.Unlock()
}

func formatExit(err error) string {
	if err == nil {
		return "exit=0"
	}
	return err.Error()
}

func findQMDLFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "diag_log_") || !strings.HasSuffix(entry.Name(), ".qmdl") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths
}

func snapshotQMDLSizes(dir string) map[string]int64 {
	sizes := make(map[string]int64)
	for _, path := range findQMDLFiles(dir) {
		if info, err := os.Stat(path); err == nil {
			sizes[path] = info.Size()
		}
	}
	return sizes
}

func sanitizeName(value string) string {
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, value)
	return strings.Trim(value, "-")
}

func readServingIdentities(ctx context.Context) []servingIdentity {
	out, err := exec.CommandContext(ctx, "ubus", "call", "zte_nwinfo_api", "nwinfo_get_netinfo", "{}").Output()
	if err != nil {
		return nil
	}
	var snapshot map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(out)))
	decoder.UseNumber()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil
	}
	var cells []servingIdentity
	if pci, ok := mapUint(snapshot, "lte_pci"); ok && pci <= 503 {
		arfcn, _ := mapUint(snapshot, "wan_active_channel")
		cells = append(cells, servingIdentity{RAT: "LTE", PCI: pci, ARFCN: arfcn})
	}
	if pci, ok := mapUint(snapshot, "nr5g_pci"); ok && pci <= 1007 {
		arfcn, _ := mapUint(snapshot, "nr5g_action_channel")
		cells = append(cells, servingIdentity{RAT: "NR", PCI: pci, ARFCN: arfcn})
	}
	return cells
}

func mapUint(values map[string]any, key string) (uint32, bool) {
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "-" || strings.EqualFold(text, "null") {
		return 0, false
	}
	parsed, err := strconv.ParseUint(text, 10, 32)
	return uint32(parsed), err == nil
}
