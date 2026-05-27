package service

import (
	"context"
	"errors"
	"fmt"
	"gossh/gin"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────── 常量 ───────────────────────────

const (
	mihomoGithubRepo        = "Jack-bin183/WebSSH-u60pro"
	mihomoDataTag           = "latest-data"
	mihomoDataBaseURL       = "https://github.com/" + mihomoGithubRepo + "/releases/download/" + mihomoDataTag + "/"
	mihomoVersionFileURL    = mihomoDataBaseURL + "data_version.txt"
	mihomoInstallVersionURL = mihomoDataBaseURL + "mihomo_version.txt"
	mihomoInstallBinaryURL  = mihomoDataBaseURL + "mihomo-linux-arm64"
	mihomoInstallMmShURL    = mihomoDataBaseURL + "mm.sh"
	mihomoDefaultDir        = "/data/kano_plugins/mihomo"
	mihomoConnTimeout       = 3 * time.Second
)

var mihomoProxies = []string{
	"https://v6.gh-proxy.org/",
	"https://ghfast.top/",
	"https://gh-proxy.com/",
	"https://ghproxy.net/",
	"https://gh.llkk.cc/",
	"https://hub.gitmirror.com/",
	"https://gh-proxy.org/",
}

var mihomoDataFiles = []struct {
	Name string
	Desc string
}{
	{"chnroute.txt", "中国 IPv4 段"},
	{"chnroute6.txt", "中国 IPv6 段"},
	{"GeoSite.dat", "地理站点数据"},
	{"geoip.metadb", "地理 IP 数据"},
}

// ─────────────────────────── 数据更新状态 ───────────────────────────

type MihomoDataUpdateStatus struct {
	State      string `json:"state"`
	Msg        string `json:"msg"`
	FileName   string `json:"file_name"`
	FileIndex  int    `json:"file_index"`
	FileTotal  int    `json:"file_total"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	StartedAt  string `json:"started_at"`
	UpdatedAt  string `json:"updated_at"`
}

var mihomoUpdateStatusMu sync.RWMutex
var mihomoUpdateStatus = MihomoDataUpdateStatus{State: "idle", Msg: "暂无更新任务"}
var mihomoUpdateCancelMu sync.Mutex
var mihomoUpdateCancel context.CancelFunc

func getMihomoUpdateStatus() MihomoDataUpdateStatus {
	mihomoUpdateStatusMu.RLock()
	defer mihomoUpdateStatusMu.RUnlock()
	return mihomoUpdateStatus
}

func setMihomoUpdateStatus(fn func(*MihomoDataUpdateStatus)) {
	mihomoUpdateStatusMu.Lock()
	defer mihomoUpdateStatusMu.Unlock()
	fn(&mihomoUpdateStatus)
	mihomoUpdateStatus.UpdatedAt = time.Now().Format(time.RFC3339)
	if mihomoUpdateStatus.Total > 0 {
		p := int(mihomoUpdateStatus.Downloaded * 100 / mihomoUpdateStatus.Total)
		if p > 100 {
			p = 100
		}
		mihomoUpdateStatus.Percent = p
	}
}

func isMihomoUpdateBusy() bool {
	return getMihomoUpdateStatus().State == "downloading"
}

// ─────────────────────────── 安装状态 ───────────────────────────

type MihomoInstallStatus struct {
	State      string `json:"state"` // idle / downloading / installing / done / failed / canceled
	Msg        string `json:"msg"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	StartedAt  string `json:"started_at"`
	UpdatedAt  string `json:"updated_at"`
}

var mihomoInstallStatusMu sync.RWMutex
var mihomoInstallStatusVar = MihomoInstallStatus{State: "idle", Msg: "暂无安装任务"}
var mihomoInstallCancelMu sync.Mutex
var mihomoInstallCancel context.CancelFunc

func getMihomoInstallStatus() MihomoInstallStatus {
	mihomoInstallStatusMu.RLock()
	defer mihomoInstallStatusMu.RUnlock()
	return mihomoInstallStatusVar
}

func setMihomoInstallStatus(fn func(*MihomoInstallStatus)) {
	mihomoInstallStatusMu.Lock()
	defer mihomoInstallStatusMu.Unlock()
	fn(&mihomoInstallStatusVar)
	mihomoInstallStatusVar.UpdatedAt = time.Now().Format(time.RFC3339)
	if mihomoInstallStatusVar.Total > 0 {
		p := int(mihomoInstallStatusVar.Downloaded * 100 / mihomoInstallStatusVar.Total)
		if p > 100 {
			p = 100
		}
		mihomoInstallStatusVar.Percent = p
	}
}

func isMihomoInstallBusy() bool {
	return getMihomoInstallStatus().State == "downloading" || getMihomoInstallStatus().State == "installing"
}

// ─────────────────────────── HTTP 工具 ───────────────────────────

func mihomoHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   mihomoConnTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = mihomoConnTimeout
	transport.ResponseHeaderTimeout = 10 * time.Second
	return &http.Client{Transport: transport}
}

