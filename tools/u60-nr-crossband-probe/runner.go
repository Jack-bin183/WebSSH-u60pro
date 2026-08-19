package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type runOptions struct {
	Experiment       experimentID
	Target           targetSpec
	ConfigPath       string
	WorkDir          string
	StatePath        string
	DiagMDLog        string
	UBUS             string
	QCRILStart       string
	QCRILStop        string
	QMITracePath     string
	Window           time.Duration
	Settle           time.Duration
	AcquisitionPause time.Duration
	Poll             time.Duration
	StopGrace        time.Duration
	FileSizeMB       int
	FileCount        int
	MinMemoryBytes   uint64
	MinWorkFreeBytes uint64
	KeepCaptures     bool
}

type diagProcess struct {
	command *exec.Cmd
	exit    chan error
	log     *os.File
	mu      sync.Mutex
}

func runExperimentCommand(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	experimentValue := flags.String("experiment", "A", "experiment A, B, C, D, E, or F")
	configPath := flags.String("config", "qtrace.cfg", "production minimal qtrace.cfg")
	workDir := flags.String("work-dir", "/tmp/u60-nr-crossband-probe", "work and report directory")
	statePath := flags.String("state", defaultStatePath, "transaction recovery journal")
	diagMDLog := flags.String("diag-mdlog", "diag_mdlog", "diag_mdlog executable")
	ubus := flags.String("ubus", "ubus", "ubus executable")
	qcrilStart := flags.String("qcril-start-command", "", "experiment B external QCRIL socket/HAL driver command")
	qcrilStop := flags.String("qcril-stop-command", "", "experiment B external stop command")
	qmiTracePath := flags.String("qmi-trace-log", "", "optional append-only CCI hook log to correlate with this run")
	window := flags.Duration("window", 8*time.Second, "maximum measurement window")
	settle := flags.Duration("settle", 400*time.Millisecond, "QSH configuration settle time")
	acquisitionPause := flags.Duration("acquisition-pause", 700*time.Millisecond, "NR-off interval for experiment E")
	poll := flags.Duration("poll", 100*time.Millisecond, "QMDL and resource poll interval")
	stopGrace := flags.Duration("stop-grace", 2*time.Second, "diag_mdlog SIGTERM grace")
	fileSize := flags.Int("file-size-mb", 2, "QMDL ring file size, 1..4 MiB")
	fileCount := flags.Int("file-count", 2, "QMDL ring file count, 1..4")
	minMemory := flags.Uint64("min-memory-mb", 32, "abort below this MemAvailable value")
	minWorkFree := flags.Uint64("min-work-free-mb", 16, "abort below this free space in work directory")
	keepCaptures := flags.Bool("keep-captures", false, "retain raw QMDL after extracting results")
	confirmActive := flags.Bool("confirm-active-measurement", false, "confirm a bounded modem measurement")
	confirmNetwork := flags.Bool("confirm-network-change", false, "confirm temporary network changes for C-F")
	band, arfcn, pci := targetFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	experiment, err := parseExperiment(*experimentValue)
	if err != nil {
		return err
	}
	target, err := parseTarget(*band, *arfcn, *pci)
	if err != nil {
		return err
	}
	if !*confirmActive {
		return errors.New("run requires -confirm-active-measurement")
	}
	if experiment >= experimentC && !*confirmNetwork {
		return fmt.Errorf("experiment %s changes network selection and requires -confirm-network-change", experiment)
	}
	if experiment == experimentB && strings.TrimSpace(*qcrilStart) == "" {
		return errors.New("experiment B requires -qcril-start-command; no unverified socket framing is built in")
	}
	if experiment == experimentF && target.PCI == nil {
		return errors.New("experiment F requires -target-pci from a known successful n41 registration")
	}
	if *window < time.Second || *window > 30*time.Second {
		return errors.New("window must be between 1s and 30s")
	}
	if *fileSize < 1 || *fileSize > 4 || *fileCount < 1 || *fileCount > 4 {
		return errors.New("file-size-mb and file-count must each be between 1 and 4")
	}
	options := runOptions{
		Experiment: experiment, Target: target, ConfigPath: *configPath, WorkDir: *workDir,
		StatePath: *statePath, DiagMDLog: *diagMDLog, UBUS: *ubus,
		QCRILStart: *qcrilStart, QCRILStop: *qcrilStop, QMITracePath: *qmiTracePath, Window: *window,
		Settle: *settle, AcquisitionPause: *acquisitionPause, Poll: *poll, StopGrace: *stopGrace,
		FileSizeMB: *fileSize, FileCount: *fileCount,
		MinMemoryBytes: *minMemory * 1024 * 1024, MinWorkFreeBytes: *minWorkFree * 1024 * 1024,
		KeepCaptures: *keepCaptures,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	record, runErr := runExperiment(ctx, options)
	if record.RunID != "" {
		if reportErr := appendExperimentReports(options.WorkDir, record); reportErr != nil {
			runErr = errors.Join(runErr, reportErr)
		}
	}
	return runErr
}

func runExperiment(ctx context.Context, options runOptions) (record experimentRecord, returnErr error) {
	if err := os.MkdirAll(options.WorkDir, 0700); err != nil {
		return record, err
	}
	lock, err := acquireProbeLock(filepath.Join(options.WorkDir, "probe.lock"))
	if err != nil {
		return record, err
	}
	defer releaseProbeLock(lock)
	if state, pending, err := pendingRecovery(options.StatePath); err != nil {
		return record, fmt.Errorf("read recovery state: %w", err)
	} else if pending {
		return record, fmt.Errorf("unfinished run %s is in stage %s; run recover first", state.RunID, state.Stage)
	}
	if err := ensureNoDiagMDLog(); err != nil {
		return record, err
	}
	if err := resourcePreflight(options); err != nil {
		return record, err
	}

	runID := nowRunID(options.Experiment)
	runDir := filepath.Join(options.WorkDir, "captures", runID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return record, err
	}
	activeConfig := filepath.Join(runDir, "qtrace-active.cfg")
	if _, err := buildActiveQTrace(options.ConfigPath, activeConfig); err != nil {
		return record, err
	}
	productionQTrace, err := os.ReadFile(options.ConfigPath)
	if err != nil {
		return record, fmt.Errorf("read production qtrace for recovery journal: %w", err)
	}
	controller := &deviceController{UBUS: options.UBUS, StatePath: options.StatePath}
	snapshotCtx, cancelSnapshot := context.WithTimeout(ctx, 10*time.Second)
	snapshot, err := controller.snapshot(snapshotCtx)
	cancelSnapshot()
	if err != nil {
		return record, fmt.Errorf("snapshot network state: %w", err)
	}
	record = experimentRecord{
		Schema: stateSchema, ToolVersion: toolVersion, RunID: runID, Experiment: options.Experiment,
		Description: experimentDescription(options.Experiment), StartedAt: time.Now().UTC(),
		ServingBefore: snapshot, Target: options.Target, CaptureDirectory: runDir,
		NetworkInterrupted: options.Experiment >= experimentC,
		QCRILPathVerified:  false,
		QCRILPathEvidence:  "U60 libqcrilNr: ONE_SHOT keeps only the current RAT mask; advanced carries Band/channel in NAS 0x0085; setSystemSelectionChannels maps Band only to NAS 0x0033",
		QMITracePath:       options.QMITracePath, QMITraceComplete: options.Experiment == experimentA,
	}
	controller.Actions = &record.Actions
	controller.QMIEvents = &record.QMIEvents
	state := &recoveryState{
		Schema: stateSchema, ToolVersion: toolVersion, RunID: runID, Experiment: options.Experiment,
		Target: options.Target, CreatedAt: time.Now().UTC(), Status: "running", Stage: "snapshot-saved", Snapshot: snapshot,
		NetworkRestoreRequired: options.Experiment >= experimentC,
		ProductionQTraceBase64: base64.StdEncoding.EncodeToString(productionQTrace), DiagMDLog: options.DiagMDLog,
		WorkDir: options.WorkDir, StopGraceMS: options.StopGrace.Milliseconds(),
	}
	if err := controller.writeState(state); err != nil {
		return record, err
	}
	networkChanged := options.Experiment >= experimentC
	traceApplied := false
	var process *diagProcess
	defer func() {
		if process != nil {
			_ = process.stop(options.StopGrace)
		}
		if traceApplied {
			restoreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := restoreProductionTrace(restoreCtx, options, runDir, &record.Actions)
			cancel()
			if err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("restore production QSH mask: %w", err))
			} else {
				traceApplied = false
				state.TraceRestoreRequired = false
				_ = controller.writeState(state)
			}
		}
		if _, err := os.Stat(options.StatePath); err == nil {
			if networkChanged {
				restoreCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				err := controller.restoreNetwork(restoreCtx, state)
				cancel()
				if err != nil {
					returnErr = errors.Join(returnErr, fmt.Errorf("deferred network restore: %w", err))
				} else if !state.TraceRestoreRequired {
					_ = os.Remove(options.StatePath)
				}
			} else if !state.TraceRestoreRequired {
				_ = os.Remove(options.StatePath)
			}
		}
	}()

	qmdlDir := filepath.Join(runDir, "qmdl")
	state.TraceRestoreRequired = true
	state.Stage = "active-trace-starting"
	if err := controller.writeState(state); err != nil {
		return record, err
	}
	traceApplied = true
	process, err = startDiagMDLog(options, activeConfig, qmdlDir, filepath.Join(runDir, "diag-mdlog.log"))
	if err != nil {
		state.Status, state.Stage, state.LastError = "failed", "diag-start", err.Error()
		_ = controller.writeState(state)
		return record, err
	}
	state.Stage = "active-trace-running"
	_ = controller.writeState(state)
	if err := waitContext(ctx, options.Settle); err != nil {
		return record, err
	}

	actionCtx, cancelAction := context.WithCancel(ctx)
	actionDone := make(chan error, 1)
	qmiTraceOffset := traceFileOffset(options.QMITracePath)
	startTime := time.Now()
	startExperimentAction(actionCtx, options, snapshot, controller, state, actionDone)

	monitorTicker := time.NewTicker(options.Poll)
	defer monitorTicker.Stop()
	deadline := time.NewTimer(options.Window)
	defer deadline.Stop()
	var metrics qmdlMetrics
	var actionErr error
	var failure string
	monitoring := true
	for monitoring {
		select {
		case <-ctx.Done():
			failure = ctx.Err().Error()
			monitoring = false
		case err := <-actionDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				actionErr = err
				failure = "experiment action failed: " + err.Error()
				monitoring = false
			}
			actionDone = nil
		case <-deadline.C:
			failure = "measurement window expired without a valid target result"
			monitoring = false
		case <-monitorTicker.C:
			sample, sampleErr := sampleResources(options, qmdlDir)
			if sampleErr != nil {
				failure = "resource monitoring failed: " + sampleErr.Error()
				monitoring = false
				continue
			}
			record.ResourceSamples = append(record.ResourceSamples, sample)
			if sample.MemAvailableBytes < options.MinMemoryBytes || sample.WorkFreeBytes < options.MinWorkFreeBytes {
				failure = "low-memory/free-space fuse triggered"
				monitoring = false
				continue
			}
			current, parseErr := analyzeQMDLFiles(findQMDLFiles(qmdlDir), options.Target)
			if parseErr == nil {
				metrics = current
				if hasTargetResult(metrics.Results, options.Target) {
					latency := time.Since(startTime).Milliseconds()
					record.FirstTargetHitMS = &latency
					monitoring = false
				}
			}
		}
	}
	cancelAction()
	if actionDone != nil {
		select {
		case err := <-actionDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				actionErr = err
			}
		case <-time.After(4 * time.Second):
			actionErr = errors.Join(actionErr, errors.New("action cleanup timed out"))
		}
	}
	if options.QMITracePath != "" {
		traceEvents, traceReadErr := readCCITraceEvents(options.QMITracePath, qmiTraceOffset)
		if traceReadErr == nil {
			traceReadErr = validateCCITraceWindow(options.QMITracePath, traceEvents)
		}
		if traceReadErr != nil {
			record.QMITraceError = traceReadErr.Error()
		} else {
			record.QMITraceComplete = true
			record.QMIEvents = append(record.QMIEvents, traceEvents...)
			if options.Experiment == experimentB && hasQMIRequest(traceEvents, "3", "0x0085") {
				record.QCRILPathVerified = true
			}
		}
	}
	if process != nil {
		_ = process.stop(options.StopGrace)
		process = nil
	}
	finalMetrics, parseErr := analyzeQMDLFiles(findQMDLFiles(qmdlDir), options.Target)
	if parseErr == nil {
		metrics = finalMetrics
	}
	for index := range metrics.Results {
		observedAt := time.Now().UTC()
		metrics.Results[index].ObservedAt = &observedAt
	}
	record.ML1Results = metrics.Results
	record.QSHEvidence = metrics.Evidence
	record.QSHFrameCount = metrics.QSHFrameCount
	record.ActiveHashCount = metrics.ActiveHashCount
	record.MalformedFrameCount = metrics.MalformedFrameCount
	record.QMDLBytes = metrics.QMDLBytes
	record.CaptureDurationMS = time.Since(startTime).Milliseconds()

	traceRestoreCtx, cancelTraceRestore := context.WithTimeout(context.Background(), 10*time.Second)
	traceErr := restoreProductionTrace(traceRestoreCtx, options, runDir, &record.Actions)
	cancelTraceRestore()
	traceApplied = traceErr != nil
	if traceErr == nil {
		state.TraceRestoreRequired = false
		state.CompletedActions = append(state.CompletedActions, "production-qsh-mask-restored")
		state.Stage = "trace-restored"
		_ = controller.writeState(state)
	}

	restoreStart := time.Now()
	var restoreErr error
	if networkChanged {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		restoreErr = controller.restoreNetwork(restoreCtx, state)
		cancel()
	} else {
		state.Stage = "network-unchanged"
		_ = controller.writeState(state)
	}
	if restoreErr == nil && traceErr == nil {
		state.Status, state.Stage, state.LastError = "restored", "complete", ""
		if stateErr := controller.writeState(state); stateErr != nil {
			restoreErr = stateErr
		} else {
			restoreErr = os.Remove(options.StatePath)
			if errors.Is(restoreErr, os.ErrNotExist) {
				restoreErr = nil
			}
		}
	}
	record.RestoreDurationMS = time.Since(restoreStart).Milliseconds()
	record.RestoreSucceeded = restoreErr == nil && traceErr == nil
	afterCtx, cancelAfter := context.WithTimeout(context.Background(), 8*time.Second)
	if after, afterErr := controller.snapshot(afterCtx); afterErr == nil {
		record.ServingAfterRestore = &after
	}
	cancelAfter()

	if hasTargetResult(record.ML1Results, options.Target) && actionErr == nil && parseErr == nil && restoreErr == nil && traceErr == nil {
		record.Result = "success"
	} else {
		record.Result = "failed"
		switch {
		case failure != "":
			record.FailureReason = failure
		case actionErr != nil:
			record.FailureReason = actionErr.Error()
		case parseErr != nil:
			record.FailureReason = parseErr.Error()
		default:
			record.FailureReason = "no valid target ML1 result"
		}
	}
	if actionErr != nil && record.FailureReason == "" {
		record.FailureReason = actionErr.Error()
	}
	if restoreErr != nil {
		record.FailureReason = joinReason(record.FailureReason, "network restore failed: "+restoreErr.Error())
	}
	if traceErr != nil {
		record.FailureReason = joinReason(record.FailureReason, "QSH mask restore failed: "+traceErr.Error())
	}
	record.FinishedAt = time.Now().UTC()
	if !options.KeepCaptures {
		_ = os.RemoveAll(qmdlDir)
		record.CaptureDirectory = ""
	}
	var measurementErr error
	if record.Result != "success" {
		measurementErr = errors.New(record.FailureReason)
	}
	return record, errors.Join(measurementErr, actionErr, parseErr, restoreErr, traceErr)
}

