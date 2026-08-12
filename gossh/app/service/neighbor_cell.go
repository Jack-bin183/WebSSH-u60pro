package service

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gossh/gin"
)

//go:embed embed/qtrace.cfg
var neighborAssets embed.FS

const (
	neighborBaseDir       = "/tmp/u60nbrqt_resident"
	neighborRingDir       = neighborBaseDir + "/ring"
	neighborConfig        = "qtrace.cfg"
	neighborLegacyParser  = "u60nbrqt_parser"
	neighborConfigSHA256  = "ea57a5616a2a94171422452e93c8683671712d33d69ba39820f615af3be503d7"
	neighborIdleTimeout   = 30 * time.Minute
	neighborStaleTimeout  = 90 * time.Second
	neighborParseTimeout  = 12 * time.Second
	neighborUbusTimeout   = 4 * time.Second
	neighborRingFileCount = 4
)

var (
	neighborMonitorOnce  sync.Once
	neighborTakeoverOnce sync.Once
	neighborTakeoverErr  error
	neighborManager      = struct {
		sync.Mutex
		pid            int
		child          *exec.Cmd
		lastAccess     time.Time
		startedAt      time.Time
		lastProgress   time.Time
		latestQMDLTime time.Time
	}{}
)

type NeighborServiceStatus struct {
	DirExists    bool   `json:"dir_exists"`
	State        string `json:"state"`
	ParserExists bool   `json:"parser_exists"`
	Source       string `json:"source"`
	DiagRunning  bool   `json:"diag_running"`
	PID          int    `json:"pid,omitempty"`
	IdleSeconds  int64  `json:"idle_seconds,omitempty"`
}

type NeighborCell struct {
	RAT              string   `json:"rat"`
	PCI              int      `json:"pci"`
	ARFCN            *int     `json:"arfcn"`
	Band             *int     `json:"band"`
	Samples          int      `json:"samples,omitempty"`
	DirectHits       int      `json:"direct_hits,omitempty"`
	FirstSeq         int      `json:"first_seq,omitempty"`
	LastSeq          int      `json:"last_seq,omitempty"`
	RSRPMedian       *float64 `json:"rsrp_median"`
	PlausibleSamples int      `json:"plausible_samples,omitempty"`
	RSRQ             *float64 `json:"rsrq"`
	SINR             *float64 `json:"sinr"`
	Bandwidth        string   `json:"bandwidth,omitempty"`
}

type NeighborCellResult struct {
	Engine          string         `json:"engine"`
	Version         string         `json:"version"`
	Source          string         `json:"source"`
	Frames          int            `json:"frames,omitempty"`
	Malformed       int            `json:"malformed,omitempty"`
	Serving         []NeighborCell `json:"serving"`
	Neighbors       []NeighborCell `json:"neighbors"`
	NetworkType     string         `json:"network_type,omitempty"`
	NetworkProvider string         `json:"network_provider,omitempty"`
	Warning         string         `json:"warning,omitempty"`
}

type nativeServingCell struct {
	Seq     int      `json:"seq"`
	ARFCN   *int     `json:"arfcn"`
	GCI     int64    `json:"gci"`
	PCI     int      `json:"pci"`
	RSRPDBM *float64 `json:"rsrp_dbm"`
	RSRQDB  *float64 `json:"rsrq_db"`
	SINRDB  *float64 `json:"sinr_db"`
}

type nativeNeighborCell struct {
	RAT              string   `json:"rat"`
	PCI              int      `json:"pci"`
	ARFCN            *int     `json:"arfcn"`
	Band             *int     `json:"band"`
	Samples          int      `json:"samples"`
	DirectHits       int      `json:"direct_hits"`
	FirstSeq         int      `json:"first_seq"`
	LastSeq          int      `json:"last_seq"`
	RSRPMedian       *float64 `json:"rsrp_median"`
	PlausibleSamples int      `json:"plausible_samples"`
}

type nativeNeighborResult struct {
	Engine    string               `json:"engine"`
	Version   string               `json:"version"`
	Frames    int                  `json:"frames"`
	Malformed int                  `json:"malformed"`
	Serving   []nativeServingCell  `json:"serving"`
	Neighbors []nativeNeighborCell `json:"neighbors"`
}