func buildMihomoTryURLs(originalURL string, proxies []string) []string {
	trimmed := strings.TrimPrefix(originalURL, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	urls := make([]string, 0, len(proxies)+1)
	seen := make(map[string]struct{})
	add := func(u string) {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			urls = append(urls, u)
		}
	}
	for _, proxy := range proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		if !strings.HasSuffix(proxy, "/") {
			proxy += "/"
		}
		add(proxy + originalURL)
		add(proxy + trimmed)
	}
	add(originalURL)
	return urls
}

// ─────────────────────────── 代理测速 ───────────────────────────

const (
	proxyProbeMaxBytes int64         = 100 * 1024
	proxyProbeTimeout  time.Duration = 3 * time.Second
	proxyRankCacheTTL  time.Duration = 60 * time.Second
)

type proxyRankCacheEntry struct {
	ranked    []string
	expiresAt time.Time
}

var (
	proxyRankCache   = make(map[string]proxyRankCacheEntry)
	proxyRankCacheMu sync.Mutex
)

// RankProxiesBySpeed 对一组代理做并行测速（Range 0-100KB），返回按吞吐量从快到慢排好序的列表。
// 测速失败的代理保留在末尾以便兜底重试。结果在 60s 内会被缓存，避免批量下载时重复探测。
func RankProxiesBySpeed(proxies []string, probeURL string) []string {
	if len(proxies) <= 1 {
		return append([]string(nil), proxies...)
	}
	cacheKey := strings.Join(proxies, "|") + "::" + probeURL
	proxyRankCacheMu.Lock()
	if e, ok := proxyRankCache[cacheKey]; ok && time.Now().Before(e.expiresAt) {
		ranked := append([]string(nil), e.ranked...)
		proxyRankCacheMu.Unlock()
		return ranked
	}
	proxyRankCacheMu.Unlock()

	type result struct {
		proxy string
		bps   float64
		ok    bool
	}
	client := mihomoHTTPClient()
	out := make(chan result, len(proxies))
	var wg sync.WaitGroup
	for _, p := range proxies {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			bps, err := probeProxySpeed(client, p, probeURL)
			out <- result{p, bps, err == nil && bps > 0}
		}(p)
	}
	wg.Wait()
	close(out)

	var good, bad []result
	for r := range out {
		if r.ok {
			good = append(good, r)
		} else {
			bad = append(bad, r)
		}
	}
	sort.SliceStable(good, func(i, j int) bool { return good[i].bps > good[j].bps })

	indexOf := func(s string) int {
		for i, p := range proxies {
			if p == s {
				return i
			}
		}
		return len(proxies)
	}
	sort.SliceStable(bad, func(i, j int) bool { return indexOf(bad[i].proxy) < indexOf(bad[j].proxy) })

	ranked := make([]string, 0, len(proxies))
	for _, r := range good {
		ranked = append(ranked, r.proxy)
	}
	for _, r := range bad {
		ranked = append(ranked, r.proxy)
	}

	if len(good) > 0 {
		slog.Info("proxy speed rank",
			"winner", good[0].proxy,
			"winner_kbps", int(good[0].bps/1024),
			"good_count", len(good),
			"bad_count", len(bad),
		)
	} else {
		slog.Warn("proxy speed rank: all probes failed", "count", len(proxies))
	}

	proxyRankCacheMu.Lock()
	proxyRankCache[cacheKey] = proxyRankCacheEntry{
		ranked:    append([]string(nil), ranked...),
		expiresAt: time.Now().Add(proxyRankCacheTTL),
	}
	proxyRankCacheMu.Unlock()
	return ranked
}