func startExperimentAction(ctx context.Context, options runOptions, snapshot networkSnapshot, controller *deviceController, state *recoveryState, done chan<- error) {
	go func() {
		var err error
		switch options.Experiment {
		case experimentA:
			err = runRawNRScan(ctx, []uint32{options.Target.ARFCN}, 2, func(event qmiEvent) {
				// The runner owns this slice for the duration of this goroutine and
				// only reads it after cancellation/exit.
				controller.appendQMI(event)
			})
		case experimentB:
			err = runExternalQCRIL(ctx, options, controller)
		case experimentC:
			err = controller.setNRBand(ctx, nrBandKind(snapshot), strconv.FormatUint(uint64(options.Target.Band), 10))
			markRecoveryAction(controller, state, "target-band-applied", err)
		case experimentD:
			err = controller.setNRBand(ctx, nrBandKind(snapshot), strconv.FormatUint(uint64(options.Target.Band), 10))
			markRecoveryAction(controller, state, "target-band-applied", err)
			if err == nil {
				mode := snapshot.NetSelect
				if mode == "" {
					mode = "WL_AND_5G"
				}
				err = controller.setNetSelect(ctx, mode)
				markRecoveryAction(controller, state, "automatic-reselection-requested", err)
			}
		case experimentE:
			err = controller.setNetSelect(ctx, "Only_LTE")
			markRecoveryAction(controller, state, "nr-temporarily-disabled", err)
			if err == nil {
				err = waitContext(ctx, options.AcquisitionPause)
			}
			if err == nil {
				err = controller.setNRBand(ctx, "SA", strconv.FormatUint(uint64(options.Target.Band), 10))
				markRecoveryAction(controller, state, "target-band-applied", err)
			}
			if err == nil {
				err = controller.setNetSelect(ctx, "Only_5G")
				markRecoveryAction(controller, state, "target-nr-acquisition-started", err)
			}
		case experimentF:
			err = controller.setNRCellLock(ctx, strconv.FormatUint(uint64(*options.Target.PCI), 10), strconv.FormatUint(uint64(options.Target.ARFCN), 10), strconv.FormatUint(uint64(options.Target.Band), 10))
			markRecoveryAction(controller, state, "known-target-cell-lock-applied", err)
			if err == nil {
				err = controller.setNetSelect(ctx, "Only_5G")
				markRecoveryAction(controller, state, "positive-control-acquisition-started", err)
			}
		}
		done <- err
	}()
}