type neighborUbusSnapshot map[string]any

func NeighborServiceStatusHandler(c *gin.Context) {
	status := NeighborServiceStatus{Source: "ubus", ParserExists: true}
	if info, err := os.Stat(neighborBaseDir); err == nil && info.IsDir() {
		status.DirExists = true
	}
	if data, err := os.ReadFile(filepath.Join(neighborBaseDir, "state")); err == nil {
		status.State = strings.TrimSpace(string(data))
	}
	neighborManager.Lock()
	refreshManagedPIDLocked()
	status.PID = neighborManager.pid
	status.DiagRunning = neighborManager.pid > 0
	if !neighborManager.lastAccess.IsZero() {
		status.IdleSeconds = int64(time.Since(neighborManager.lastAccess).Seconds())
	}
	neighborManager.Unlock()
	if status.ParserExists && status.DiagRunning {
		status.Source = "parser"
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": status})
}

func NeighborCellHandler(c *gin.Context) {
	snapshot, ubusErr := readNeighborUbus(c.Request.Context())
	pipelineErr := ensureNeighborPipeline()
	if pipelineErr == nil {
		waitForFirstQMDL(c.Request.Context(), 4*time.Second)
	}

	if pipelineErr == nil {
		native, parseErr := parseNeighborQMDL(c.Request.Context())
		if parseErr == nil {
			result := adaptNativeNeighborResult(native, snapshot)
			c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
			return
		}
		pipelineErr = fmt.Errorf("原生日志暂不可用: %w", parseErr)
	}

	if ubusErr == nil {
		result := neighborResultFromUbus(snapshot)
		result.Warning = pipelineErr.Error()
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
		return
	}

	message := "获取邻近小区失败"
	if pipelineErr != nil {
		message += ": " + pipelineErr.Error()
	}
	if ubusErr != nil {
		message += "; UBUS: " + ubusErr.Error()
	}
	c.JSON(http.StatusOK, gin.H{"code": 2, "msg": message})
}

func NeighborServiceStopHandler(c *gin.Context) {
	neighborManager.Lock()
	neighborManager.lastAccess = time.Time{}
	stopped := stopManagedDiagLocked("stopped: requested by WebSSH")
	neighborManager.Unlock()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"stopped": stopped}})
}

func ensureNeighborPipeline() error {
	if err := extractNeighborAssets(); err != nil {
		return err
	}
	neighborTakeoverOnce.Do(func() {
		neighborTakeoverErr = stopLegacyNeighborService()
	})
	if neighborTakeoverErr != nil {
		return neighborTakeoverErr
	}
	neighborMonitorOnce.Do(func() { go monitorNeighborPipeline() })

	neighborManager.Lock()
	defer neighborManager.Unlock()
	neighborManager.lastAccess = time.Now()
	return ensureManagedDiagLocked()
}