func probeProxySpeed(client *http.Client, proxy, originalURL string) (float64, error) {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return 0, errors.New("empty proxy")
	}
	if !strings.HasSuffix(proxy, "/") {
		proxy += "/"
	}
	probeURL := proxy + originalURL
	ctx, cancel := context.WithTimeout(context.Background(), proxyProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", proxyProbeMaxBytes-1))
	req.Header.Set("User-Agent", "WebSSH-u60pro-Probe")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// 200 (Range 不支持时整体返回) / 206 (部分内容) 都视为成功
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, proxyProbeMaxBytes))
	elapsed := time.Since(start).Seconds()
	if n == 0 || elapsed <= 0 {
		if err != nil {
			return 0, err
		}
		return 0, errors.New("empty body")
	}
	// 即便后半段读到一半被取消，已读字节也能反映出实际吞吐量
	return float64(n) / elapsed, nil
}

func mihomoFetchText(rawURL string) (string, error) {
	client := mihomoHTTPClient()
	var lastErr error
	for _, u := range buildMihomoTryURLs(rawURL, mihomoProxies) {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "WebSSH-u60pro-Mihomo-Updater")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
			resp.Body.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("所有线路均失败: %w", lastErr)
}

func mihomoDownloadFile(ctx context.Context, rawURL string, destPath string, onProgress func(downloaded, total int64)) error {
	client := mihomoHTTPClient()
	var lastErr error
	rankedProxies := RankProxiesBySpeed(mihomoProxies, rawURL)
	for _, u := range buildMihomoTryURLs(rawURL, rankedProxies) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "WebSSH-u60pro-Mihomo-Updater")
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
			resp.Body.Close()
			continue
		}
		total := resp.ContentLength
		tmpPath := destPath + ".tmp"
		out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			resp.Body.Close()
			return err
		}
		buf := make([]byte, 64*1024)
		var downloaded int64
		var writeErr error
		for {
			if ctx.Err() != nil {
				out.Close()
				resp.Body.Close()
				_ = os.Remove(tmpPath)
				return ctx.Err()
			}
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				written, werr := out.Write(buf[:n])
				downloaded += int64(written)
				if onProgress != nil {
					onProgress(downloaded, total)
				}
				if werr != nil {
					writeErr = werr
					break
				}
				if written != n {
					writeErr = io.ErrShortWrite
					break
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				writeErr = readErr
				break
			}
		}
		resp.Body.Close()
		out.Close()
		if writeErr != nil {
			_ = os.Remove(tmpPath)
			lastErr = writeErr
			continue
		}
		info, err := os.Stat(tmpPath)
		if err != nil || info.Size() == 0 {
			_ = os.Remove(tmpPath)
			lastErr = fmt.Errorf("下载文件为空")
			continue
		}
		if err := os.Rename(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		return nil
	}
	return fmt.Errorf("所有线路均失败: %w", lastErr)
}

// ─────────────────────────── 辅助函数 ───────────────────────────

func getMihomoDir() string {
	return mihomoDefaultDir
}

func getMihomoLocalVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "data_version.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func isMihomoRunning(dir string) (bool, int) {
	pidFile := filepath.Join(dir, "mihomo.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}
	if _, statErr := os.Stat(fmt.Sprintf("/proc/%d", pid)); statErr == nil {
		return true, pid
	}
	return false, 0
}