func runExternalQCRIL(ctx context.Context, options runOptions, controller *deviceController) error {
	command := exec.CommandContext(ctx, "/bin/sh", "-c", options.QCRILStart)
	command.Env = append(os.Environ(),
		"U60_TARGET_RAT=NGRAN",
		"U60_TARGET_BAND="+strconv.FormatUint(uint64(options.Target.Band), 10),
		"U60_TARGET_ARFCN="+strconv.FormatUint(uint64(options.Target.ARFCN), 10),
		"U60_SCAN_TYPE=ONE_SHOT",
	)
	output, err := command.CombinedOutput()
	event := actionEvent{At: time.Now().UTC(), Action: "external-qcril-start", Request: map[string]any{
		"band": options.Target.Band, "arfcn": options.Target.ARFCN, "type": "ONE_SHOT",
	}, Response: map[string]any{"output": strings.TrimSpace(string(output))}}
	if err != nil && !errors.Is(err, context.Canceled) {
		event.Error = err.Error()
	}
	if controller.Actions != nil {
		*controller.Actions = append(*controller.Actions, event)
	}
	if err == nil {
		// A normal start command is expected to return after submitting the
		// request. Keep the scan alive until the QMDL monitor finds a target or
		// reaches its bounded deadline, then issue the separate stop command.
		<-ctx.Done()
	} else if ctx.Err() != nil {
		// A blocking external driver being canceled at the probe boundary is
		// expected. Its explicit stop command below still has to succeed.
		err = nil
	}
	if strings.TrimSpace(options.QCRILStop) != "" {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		stopOutput, stopErr := exec.CommandContext(stopCtx, "/bin/sh", "-c", options.QCRILStop).CombinedOutput()
		cancel()
		stopEvent := actionEvent{At: time.Now().UTC(), Action: "external-qcril-stop", Response: map[string]any{"output": strings.TrimSpace(string(stopOutput))}}
		if stopErr != nil {
			stopEvent.Error = stopErr.Error()
		}
		if controller.Actions != nil {
			*controller.Actions = append(*controller.Actions, stopEvent)
		}
		err = errors.Join(err, stopErr)
	}
	return err
}