func extractNeighborAssets() error {
	if err := os.MkdirAll(neighborBaseDir, 0700); err != nil {
		return fmt.Errorf("创建邻区目录失败: %w", err)
	}
	if err := os.Chmod(neighborBaseDir, 0700); err != nil {
		return fmt.Errorf("设置邻区目录权限失败: %w", err)
	}
	if err := os.MkdirAll(neighborRingDir, 0700); err != nil {
		return fmt.Errorf("创建 QMDL 目录失败: %w", err)
	}
	if err := os.Remove(filepath.Join(neighborBaseDir, neighborLegacyParser)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("移除旧邻区解析器失败: %w", err)
	}

	assets := []struct {
		name string
		mode os.FileMode
		hash string
	}{
		{name: neighborConfig, mode: 0600, hash: neighborConfigSHA256},
	}
	for _, asset := range assets {
		data, err := neighborAssets.ReadFile("embed/" + asset.name)
		if err != nil {
			return fmt.Errorf("读取内嵌 %s 失败: %w", asset.name, err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != asset.hash {
			return fmt.Errorf("内嵌 %s 校验失败", asset.name)
		}
		if err := writeAssetIfChanged(filepath.Join(neighborBaseDir, asset.name), data, asset.mode, asset.hash); err != nil {
			return err
		}
	}
	return nil
}

func writeAssetIfChanged(path string, data []byte, mode os.FileMode, expectedHash string) error {
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		current, readErr := os.ReadFile(path)
		if readErr == nil {
			sum := sha256.Sum256(current)
			if hex.EncodeToString(sum[:]) == expectedHash {
				return os.Chmod(path, mode)
			}
		}
	} else if err == nil {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			return fmt.Errorf("移除无效资源 %s 失败: %w", filepath.Base(path), removeErr)
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".neighbor-asset-*")
	if err != nil {
		return fmt.Errorf("创建临时资源失败: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("安装资源 %s 失败: %w", filepath.Base(path), err)
	}
	ok = true
	return nil
}

func stopLegacyNeighborService() error {
	pidData, err := os.ReadFile(filepath.Join(neighborBaseDir, "service.pid"))
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil || !pidAlive(pid) {
		_ = os.Remove(filepath.Join(neighborBaseDir, "service.pid"))
		return nil
	}
	cmdline := processCmdline(pid)
	if !strings.Contains(cmdline, neighborBaseDir) || !strings.Contains(cmdline, "service.sh") {
		return fmt.Errorf("service.pid=%d 不属于旧邻区服务，拒绝接管", pid)
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 20 && pidAlive(pid); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if pidAlive(pid) && strings.Contains(processCmdline(pid), neighborBaseDir) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	_ = os.Remove(filepath.Join(neighborBaseDir, "service.pid"))
	return nil
}

func ensureManagedDiagLocked() error {
	refreshManagedPIDLocked()
	if neighborManager.pid > 0 {
		return nil
	}

	ownPID, foreignPID := discoverDiagProcesses()
	if ownPID > 0 {
		neighborManager.pid = ownPID
		neighborManager.startedAt = time.Now()
		neighborManager.lastProgress = time.Now()
		neighborManager.latestQMDLTime = latestNeighborQMDLTime()
		_ = writeNeighborState(fmt.Sprintf("running: adopted diag_mdlog pid=%d", ownPID))
		_ = writeNeighborPID(ownPID)
		return nil
	}
	if foreignPID > 0 {
		_ = writeNeighborState(fmt.Sprintf("blocked: foreign diag_mdlog pid=%d", foreignPID))
		return fmt.Errorf("diag_mdlog 被其他进程占用 (pid=%d)", foreignPID)
	}
	clearNeighborQMDLFiles()

	logFile, err := os.OpenFile(filepath.Join(neighborBaseDir, "diag.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("打开 diag 日志失败: %w", err)
	}
	cmd := exec.Command("diag_mdlog",
		"-f", filepath.Join(neighborBaseDir, neighborConfig),
		"-o", neighborRingDir,
		"-s", "4",
		"-n", strconv.Itoa(neighborRingFileCount),
		"-c",
		"-d",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("启动 diag_mdlog 失败: %w", err)
	}
	_ = logFile.Close()

	now := time.Now()
	pid := cmd.Process.Pid
	neighborManager.pid = pid
	neighborManager.child = cmd
	neighborManager.startedAt = now
	neighborManager.lastProgress = now
	neighborManager.latestQMDLTime = latestNeighborQMDLTime()
	_ = writeNeighborPID(pid)
	_ = writeNeighborState(fmt.Sprintf("running: diag_mdlog pid=%d", pid))
	go reapManagedDiag(cmd, pid)
	return nil
}

func reapManagedDiag(cmd *exec.Cmd, pid int) {
	err := cmd.Wait()
	neighborManager.Lock()
	defer neighborManager.Unlock()
	if neighborManager.pid != pid {
		return
	}
	neighborManager.pid = 0
	neighborManager.child = nil
	_ = os.Remove(filepath.Join(neighborBaseDir, "diag.pid"))
	state := fmt.Sprintf("exited: diag_mdlog pid=%d", pid)
	if err != nil {
		state += " error=" + err.Error()
	}
	_ = writeNeighborState(state)
}

func monitorNeighborPipeline() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		neighborManager.Lock()
		refreshManagedPIDLocked()
		now := time.Now()
		if neighborManager.lastAccess.IsZero() {
			neighborManager.Unlock()
			continue
		}
		if now.Sub(neighborManager.lastAccess) >= neighborIdleTimeout {
			stopManagedDiagLocked("stopped: idle timeout")
			neighborManager.lastAccess = time.Time{}
			neighborManager.Unlock()
			continue
		}
		if neighborManager.pid == 0 {
			_ = ensureManagedDiagLocked()
			neighborManager.Unlock()
			continue
		}

		latest := latestNeighborQMDLTime()
		if latest.After(neighborManager.latestQMDLTime) {
			neighborManager.latestQMDLTime = latest
			neighborManager.lastProgress = now
		}
		if now.Sub(neighborManager.lastProgress) >= neighborStaleTimeout {
			stopManagedDiagLocked("restarting: QMDL stalled")
			if err := ensureManagedDiagLocked(); err != nil {
				_ = writeNeighborState("restart failed: " + err.Error())
			}
		}
		neighborManager.Unlock()
	}
}

func refreshManagedPIDLocked() {
	if neighborManager.pid > 0 && pidAlive(neighborManager.pid) && isNeighborDiag(neighborManager.pid) {
		return
	}
	neighborManager.pid = 0
	neighborManager.child = nil
	pidData, err := os.ReadFile(filepath.Join(neighborBaseDir, "diag.pid"))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err == nil && pidAlive(pid) && isNeighborDiag(pid) {
		neighborManager.pid = pid
		return
	}
	_ = os.Remove(filepath.Join(neighborBaseDir, "diag.pid"))
}

func discoverDiagProcesses() (ownPID int, foreignPID int) {
	out, err := exec.Command("pidof", "diag_mdlog").Output()
	if err != nil {
		return 0, 0
	}
	for _, value := range strings.Fields(string(out)) {
		pid, parseErr := strconv.Atoi(value)
		if parseErr != nil || !pidAlive(pid) {
			continue
		}
		if isNeighborDiag(pid) {
			if ownPID == 0 {
				ownPID = pid
			}
		} else if foreignPID == 0 {
			foreignPID = pid
		}
	}
	return ownPID, foreignPID
}

func stopManagedDiagLocked(state string) bool {
	refreshManagedPIDLocked()
	pid := neighborManager.pid
	if pid <= 0 || !isNeighborDiag(pid) {
		return false
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 10 && pidAlive(pid); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if pidAlive(pid) && isNeighborDiag(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if neighborManager.pid == pid {
		neighborManager.pid = 0
		neighborManager.child = nil
	}
	_ = os.Remove(filepath.Join(neighborBaseDir, "diag.pid"))
	_ = writeNeighborState(state)
	return true
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func processCmdline(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(data), "\x00", " ")
}

func isNeighborDiag(pid int) bool {
	cmdline := processCmdline(pid)
	return strings.Contains(cmdline, "diag_mdlog") && strings.Contains(cmdline, neighborRingDir)
}

func writeNeighborPID(pid int) error {
	return writeNeighborTextFile(filepath.Join(neighborBaseDir, "diag.pid"), strconv.Itoa(pid)+"\n")
}

func writeNeighborState(state string) error {
	return writeNeighborTextFile(filepath.Join(neighborBaseDir, "state"), state+"\n")
}

func writeNeighborTextFile(path, value string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".neighbor-state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.WriteString(tmp, value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func waitForFirstQMDL(ctx context.Context, timeout time.Duration) {
	if len(findNeighborQMDLFiles()) > 0 {
		return
	}
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			return
		case <-ticker.C:
			if len(findNeighborQMDLFiles()) > 0 {
				return
			}
		}
	}
}

func parseNeighborQMDL(parent context.Context) (*nativeNeighborResult, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		files := findNeighborQMDLFiles()
		if len(files) == 0 {
			return nil, os.ErrNotExist
		}
		ctx, cancel := context.WithTimeout(parent, neighborParseTimeout)
		result, err := parseNeighborQMDLFiles(ctx, files)
		ctxErr := ctx.Err()
		cancel()
		if err != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
				lastErr = fmt.Errorf("解析 QMDL 超时")
			} else {
				lastErr = err
			}
			continue
		}
		if result.Serving == nil {
			result.Serving = []nativeServingCell{}
		}
		if result.Neighbors == nil {
			result.Neighbors = []nativeNeighborCell{}
		}
		return result, nil
	}
	return nil, lastErr
}