func getMihomoInstalledVersion(dir string) string {
	binPath := filepath.Join(dir, "mihomo")
	if _, err := os.Stat(binPath); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// parseMihomoConfig 从 config.yaml 提取 external-controller 和 secret
func parseMihomoConfig(configPath string) (extCtrl string, secret string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "external-controller:") {
			val := strings.TrimSpace(strings.TrimPrefix(trim, "external-controller:"))
			extCtrl = strings.Trim(val, `"'`)
		}
		if strings.HasPrefix(trim, "secret:") {
			val := strings.TrimSpace(strings.TrimPrefix(trim, "secret:"))
			secret = strings.Trim(val, `"'`)
		}
	}
	// ":9999" → "192.168.0.1:9999/ui"
	if strings.HasPrefix(extCtrl, ":") {
		extCtrl = "192.168.0.1" + extCtrl + "/ui"
	}
	return extCtrl, secret
}

func checkMihomoAPI(configPath string) (reachable bool, version string) {
	extCtrl, secret := parseMihomoConfig(configPath)
	if extCtrl == "" {
		return false, ""
	}

	// 没 scheme 时补 http://
	if !strings.Contains(extCtrl, "://") {
		extCtrl = "http://" + extCtrl
	}

	u, err := url.Parse(extCtrl)
	if err != nil {
		return false, ""
	}

	reqURL := u.Scheme + "://" + u.Host + "/version"

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return false, ""
	}

	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, ""
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	return true, strings.TrimSpace(string(body))
}