func markRecoveryAction(controller *deviceController, state *recoveryState, action string, err error) {
	if err != nil {
		state.LastError = err.Error()
		state.Stage = action + "-failed"
	} else {
		state.CompletedActions = append(state.CompletedActions, action)
		state.Stage = action
	}
	_ = controller.writeState(state)
}

func nrBandKind(snapshot networkSnapshot) string {
	if strings.Contains(strings.ToUpper(snapshot.NetworkType), "NSA") {
		return "NSA"
	}
	return "SA"
}

func experimentDescription(experiment experimentID) string {
	for _, plan := range defaultExperimentPlans() {
		if plan.ID == experiment {
			return plan.ControlPath
		}
	}
	return ""
}

func startDiagMDLog(options runOptions, configPath, outputDir, logPath string) (*diagProcess, error) {
	if err := ensureNoDiagMDLog(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	command := exec.Command(options.DiagMDLog,
		"-f", configPath, "-o", outputDir,
		"-s", strconv.Itoa(options.FileSizeMB), "-n", strconv.Itoa(options.FileCount), "-d")
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	process := &diagProcess{command: command, exit: make(chan error, 1), log: logFile}
	go func() { process.exit <- command.Wait() }()
	return process, nil
}

func (process *diagProcess) stop(grace time.Duration) error {
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.command == nil || process.command.Process == nil {
		return nil
	}
	_ = process.command.Process.Signal(syscall.SIGTERM)
	var err error
	select {
	case err = <-process.exit:
	case <-time.After(grace):
		_ = process.command.Process.Kill()
		err = <-process.exit
	}
	if process.log != nil {
		_ = process.log.Close()
	}
	process.command = nil
	if exitError := new(exec.ExitError); errors.As(err, &exitError) {
		// SIGTERM is the expected bounded-capture exit.
		return nil
	}
	return err
}

func restoreProductionTrace(ctx context.Context, options runOptions, runDir string, actions *[]actionEvent) error {
	if err := ensureNoDiagMDLog(); err != nil {
		return err
	}
	restoreDir := filepath.Join(runDir, "trace-restore")
	restoreOptions := options
	restoreOptions.FileSizeMB = 1
	restoreOptions.FileCount = 1
	process, err := startDiagMDLog(restoreOptions, options.ConfigPath, restoreDir, filepath.Join(runDir, "trace-restore.log"))
	if err != nil {
		return err
	}
	waitErr := waitContext(ctx, 350*time.Millisecond)
	stopErr := process.stop(options.StopGrace)
	_ = os.RemoveAll(restoreDir)
	event := actionEvent{At: time.Now().UTC(), Action: "restore-production-qsh-mask", Request: map[string]any{"config": options.ConfigPath}}
	if err := errors.Join(waitErr, stopErr); err != nil {
		event.Error = err.Error()
	}
	*actions = append(*actions, event)
	return errors.Join(waitErr, stopErr)
}

func restoreProductionTraceFromState(ctx context.Context, state *recoveryState, diagOverride string) error {
	if !state.TraceRestoreRequired {
		return nil
	}
	if state.ProductionQTraceBase64 == "" {
		return errors.New("recovery journal requires QSH restore but has no production qtrace")
	}
	data, err := base64.StdEncoding.DecodeString(state.ProductionQTraceBase64)
	if err != nil {
		return fmt.Errorf("decode production qtrace from recovery journal: %w", err)
	}
	if _, _, err := patchActiveQTrace(data); err != nil {
		return fmt.Errorf("validate journaled production qtrace: %w", err)
	}
	workDir := state.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(defaultStatePath)
	}
	recoveryDir, err := os.MkdirTemp(workDir, "trace-recovery-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(recoveryDir)
	configPath := filepath.Join(recoveryDir, "qtrace-production.cfg")
	if err := writeFileAtomic(configPath, data, 0600); err != nil {
		return err
	}
	diagMDLog := state.DiagMDLog
	if strings.TrimSpace(diagOverride) != "" {
		diagMDLog = diagOverride
	}
	if diagMDLog == "" {
		diagMDLog = "diag_mdlog"
	}
	stopGrace := time.Duration(state.StopGraceMS) * time.Millisecond
	if stopGrace <= 0 {
		stopGrace = 2 * time.Second
	}
	options := runOptions{DiagMDLog: diagMDLog, ConfigPath: configPath, FileSizeMB: 1, FileCount: 1, StopGrace: stopGrace}
	if err := restoreProductionTrace(ctx, options, recoveryDir, &[]actionEvent{}); err != nil {
		return err
	}
	state.TraceRestoreRequired = false
	state.Stage = "trace-restored"
	state.LastError = ""
	return nil
}

func ensureNoDiagMDLog() error {
	output, err := exec.Command("pidof", "diag_mdlog").Output()
	if err != nil && len(output) == 0 {
		return nil
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("diag_mdlog is already running (pid %s); stop it explicitly", strings.Join(strings.Fields(string(output)), ","))
	}
	return nil
}

func findQMDLFiles(directory string) []string {
	entries, _ := os.ReadDir(directory)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".qmdl") {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths
}

func resourcePreflight(options runOptions) error {
	memory, err := readMemAvailable()
	if err != nil {
		return fmt.Errorf("read MemAvailable: %w", err)
	}
	free, err := workFreeBytes(options.WorkDir)
	if err != nil {
		return fmt.Errorf("read work directory free space: %w", err)
	}
	if memory < options.MinMemoryBytes {
		return fmt.Errorf("MemAvailable %d MiB is below fuse %d MiB", memory/1024/1024, options.MinMemoryBytes/1024/1024)
	}
	if free < options.MinWorkFreeBytes {
		return fmt.Errorf("work free space %d MiB is below fuse %d MiB", free/1024/1024, options.MinWorkFreeBytes/1024/1024)
	}
	return nil
}

func sampleResources(options runOptions, captureDir string) (resourceSample, error) {
	memory, memoryErr := readMemAvailable()
	free, freeErr := workFreeBytes(options.WorkDir)
	return resourceSample{At: time.Now().UTC(), MemAvailableBytes: memory, WorkFreeBytes: free, CaptureBytes: directoryBytes(captureDir)}, errors.Join(memoryErr, freeErr)
}

func acquireProbeLock(path string) (*os.File, error) {
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("another cross-band probe is active: %w", err)
	}
	_ = lock.Truncate(0)
	_, _ = lock.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid()))
	return lock, nil
}

func releaseProbeLock(lock *os.File) {
	if lock != nil {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
	}
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func joinReason(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}

func hasTargetResult(results []ml1Result, target targetSpec) bool {
	for _, result := range results {
		if matchesTargetResult(result, target) {
			return true
		}
	}
	return false
}