type neighborQMDLFile struct {
	path    string
	modTime time.Time
}

func findNeighborQMDLFiles() []string {
	entries, err := os.ReadDir(neighborRingDir)
	if err != nil {
		return nil
	}
	files := make([]neighborQMDLFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "diag_log_") || !strings.HasSuffix(entry.Name(), ".qmdl") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, neighborQMDLFile{path: filepath.Join(neighborRingDir, entry.Name()), modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].path < files[j].path
		}
		return files[i].modTime.Before(files[j].modTime)
	})
	if len(files) > neighborRingFileCount {
		files = files[len(files)-neighborRingFileCount:]
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.path)
	}
	return paths
}

func latestNeighborQMDLTime() time.Time {
	var latest time.Time
	entries, err := os.ReadDir(neighborRingDir)
	if err != nil {
		return latest
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "diag_log_") || !strings.HasSuffix(entry.Name(), ".qmdl") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func clearNeighborQMDLFiles() {
	entries, err := os.ReadDir(neighborRingDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "diag_log_") || !strings.HasSuffix(entry.Name(), ".qmdl") {
			continue
		}
		_ = os.Remove(filepath.Join(neighborRingDir, entry.Name()))
	}
}

func readNeighborUbus(parent context.Context) (neighborUbusSnapshot, error) {
	ctx, cancel := context.WithTimeout(parent, neighborUbusTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ubus", "call", "zte_nwinfo_api", "nwinfo_get_netinfo", "{}").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("读取网络信息超时")
		}
		return nil, err
	}
	var snapshot neighborUbusSnapshot
	if err := json.Unmarshal(out, &snapshot); err != nil {
		return nil, fmt.Errorf("网络信息 JSON 无效: %w", err)
	}
	return snapshot, nil
}