func runMihomoMmSh(dir string, action string) (string, error) {
	mmScript := filepath.Join(dir, "mm.sh")
	if _, err := os.Stat(mmScript); err != nil {
		return "", fmt.Errorf("mm.sh 不存在: %s", mmScript)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", mmScript, action)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ─────────────────────────── Handlers: 状态 & 控制 ───────────────────────────

// MihomoStatusHandler GET /api/mihomo/status
func MihomoStatusHandler(c *gin.Context) {
	dir := getMihomoDir()
	_, dirErr := os.Stat(dir)
	dirExists := dirErr == nil

	running, pid := isMihomoRunning(dir)
	localVersion := getMihomoLocalVersion(dir)

	// 二进制信息
	binPath := filepath.Join(dir, "mihomo")
	_, binErr := os.Stat(binPath)
	binaryExists := binErr == nil
	binaryVersion := ""
	if binaryExists {
		binaryVersion = getMihomoInstalledVersion(dir)
	}

	// 启动时间（PID 文件修改时间近似）
	startTime := ""
	if running {
		if info, err := os.Stat(filepath.Join(dir, "mihomo.pid")); err == nil {
			startTime = info.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	// API 状态
	configPath := filepath.Join(dir, "config.yaml")
	extCtrl, _ := parseMihomoConfig(configPath)
	apiReachable, apiVersion := false, ""
	if running {
		apiReachable, apiVersion = checkMihomoAPI(configPath)
	}

	type FileInfo struct {
		Name    string `json:"name"`
		Desc    string `json:"desc"`
		Exists  bool   `json:"exists"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}
	files := make([]FileInfo, 0, len(mihomoDataFiles))
	for _, f := range mihomoDataFiles {
		fi := FileInfo{Name: f.Name, Desc: f.Desc}
		if info, err := os.Stat(filepath.Join(dir, f.Name)); err == nil {
			fi.Exists = true
			fi.Size = info.Size()
			fi.ModTime = info.ModTime().Format("2006-01-02 15:04:05")
		}
		files = append(files, fi)
	}

	c.JSON(200, gin.H{
		"code": 0, "msg": "ok",
		"data": gin.H{
			"running":             running,
			"pid":                 pid,
			"start_time":          startTime,
			"mihomo_dir":          dir,
			"dir_exists":          dirExists,
			"binary_exists":       binaryExists,
			"binary_version":      binaryVersion,
			"local_version":       localVersion,
			"api_reachable":       apiReachable,
			"api_version":         apiVersion,
			"external_controller": extCtrl,
			"files":               files,
			"autostart_enabled":   getMihomoAutostartEnabled(dir),
		},
	})
}

// MihomoControlHandler POST /api/mihomo/control
func MihomoControlHandler(c *gin.Context) {
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	validActions := map[string]bool{
		"start": true, "stop": true, "restart": true, "reload-ipset": true,
	}
	if !validActions[req.Action] {
		c.JSON(200, gin.H{"code": 1, "msg": "无效操作: " + req.Action})
		return
	}
	slog.Info("[mihomo] 执行控制命令", "action", req.Action)
	out, err := runMihomoMmSh(getMihomoDir(), req.Action)
	if err != nil {
		slog.Warn("[mihomo] 控制命令非 0", "action", req.Action, "err", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "执行失败: " + err.Error(), "output": out})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "action": req.Action, "output": out})
}

// MihomoGetDirHandler GET /api/mihomo/dir
func MihomoGetDirHandler(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"mihomo_dir": getMihomoDir()}})
}

// ─────────────────────────── Handlers: 数据文件更新 ───────────────────────────

// MihomoDataVersionHandler GET /api/mihomo/data/version
func MihomoDataVersionHandler(c *gin.Context) {
	remoteVersion, err := mihomoFetchText(mihomoVersionFileURL)
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "获取远端版本失败: " + err.Error()})
		return
	}
	localVersion := getMihomoLocalVersion(getMihomoDir())
	c.JSON(200, gin.H{
		"code": 0, "msg": "ok",
		"data": gin.H{
			"remote_version": remoteVersion,
			"local_version":  localVersion,
			"has_update":     remoteVersion != "" && remoteVersion != localVersion,
		},
	})
}

// MihomoDataUpdateStatusHandler GET /api/mihomo/data/update/status
func MihomoDataUpdateStatusHandler(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": getMihomoUpdateStatus()})
}

// MihomoDataUpdateCancelHandler POST /api/mihomo/data/update/cancel
func MihomoDataUpdateCancelHandler(c *gin.Context) {
	if getMihomoUpdateStatus().State != "downloading" {
		c.JSON(200, gin.H{"code": 1, "msg": "当前无下载任务", "data": getMihomoUpdateStatus()})
		return
	}
	mihomoUpdateCancelMu.Lock()
	cancel := mihomoUpdateCancel
	mihomoUpdateCancelMu.Unlock()
	if cancel == nil {
		c.JSON(200, gin.H{"code": 1, "msg": "没有可取消的任务", "data": getMihomoUpdateStatus()})
		return
	}
	cancel()
	c.JSON(200, gin.H{"code": 0, "msg": "已取消", "data": getMihomoUpdateStatus()})
}

// MihomoDataUpdateHandler POST /api/mihomo/data/update
func MihomoDataUpdateHandler(c *gin.Context) {
	if isMihomoUpdateBusy() {
		c.JSON(200, gin.H{"code": 1, "msg": "已有更新任务正在进行", "data": getMihomoUpdateStatus()})
		return
	}
	setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) {
		*s = MihomoDataUpdateStatus{
			State:     "downloading",
			Msg:       "正在准备下载...",
			FileTotal: len(mihomoDataFiles),
			StartedAt: time.Now().Format(time.RFC3339),
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	mihomoUpdateCancelMu.Lock()
	mihomoUpdateCancel = cancel
	mihomoUpdateCancelMu.Unlock()
	go runMihomoDataUpdate(ctx, cancel)
	c.JSON(200, gin.H{"code": 0, "msg": "已开始下载", "data": getMihomoUpdateStatus()})
}

func runMihomoDataUpdate(ctx context.Context, cancel context.CancelFunc) {
	defer func() {
		mihomoUpdateCancelMu.Lock()
		mihomoUpdateCancel = nil
		mihomoUpdateCancelMu.Unlock()
	}()
	dir := getMihomoDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) {
			s.State = "failed"
			s.Msg = "创建目录失败: " + err.Error()
		})
		return
	}
	total := len(mihomoDataFiles)
	for i, f := range mihomoDataFiles {
		if ctx.Err() != nil {
			setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) { s.State = "canceled"; s.Msg = "已取消"; s.Percent = 0 })
			return
		}
		setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) {
			s.FileName = f.Name
			s.FileIndex = i + 1
			s.FileTotal = total
			s.Downloaded = 0
			s.Total = 0
			s.Percent = 0
			s.Msg = fmt.Sprintf("[%d/%d] 正在下载 %s", i+1, total, f.Name)
		})
		err := mihomoDownloadFile(ctx, mihomoDataBaseURL+f.Name, filepath.Join(dir, f.Name), func(d, t int64) {
			setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) { s.Downloaded = d; s.Total = t })
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) { s.State = "canceled"; s.Msg = "已取消"; s.Percent = 0 })
				return
			}
			setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) {
				s.State = "failed"
				s.Msg = fmt.Sprintf("下载 %s 失败: %s", f.Name, err.Error())
			})
			return
		}
		slog.Info("[mihomo] 数据文件下载完成", "file", f.Name)
	}
	// 写版本号
	if remoteVersion, err := mihomoFetchText(mihomoVersionFileURL); err == nil && remoteVersion != "" {
		_ = os.WriteFile(filepath.Join(dir, "data_version.txt"), []byte(remoteVersion), 0644)
	}
	// 若在运行则重载 ipset
	if running, _ := isMihomoRunning(dir); running {
		setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) { s.Msg = "下载完成，正在重载 ipset..." })
		if out, err := runMihomoMmSh(dir, "reload-ipset"); err != nil {
			slog.Warn("[mihomo] reload-ipset 失败", "err", err.Error(), "out", out)
		}
	}
	setMihomoUpdateStatus(func(s *MihomoDataUpdateStatus) {
		s.State = "done"
		s.Msg = "所有数据文件更新完成"
		s.Percent = 100
		s.FileName = ""
	})
}

// ─────────────────────────── Handlers: 二进制安装管理 ───────────────────────────

// MihomoCheckBinaryVersionHandler GET /api/mihomo/binary/version
func MihomoCheckBinaryVersionHandler(c *gin.Context) {
	remoteVersion, err := mihomoFetchText(mihomoInstallVersionURL)
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "获取最新版本失败: " + err.Error()})
		return
	}
	localVersion := getMihomoInstalledVersion(getMihomoDir())

	remoteTag := extractMihomoVersionTag(remoteVersion)
	localTag := extractMihomoVersionTag(localVersion)

	// 只有当能从两边都提取出版本号且不相等时，才认为有更新；
	// 任何一边为空都视为"未知"，不报 has_update（避免空 local 永远触发更新提示）。
	hasUpdate := remoteTag != "" && localTag != "" && remoteTag != localTag

	c.JSON(200, gin.H{
		"code": 0, "msg": "ok",
		"data": gin.H{
			"local_version":  localVersion,
			"remote_version": remoteVersion,
			"local_tag":      localTag,
			"remote_tag":     remoteTag,
			"has_update":     hasUpdate,
		},
	})
}

// extractMihomoVersionTag 从一段任意字符串里提取形如 v1.19.1 或 alpha-xxxxxxx 的版本片段
var mihomoVersionRe = regexp.MustCompile(`(v\d+\.\d+\.\d+(?:-[A-Za-z0-9.]+)?|alpha-[A-Fa-f0-9]+|beta-[A-Fa-f0-9]+)`)

func extractMihomoVersionTag(s string) string {
	if s == "" {
		return ""
	}
	m := mihomoVersionRe.FindString(s)
	return strings.TrimSpace(m)
}

// MihomoInstallStatusHandler GET /api/mihomo/install/status
func MihomoInstallStatusHandler(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": getMihomoInstallStatus()})
}

// MihomoInstallCancelHandler POST /api/mihomo/install/cancel
func MihomoInstallCancelHandler(c *gin.Context) {
	if !isMihomoInstallBusy() {
		c.JSON(200, gin.H{"code": 1, "msg": "当前无安装任务", "data": getMihomoInstallStatus()})
		return
	}
	mihomoInstallCancelMu.Lock()
	cancel := mihomoInstallCancel
	mihomoInstallCancelMu.Unlock()
	if cancel == nil {
		c.JSON(200, gin.H{"code": 1, "msg": "没有可取消的任务"})
		return
	}
	cancel()
	c.JSON(200, gin.H{"code": 0, "msg": "已取消", "data": getMihomoInstallStatus()})
}

// MihomoInstallHandler POST /api/mihomo/install
func MihomoInstallHandler(c *gin.Context) {
	if isMihomoInstallBusy() {
		c.JSON(200, gin.H{"code": 1, "msg": "已有安装任务正在进行", "data": getMihomoInstallStatus()})
		return
	}
	setMihomoInstallStatus(func(s *MihomoInstallStatus) {
		*s = MihomoInstallStatus{State: "downloading", Msg: "正在准备...", StartedAt: time.Now().Format(time.RFC3339)}
	})
	ctx, cancel := context.WithCancel(context.Background())
	mihomoInstallCancelMu.Lock()
	mihomoInstallCancel = cancel
	mihomoInstallCancelMu.Unlock()
	go runMihomoInstall(ctx, cancel)
	c.JSON(200, gin.H{"code": 0, "msg": "已开始安装", "data": getMihomoInstallStatus()})
}

func runMihomoInstall(ctx context.Context, cancel context.CancelFunc) {
	defer func() {
		mihomoInstallCancelMu.Lock()
		mihomoInstallCancel = nil
		mihomoInstallCancelMu.Unlock()
	}()

	dir := getMihomoDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "failed"; s.Msg = "创建目录失败: " + err.Error() })
		return
	}

	// 下载 mm.sh（仅在不存在时）
	mmScript := filepath.Join(dir, "mm.sh")
	if _, err := os.Stat(mmScript); os.IsNotExist(err) {
		setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.Msg = "正在下载 mm.sh..." })
		if err := mihomoDownloadFile(ctx, mihomoInstallMmShURL, mmScript, nil); err != nil {
			slog.Warn("[mihomo] 下载 mm.sh 失败", "err", err.Error())
		} else {
			_ = os.Chmod(mmScript, 0755)
		}
	}

	if ctx.Err() != nil {
		setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "canceled"; s.Msg = "已取消"; s.Percent = 0 })
		return
	}

	// 下载二进制
	tmpBin := filepath.Join(dir, "mihomo.download.tmp")
	setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.Msg = "正在下载 mihomo 二进制..." })
	err := mihomoDownloadFile(ctx, mihomoInstallBinaryURL, tmpBin, func(d, t int64) {
		setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.Downloaded = d; s.Total = t })
	})
	if err != nil {
		_ = os.Remove(tmpBin)
		if errors.Is(err, context.Canceled) {
			setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "canceled"; s.Msg = "已取消"; s.Percent = 0 })
			return
		}
		setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "failed"; s.Msg = "下载失败: " + err.Error() })
		return
	}

	setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "installing"; s.Msg = "正在安装..."; s.Percent = 100 })

	// 若在运行先停止
	running, _ := isMihomoRunning(dir)
	if running {
		if out, err := runMihomoMmSh(dir, "stop"); err != nil {
			slog.Warn("[mihomo] 安装前停止失败", "err", err.Error(), "out", out)
		}
	}

	// 备份旧二进制并替换
	binPath := filepath.Join(dir, "mihomo")
	if _, err := os.Stat(binPath); err == nil {
		_ = os.Rename(binPath, binPath+".bak")
	}
	if err := os.Rename(tmpBin, binPath); err != nil {
		_ = os.Remove(tmpBin)
		setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "failed"; s.Msg = "安装失败: " + err.Error() })
		return
	}
	_ = os.Chmod(binPath, 0755)

	// 若之前在运行则重启
	if running {
		if out, err := runMihomoMmSh(dir, "start"); err != nil {
			slog.Warn("[mihomo] 安装后启动失败", "err", err.Error(), "out", out)
		}
	}

	setMihomoInstallStatus(func(s *MihomoInstallStatus) { s.State = "done"; s.Msg = "安装完成"; s.Percent = 100 })
}

// MihomoUninstallHandler POST /api/mihomo/uninstall
// body: {"mode": "soft"} 只删二进制  |  {"mode": "full"} 删整个目录
func MihomoUninstallHandler(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Mode == "" {
		req.Mode = "soft"
	}

	dir := getMihomoDir()

	// 先停止
	if running, _ := isMihomoRunning(dir); running {
		if _, err := runMihomoMmSh(dir, "stop"); err != nil {
			slog.Warn("[mihomo] 卸载前停止失败", "err", err.Error())
		}
	}

	if req.Mode == "full" {
		if err := os.RemoveAll(dir); err != nil {
			c.JSON(200, gin.H{"code": 1, "msg": "删除失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 0, "msg": "已删除全部 mihomo 文件"})
		return
	}

	binPath := filepath.Join(dir, "mihomo")
	_ = os.Remove(binPath + ".bak")
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		c.JSON(200, gin.H{"code": 1, "msg": "删除二进制失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "已删除 mihomo 二进制"})
}

// ─────────────────────────── Handlers: 配置文件 ───────────────────────────

// MihomoGetConfigHandler GET /api/mihomo/config
func MihomoGetConfigHandler(c *gin.Context) {
	configPath := filepath.Join(getMihomoDir(), "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"content": "", "exists": false}})
			return
		}
		c.JSON(200, gin.H{"code": 1, "msg": "读取配置失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"content": string(data), "exists": true}})
}

// MihomoSaveConfigHandler PUT /api/mihomo/config
func MihomoSaveConfigHandler(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	dir := getMihomoDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "创建目录失败: " + err.Error()})
		return
	}
	configPath := filepath.Join(dir, "config.yaml")
	// 备份旧配置
	if existing, err := os.ReadFile(configPath); err == nil {
		_ = os.WriteFile(configPath+".bak", existing, 0644)
	}
	if err := os.WriteFile(configPath, []byte(req.Content), 0644); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "保存失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "配置已保存"})
}

// ─────────────────────────── 开机自启 ───────────────────────────

const mihomoAutostartMarker = ".autostart"

func getMihomoAutostartEnabled(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, mihomoAutostartMarker))
	return err == nil
}

// InitMihomoAutostart 在 webssh 启动时检查自启标记，若存在则后台启动 mihomo。
func InitMihomoAutostart() {
	dir := getMihomoDir()
	if !getMihomoAutostartEnabled(dir) {
		return
	}
	go func() {
		mmSh := filepath.Join(dir, "mm.sh")
		if _, err := os.Stat(mmSh); err != nil {
			slog.Warn("mihomo autostart: mm.sh not found", "path", mmSh)
			return
		}
		slog.Info("mihomo autostart: starting mihomo via mm.sh")
		cmd := exec.Command(mmSh, "start")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Warn("mihomo autostart: start failed", "err", err, "out", string(out))
		} else {
			slog.Info("mihomo autostart: started", "out", strings.TrimSpace(string(out)))
		}
	}()
}

// MihomoGetAutostartHandler GET /api/mihomo/autostart
func MihomoGetAutostartHandler(c *gin.Context) {
	dir := getMihomoDir()
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"enabled": getMihomoAutostartEnabled(dir)}})
}

// MihomoSetAutostartHandler POST /api/mihomo/autostart  {"enabled": true/false}
func MihomoSetAutostartHandler(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	dir := getMihomoDir()
	marker := filepath.Join(dir, mihomoAutostartMarker)
	if req.Enabled {
		if err := os.MkdirAll(dir, 0755); err != nil {
			c.JSON(200, gin.H{"code": 1, "msg": "创建目录失败: " + err.Error()})
			return
		}
		if err := os.WriteFile(marker, []byte(""), 0644); err != nil {
			c.JSON(200, gin.H{"code": 1, "msg": "写入标记失败: " + err.Error()})
			return
		}
	} else {
		if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
			c.JSON(200, gin.H{"code": 1, "msg": "删除标记失败: " + err.Error()})
			return
		}
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"enabled": req.Enabled}})
}