func (snapshot neighborUbusSnapshot) text(key string) string {
	value, ok := snapshot[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func neighborResultFromUbus(snapshot neighborUbusSnapshot) *NeighborCellResult {
	result := &NeighborCellResult{
		Engine:          "ubus",
		Version:         "native",
		Source:          "ubus",
		Serving:         []NeighborCell{},
		Neighbors:       []NeighborCell{},
		NetworkType:     snapshot.text("network_type"),
		NetworkProvider: snapshot.text("network_provider"),
	}
	mode := strings.ToUpper(result.NetworkType)
	isNSA := strings.Contains(mode, "NSA") || strings.Contains(mode, "ENDC")
	isNR := isNSA || strings.Contains(mode, "5G") || strings.Contains(mode, "NR") || strings.Contains(mode, "SA")
	isLTE := isNSA || strings.Contains(mode, "LTE") || strings.Contains(mode, "4G")
	if !isLTE && !isNR {
		isLTE = snapshot.text("lte_pci") != ""
		isNR = snapshot.text("nr5g_pci") != ""
	}
	if isLTE {
		if cell, ok := currentLTECell(snapshot); ok {
			result.Serving = append(result.Serving, cell)
			result.Neighbors = append(result.Neighbors, parseLTECA(snapshot.text("lteca"), cell)...)
		}
	}
	if isNR {
		if cell, ok := currentNRCell(snapshot); ok {
			result.Serving = append(result.Serving, cell)
		}
		result.Neighbors = append(result.Neighbors, parseNRCA(snapshot.text("nrca"))...)
	}
	return result
}

func currentLTECell(snapshot neighborUbusSnapshot) (NeighborCell, bool) {
	pci, ok := optionalInt(snapshot.text("lte_pci"))
	if !ok || pci < 0 || pci > 503 {
		return NeighborCell{}, false
	}
	band, bandOK := parseNeighborBand(snapshot.text("wan_active_band"))
	arfcn, arfcnOK := optionalInt(snapshot.text("wan_active_channel"))
	return NeighborCell{
		RAT: "LTE", PCI: pci, ARFCN: pointerIf(arfcn, arfcnOK), Band: pointerIf(band, bandOK),
		Samples: 1, RSRPMedian: optionalFloatPtr(snapshot.text("lte_rsrp")),
		RSRQ: optionalFloatPtr(snapshot.text("lte_rsrq")), SINR: optionalFloatPtr(snapshot.text("lte_snr")),
		Bandwidth: lteBandwidthFromCA(snapshot.text("lteca")),
	}, true
}

func currentNRCell(snapshot neighborUbusSnapshot) (NeighborCell, bool) {
	pci, ok := optionalInt(snapshot.text("nr5g_pci"))
	if !ok || pci < 0 || pci > 1007 {
		return NeighborCell{}, false
	}
	bandText := snapshot.text("nr5g_action_band")
	if bandText == "" {
		bandText = snapshot.text("wan_active_band")
	}
	band, bandOK := parseNeighborBand(bandText)
	arfcn, arfcnOK := optionalInt(snapshot.text("nr5g_action_channel"))
	return NeighborCell{
		RAT: "NR", PCI: pci, ARFCN: pointerIf(arfcn, arfcnOK), Band: pointerIf(band, bandOK),
		Samples: 1, RSRPMedian: optionalFloatPtr(snapshot.text("nr5g_rsrp")),
		RSRQ: optionalFloatPtr(snapshot.text("nr5g_rsrq")), SINR: optionalFloatPtr(snapshot.text("nr5g_snr")),
		Bandwidth: snapshot.text("nr5g_bandwidth"),
	}, true
}

func parseLTECA(value string, serving NeighborCell) []NeighborCell {
	cells := []NeighborCell{}
	for _, item := range strings.Split(value, ";") {
		fields := splitNeighborFields(item)
		if len(fields) < 5 {
			continue
		}
		pci, pciOK := optionalInt(fields[0])
		band, bandOK := parseNeighborBand(fields[1])
		arfcn, arfcnOK := optionalInt(fields[3])
		if !pciOK || pci < 0 || pci > 503 {
			continue
		}
		if pci == serving.PCI && sameOptionalInt(pointerIf(arfcn, arfcnOK), serving.ARFCN) {
			continue
		}
		cells = append(cells, NeighborCell{
			RAT: "LTE", PCI: pci, Band: pointerIf(band, bandOK), ARFCN: pointerIf(arfcn, arfcnOK),
			Bandwidth: fields[4], Samples: 1,
		})
	}
	return cells
}

func parseNRCA(value string) []NeighborCell {
	cells := []NeighborCell{}
	for _, item := range strings.Split(value, ";") {
		fields := splitNeighborFields(item)
		if len(fields) < 11 {
			continue
		}
		pci, pciOK := optionalInt(fields[1])
		band, bandOK := parseNeighborBand(fields[3])
		arfcn, arfcnOK := optionalInt(fields[4])
		if !pciOK || pci < 0 || pci > 1007 {
			continue
		}
		cells = append(cells, NeighborCell{
			RAT: "NR", PCI: pci, Band: pointerIf(band, bandOK), ARFCN: pointerIf(arfcn, arfcnOK),
			Bandwidth: fields[5], RSRPMedian: optionalFloatPtr(fields[7]), SINR: optionalFloatPtr(fields[9]), Samples: 1,
		})
	}
	return cells
}

func splitNeighborFields(value string) []string {
	fields := strings.Split(value, ",")
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return fields
}

func lteBandwidthFromCA(value string) string {
	for _, item := range strings.Split(value, ";") {
		fields := splitNeighborFields(item)
		if len(fields) >= 5 && fields[4] != "" {
			return fields[4]
		}
	}
	return ""
}

func adaptNativeNeighborResult(native *nativeNeighborResult, snapshot neighborUbusSnapshot) *NeighborCellResult {
	ubus := neighborResultFromUbus(snapshot)
	result := &NeighborCellResult{
		Engine: native.Engine, Version: native.Version, Source: "parser",
		Frames: native.Frames, Malformed: native.Malformed,
		Serving: []NeighborCell{}, Neighbors: []NeighborCell{},
		NetworkType: ubus.NetworkType, NetworkProvider: ubus.NetworkProvider,
	}

	for _, raw := range native.Serving {
		cell, found := matchServingCell(raw.PCI, raw.ARFCN, ubus.Serving)
		if !found {
			rat := inferNativeRAT(raw.PCI, raw.ARFCN, result.NetworkType)
			cell = NeighborCell{RAT: rat, PCI: raw.PCI, ARFCN: raw.ARFCN, Band: bandForRAT(rat, ubus.Serving)}
		}
		cell.Samples = 1
		cell.RSRPMedian = raw.RSRPDBM
		cell.RSRQ = raw.RSRQDB
		cell.SINR = raw.SINRDB
		appendUniqueNeighborCell(&result.Serving, cell)
	}
	for _, cell := range ubus.Serving {
		appendUniqueNeighborCell(&result.Serving, cell)
	}

	for _, raw := range native.Neighbors {
		rat := strings.ToUpper(strings.TrimSpace(raw.RAT))
		if rat == "" {
			rat = inferNativeRAT(raw.PCI, raw.ARFCN, result.NetworkType)
		}
		cell := NeighborCell{
			RAT: rat, PCI: raw.PCI, ARFCN: raw.ARFCN, Band: raw.Band,
			Samples: raw.Samples, DirectHits: raw.DirectHits, FirstSeq: raw.FirstSeq, LastSeq: raw.LastSeq,
			RSRPMedian: raw.RSRPMedian, PlausibleSamples: raw.PlausibleSamples,
		}
		appendUniqueNeighborCell(&result.Neighbors, cell)
	}
	return result
}

func matchServingCell(pci int, arfcn *int, cells []NeighborCell) (NeighborCell, bool) {
	for _, cell := range cells {
		if cell.PCI == pci && (arfcn == nil || cell.ARFCN == nil || *arfcn == *cell.ARFCN) {
			return cell, true
		}
	}
	return NeighborCell{}, false
}

func appendUniqueNeighborCell(cells *[]NeighborCell, candidate NeighborCell) {
	for _, cell := range *cells {
		if cell.RAT == candidate.RAT && cell.PCI == candidate.PCI && sameOptionalInt(cell.ARFCN, candidate.ARFCN) {
			return
		}
	}
	*cells = append(*cells, candidate)
}

func sameOptionalInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func inferNativeRAT(pci int, arfcn *int, networkType string) string {
	if pci > 503 {
		return "NR"
	}
	// LTE EARFCN 的有效范围不会超过 262143；更高的频点可直接判定为 NR-ARFCN。
	if arfcn != nil && *arfcn > 262143 {
		return "NR"
	}
	mode := strings.ToUpper(networkType)
	if strings.Contains(mode, "NSA") || strings.Contains(mode, "ENDC") || strings.Contains(mode, "5G") || strings.Contains(mode, "NR") || strings.Contains(mode, "SA") {
		return "NR"
	}
	return "LTE"
}

func bandForRAT(rat string, cells []NeighborCell) *int {
	for _, cell := range cells {
		if cell.RAT == rat && cell.Band != nil {
			value := *cell.Band
			return &value
		}
	}
	return nil
}

func optionalInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if isEmptyNeighborValue(value) {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func optionalFloatPtr(value string) *float64 {
	value = strings.TrimSpace(value)
	if isEmptyNeighborValue(value) {
		return nil
	}
	if fields := strings.Fields(value); len(fields) > 0 {
		value = fields[0]
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func isEmptyNeighborValue(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "-", "--", "N/A", "NA", "NULL", "UNKNOWN":
		return true
	default:
		return false
	}
}

func parseNeighborBand(value string) (int, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	for _, prefix := range []string{"LTE BAND ", "NR BAND ", "BAND ", "LTE ", "NR "} {
		value = strings.TrimPrefix(value, prefix)
	}
	value = strings.TrimSpace(value)
	if len(value) > 1 && (value[0] == 'B' || value[0] == 'N') {
		value = strings.TrimSpace(value[1:])
	}
	return optionalInt(value)
}

func pointerIf(value int, ok bool) *int {
	if !ok {
		return nil
	}
	return &value
}
