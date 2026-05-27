package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"gossh/app/config"
	"gossh/app/middleware"
	"gossh/app/model"
	"gossh/app/service"
	"gossh/gin"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var version = "dev"

const (
	GithubRepo            = "Jack-bin183/WebSSH-u60pro"
	updateConnectTimeout  = 3 * time.Second
	updateVersionFileURL  = "https://raw.githubusercontent.com/" + GithubRepo + "/version/version.txt"
	updateReleaseURL      = "https://github.com/" + GithubRepo + "/releases/latest"
	updateDownloadBaseURL = "https://github.com/" + GithubRepo + "/releases/latest/download/webssh_"
)

type GithubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type UpdateVersionInfo struct {
	CurrentVersion string   `json:"current_version"`
	LatestVersion  string   `json:"latest_version"`
	HasUpdate      bool     `json:"has_update"`
	ReleaseURL     string   `json:"release_url"`
	ReleaseName    string   `json:"release_name"`
	ReleaseBody    string   `json:"release_body"`
	AssetName      string   `json:"asset_name"`
	AssetSize      int64    `json:"asset_size"`
	ProxyURLs      []string `json:"proxy_urls"`
}

type UpdateDownloadStatus struct {
	State          string `json:"state"`
	Msg            string `json:"msg"`
	Mode           string `json:"mode"`
	Domain         string `json:"domain"`
	URL            string `json:"url"`
	AssetName      string `json:"asset_name"`
	Downloaded     int64  `json:"downloaded"`
	Total          int64  `json:"total"`
	Percent        int    `json:"percent"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseURL     string `json:"release_url"`
	StartedAt      string `json:"started_at"`
	UpdatedAt      string `json:"updated_at"`
}

var updateStatusMu sync.RWMutex
var updateStatus = UpdateDownloadStatus{State: "idle", Msg: "暂无更新任务"}
var updateCancelMu sync.Mutex
var updateCancel context.CancelFunc

func getUpdateStatus() UpdateDownloadStatus {
	updateStatusMu.RLock()
	defer updateStatusMu.RUnlock()
	return updateStatus
}

func setUpdateStatus(fn func(*UpdateDownloadStatus)) UpdateDownloadStatus {
	updateStatusMu.Lock()
	defer updateStatusMu.Unlock()
	fn(&updateStatus)
	updateStatus.UpdatedAt = time.Now().Format(time.RFC3339)
	if updateStatus.Total > 0 {
		updateStatus.Percent = int(updateStatus.Downloaded * 100 / updateStatus.Total)
		if updateStatus.Percent > 100 {
			updateStatus.Percent = 100
		}
	}
	return updateStatus
}

func isUpdateBusy() bool {
	state := getUpdateStatus().State
	return state == "starting" || state == "downloading" || state == "installing" || state == "restarting"
}

func setUpdateCancel(cancel context.CancelFunc) {
	updateCancelMu.Lock()
	defer updateCancelMu.Unlock()
	updateCancel = cancel
}

func clearUpdateCancel() {
	updateCancelMu.Lock()
	defer updateCancelMu.Unlock()
	updateCancel = nil
}

// 使用go 1.16+ 新特性
//
//go:embed webroot
var dir embed.FS

// StaticFile 嵌入普通的静态资源
type StaticFile struct {
	// 静态资源
	embedFS embed.FS

	// 设置embed文件到静态资源的相对路径，也就是embed注释里的路径
	path string
}

// Open 静态资源被访问的核心逻辑
func (w StaticFile) Open(name string) (fs.File, error) {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, errors.New("http: invalid character in file path")
	}

	fullName := filepath.Join(w.path, filepath.FromSlash(path.Clean("/"+name)))
	fullName = strings.ReplaceAll(fullName, `\`, `/`)
	file, err := w.embedFS.Open(fullName)
	return file, err
}

func getLatestVersionFromFile(proxies []string) (string, error) {
	client := updateHTTPClient()
	var lastErr error

	for _, u := range buildUpdateTryURLs(updateVersionFileURL, proxies) {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("User-Agent", "WebSSH-u60pro-Updater")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s 返回异常: %s", u, resp.Status)
			resp.Body.Close()
			continue
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		latestVersion := strings.TrimSpace(string(data))
		if latestVersion == "" {
			lastErr = fmt.Errorf("version.txt 内容为空")
			continue
		}
		if !isSafeUpdateVersion(latestVersion) {
			lastErr = fmt.Errorf("version.txt 版本号格式不正确: %s", latestVersion)
			continue
		}

		return latestVersion, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的更新线路")
	}
	return "", fmt.Errorf("已尝试代理和 raw.githubusercontent.com 兜底: %w", lastErr)
}

func isSafeUpdateVersion(v string) bool {
	if len(v) > 128 {
		return false
	}
	for _, r := range v {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '.', '_', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func updateAssetForVersion(latestVersion string) GithubAsset {
	return GithubAsset{
		Name:               "webssh_" + latestVersion,
		Size:               0,
		BrowserDownloadURL: updateDownloadBaseURL + latestVersion,
	}
}

func UpdateVersionHandler(c *gin.Context) {
	updateProxies, err := updateProxiesFromRaw(c.Query("proxy_url"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  err.Error(),
		})
		return
	}

	latestVersion, err := getLatestVersionFromFile(updateProxies)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "获取 version.txt 失败: " + err.Error(),
		})
		return
	}

	currentVersion := strings.TrimSpace(version)
	asset := updateAssetForVersion(latestVersion)

	info := UpdateVersionInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      currentVersion != latestVersion,
		ReleaseURL:     updateReleaseURL,
		ReleaseName:    "WebSSH " + latestVersion,
		ProxyURLs:      append([]string(nil), PROXIES...),
		AssetName:      asset.Name,
		AssetSize:      asset.Size,
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": info,
	})
}

var PROXIES = []string{
	"https://v6.gh-proxy.org/",
	"https://ghfast.top/",
	"https://gh-proxy.com/",
	"https://ghproxy.net/",
	"https://gh.llkk.cc/",
	"https://hub.gitmirror.com/",
	"https://gh-proxy.org/",
}

func updateHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   updateConnectTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = updateConnectTimeout
	transport.ResponseHeaderTimeout = 10 * time.Second

	return &http.Client{Transport: transport}
}

func updateTryURLs(originalURL string) []string {
	return buildUpdateTryURLs(originalURL, PROXIES)
}

func buildUpdateTryURLs(originalURL string, proxies []string) []string {
	trimmedURL := strings.TrimPrefix(originalURL, "https://")
	trimmedURL = strings.TrimPrefix(trimmedURL, "http://")

	urls := make([]string, 0, len(proxies)+1)
	seen := make(map[string]struct{}, len(proxies)*2+1)
	addURL := func(rawURL string) {
		if _, ok := seen[rawURL]; ok {
			return
		}
		seen[rawURL] = struct{}{}
		urls = append(urls, rawURL)
	}
	for _, proxy := range proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		if !strings.HasSuffix(proxy, "/") {
			proxy += "/"
		}
		addURL(proxy + originalURL)
		addURL(proxy + trimmedURL)
	}
	addURL(originalURL)
	return urls
}

func normalizeUpdateProxy(rawProxy string) (string, error) {
	proxy := strings.TrimSpace(rawProxy)
	if proxy == "" {
		return "", nil
	}
	if !strings.HasPrefix(proxy, "http://") && !strings.HasPrefix(proxy, "https://") {
		proxy = "https://" + proxy
	}
	parsed, err := url.Parse(proxy)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("代理 URL 格式不正确")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("代理 URL 只支持 http 或 https")
	}
	if !strings.HasSuffix(proxy, "/") {
		proxy += "/"
	}
	return proxy, nil
}

func updateProxiesFromRaw(rawProxy string) ([]string, error) {
	updateProxies := append([]string(nil), PROXIES...)
	proxyURL, err := normalizeUpdateProxy(rawProxy)
	if err != nil {
		return nil, err
	}
	if proxyURL != "" {
		updateProxies = []string{proxyURL}
	}
	return updateProxies, nil
}

func updateAttemptStatus(rawURL string, originalURL string) {
	parsed, _ := url.Parse(rawURL)
	mode := "代理"
	originalHost := ""
	if originalParsed, err := url.Parse(originalURL); err == nil {
		originalHost = originalParsed.Host
	}
	if parsed != nil && parsed.Host == originalHost {
		mode = "直连"
	}

	setUpdateStatus(func(status *UpdateDownloadStatus) {
		status.State = "downloading"
		status.Msg = "正在下载更新文件"
		status.Mode = mode
		if parsed != nil {
			status.Domain = parsed.Host
		}
		status.URL = rawURL
		status.Downloaded = 0
		status.Percent = 0
	})
}

// downloadFile 代理优先，GitHub 直连兜底，并记录下载进度。
// 实际下载前会先并行探测各代理的速度（Range 100KB），按吞吐排序后再依次尝试。
func downloadFile(ctx context.Context, downloadURL string, savePath string, totalHint int64, proxies []string) error {
	var lastErr error
	client := updateHTTPClient()
	rankedProxies := service.RankProxiesBySpeed(proxies, downloadURL)
	for _, u := range buildUpdateTryURLs(downloadURL, rankedProxies) {
		if err := ctx.Err(); err != nil {
			return err
		}
		updateAttemptStatus(u, downloadURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("User-Agent", "WebSSH-u60pro-Updater")

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			setUpdateStatus(func(status *UpdateDownloadStatus) {
				status.Msg = "当前线路 3 秒内未连上或连接失败，正在尝试下一线路"
			})
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
			setUpdateStatus(func(status *UpdateDownloadStatus) {
				status.Msg = fmt.Sprintf("当前线路返回 HTTP %d，正在尝试下一线路", resp.StatusCode)
			})
			continue
		}

		out, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			resp.Body.Close()
			return err
		}

		total := totalHint
		if resp.ContentLength > 0 {
			total = resp.ContentLength
		}
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.Total = total
			status.Downloaded = 0
			status.Percent = 0
			status.Msg = "正在下载更新文件"
		})

		buf := make([]byte, 64*1024)
		var downloaded int64
		for {
			if err := ctx.Err(); err != nil {
				_ = out.Close()
				resp.Body.Close()
				return err
			}
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				written, writeErr := out.Write(buf[:n])
				downloaded += int64(written)
				setUpdateStatus(func(status *UpdateDownloadStatus) {
					status.Downloaded = downloaded
				})
				if writeErr != nil {
					err = writeErr
					break
				}
				if written != n {
					err = io.ErrShortWrite
					break
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				err = readErr
				break
			}
		}
		resp.Body.Close()
		out.Close()
		if err != nil {
			lastErr = err
			setUpdateStatus(func(status *UpdateDownloadStatus) {
				status.Msg = "当前线路下载中断，正在尝试下一线路"
			})
			continue
		}

		info, err := os.Stat(savePath)
		if err != nil || info.Size() <= 0 {
			lastErr = fmt.Errorf("下载文件为空")
			continue
		}

		// 成功下载
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.State = "installing"
			status.Msg = "下载完成，正在准备替换程序"
			status.Downloaded = info.Size()
			if status.Total <= 0 {
				status.Total = info.Size()
			}
			status.Percent = 100
		})
		return nil
	}

	return fmt.Errorf("下载失败，尝试代理和 GitHub 兜底都失败: %v", lastErr)
}

func createTempUpdateScript(currentBin string, newBin string, logFile string, args []string) (string, error) {
	pid := os.Getpid()

	workDir, err := os.Getwd()
	if err != nil {
		workDir = filepath.Dir(currentBin)
	}

	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("webssh_update_%d.sh", time.Now().UnixNano()))

	quotedArgs := ""
	for _, arg := range args[1:] {
		quotedArgs += " " + shellQuote(arg)
	}

	content := fmt.Sprintf(`#!/bin/sh

LOG_FILE=%s
OLD_PID=%d
CURRENT_BIN=%s
NEW_BIN=%s
WORK_DIR=%s
ARGS=%s

echo "==============================" >> "$LOG_FILE"
echo "WebSSH 更新开始: $(date)" >> "$LOG_FILE"
echo "旧进程 PID: $OLD_PID" >> "$LOG_FILE"
echo "当前二进制: $CURRENT_BIN" >> "$LOG_FILE"
echo "新二进制: $NEW_BIN" >> "$LOG_FILE"

sleep 1

echo "停止旧进程..." >> "$LOG_FILE"
kill "$OLD_PID" >> "$LOG_FILE" 2>&1 || true

sleep 1

if kill -0 "$OLD_PID" 2>/dev/null; then
  echo "旧进程仍存在，强制结束..." >> "$LOG_FILE"
  kill -9 "$OLD_PID" >> "$LOG_FILE" 2>&1 || true
fi

if [ ! -s "$NEW_BIN" ]; then
  echo "新二进制不存在或为空，更新终止" >> "$LOG_FILE"
  rm -f "$0"
  exit 1
fi

chmod +x "$NEW_BIN"

echo "备份旧二进制..." >> "$LOG_FILE"
if [ -f "$CURRENT_BIN" ]; then
  cp "$CURRENT_BIN" "$CURRENT_BIN.bak" >> "$LOG_FILE" 2>&1 || true
fi

echo "替换二进制..." >> "$LOG_FILE"
mv "$NEW_BIN" "$CURRENT_BIN" >> "$LOG_FILE" 2>&1

chmod +x "$CURRENT_BIN"

echo "启动新进程..." >> "$LOG_FILE"
cd "$WORK_DIR" || cd /
nohup "$CURRENT_BIN" $ARGS >> /tmp/webssh_run.log 2>&1 &

echo "新进程已启动: $(date)" >> "$LOG_FILE"
echo "清理临时脚本" >> "$LOG_FILE"

rm -f "$0"
`, shellQuote(logFile), pid, shellQuote(currentBin), shellQuote(newBin), shellQuote(workDir), strconv.Quote(quotedArgs))

	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return "", err
	}

	return scriptPath, nil
}
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func UpdateStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": getUpdateStatus(),
	})
}

func runUpdateTask(ctx context.Context, cancel context.CancelFunc, asset *GithubAsset, latestVersion string, releaseURL string, proxies []string) {
	defer clearUpdateCancel()

	currentBin, err := os.Executable()
	if err != nil {
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.State = "failed"
			status.Msg = "获取当前二进制路径失败: " + err.Error()
		})
		return
	}

	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.State = "failed"
			status.Msg = "解析当前二进制真实路径失败: " + err.Error()
		})
		return
	}

	tmpNewBin := filepath.Join(os.TempDir(), fmt.Sprintf("webssh_%s_%s.new", runtime.GOARCH, latestVersion))
	logFile := filepath.Join(os.TempDir(), "webssh_update.log")

	if err := downloadFile(ctx, asset.BrowserDownloadURL, tmpNewBin, asset.Size, proxies); err != nil {
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			if errors.Is(err, context.Canceled) {
				status.State = "canceled"
				status.Msg = "用户已取消下载"
				status.Percent = 0
				return
			}
			status.State = "failed"
			status.Msg = "下载新版本失败: " + err.Error()
		})
		_ = os.Remove(tmpNewBin)
		return
	}

	if err := os.Chmod(tmpNewBin, 0755); err != nil {
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.State = "failed"
			status.Msg = "设置新二进制权限失败: " + err.Error()
		})
		return
	}

	scriptPath, err := createTempUpdateScript(currentBin, tmpNewBin, logFile, os.Args)
	if err != nil {
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.State = "failed"
			status.Msg = "创建临时更新脚本失败: " + err.Error()
		})
		return
	}

	cmd := exec.Command("/bin/sh", scriptPath)
	if err := cmd.Start(); err != nil {
		setUpdateStatus(func(status *UpdateDownloadStatus) {
			status.State = "failed"
			status.Msg = "启动临时更新脚本失败: " + err.Error()
		})
		return
	}

	setUpdateStatus(func(status *UpdateDownloadStatus) {
		status.State = "restarting"
		status.Msg = "更新文件已准备完成，程序即将重启"
		status.Percent = 100
		status.ReleaseURL = releaseURL
	})
}

type UpdateRunRequest struct {
	ProxyURL string `json:"proxy_url"`
}

func UpdateRunHandler(c *gin.Context) {
	if isUpdateBusy() {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "已有更新任务正在进行",
			"data": getUpdateStatus(),
		})
		return
	}

	var runReq UpdateRunRequest
	if err := c.ShouldBindJSON(&runReq); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "更新参数错误: " + err.Error(),
		})
		return
	}

	updateProxies, err := updateProxiesFromRaw(runReq.ProxyURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  err.Error(),
		})
		return
	}

	latestVersion, err := getLatestVersionFromFile(updateProxies)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "获取 version.txt 失败: " + err.Error(),
		})
		return
	}

	currentVersion := strings.TrimSpace(version)

	if currentVersion == latestVersion {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "当前已经是最新版本",
			"data": gin.H{
				"current_version": currentVersion,
				"latest_version":  latestVersion,
			},
		})
		return
	}

	asset := updateAssetForVersion(latestVersion)

	setUpdateStatus(func(status *UpdateDownloadStatus) {
		*status = UpdateDownloadStatus{
			State:          "starting",
			Msg:            "正在准备更新任务",
			AssetName:      asset.Name,
			Total:          asset.Size,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			ReleaseURL:     updateReleaseURL,
			StartedAt:      time.Now().Format(time.RFC3339),
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	setUpdateCancel(cancel)
	go runUpdateTask(ctx, cancel, &asset, latestVersion, updateReleaseURL, updateProxies)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "已开始下载更新文件",
		"data": getUpdateStatus(),
	})
}

func UpdateCancelHandler(c *gin.Context) {
	state := getUpdateStatus().State
	if state != "starting" && state != "downloading" {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "当前更新状态不可取消",
			"data": getUpdateStatus(),
		})
		return
	}

	updateCancelMu.Lock()
	cancel := updateCancel
	updateCancelMu.Unlock()
	if cancel == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "没有可取消的下载任务",
			"data": getUpdateStatus(),
		})
		return
	}

	cancel()
	setUpdateStatus(func(status *UpdateDownloadStatus) {
		status.Msg = "正在取消下载"
	})
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "已取消下载",
		"data": getUpdateStatus(),
	})
}

func OpenAdbHandler(c *gin.Context) {
	slog.Info("[API] /api/openadb 调用开始")

	cmd := exec.Command("/sbin/usb/compositions/usb_switch",
		"0x19d2", "0x1404",
		"rndis_gsi,diag,serial,modem,ffs,dpl,qdss",
		"MU5120ZTED0000000",
	)

	// 捕获 stdout 和 stderr
	output, err := cmd.CombinedOutput()

	// 即使 err != nil，也不直接认为失败，只记录日志
	if err != nil {
		slog.Warn("[API] openadb 执行返回非 0，但忽略错误",
			"err", err.Error(),
			"output", string(output),
		)
	} else {
		slog.Info("[API] openadb 执行成功", "output", string(output))
	}

	// 返回前端统一成功
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"success": true,
		"msg":     "ADB 命令已触发（注意：部分报错可忽略）",
		"output":  string(output),
	})
}

func initApplication() {
	config.InitConfig()
	model.InitDatabase()
	service.InitSessionClean()
	service.InitSshServer()
	service.InitMihomoAutostart()
	fmt.Printf("WebBaseDir:[%s]\n", config.DefaultConfig.WebBaseDir)
}

func main() {
	service.MaybeExecShellHelper()
	initApplication()

	gin.SetMode(gin.ReleaseMode)
	var engine = gin.Default()
	engine.MaxMultipartMemory = 8 << 20
	engine.Use(middleware.DbCheck(), middleware.NetFilter())
	engine.GET("/web_base_dir", func(c *gin.Context) { c.JSON(200, gin.H{"code": 0, "web_base_dir": config.DefaultConfig.WebBaseDir}) })

	appPath := config.DefaultConfig.WebBaseDir + "/app/"
	engine.GET(config.DefaultConfig.WebBaseDir+"/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, appPath)
	})
	engine.GET(config.DefaultConfig.WebBaseDir+"/app", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, appPath)
	})
	engine.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, appPath)
	})

	// 不需要认证的路由
	var open = engine.Group(config.DefaultConfig.WebBaseDir)
	open.StaticFS("/app", http.FS(StaticFile{embedFS: dir, path: "webroot"}))
	open.POST("/api/login", service.UserLogin)
	open.POST("/api/sys/db_conn_check", service.DbConnCheck)
	open.GET("/api/sys/is_init", service.GetIsInit)
	open.POST("/api/sys/init", service.SysInit)

	// 需要认证的路由
	var auth = engine.Group(config.DefaultConfig.WebBaseDir,
		middleware.SysInit(),
		middleware.JWTAuth(),
		middleware.PremCheck(engine),
	)

	{
		// UBUS 调用接口
		// auth.POST("/api/ubus", service.UbusAction)
		auth.POST("/api/ubus", service.ZteUbusBatchHandler)
		// WiFi高性能模型 查询和修改
		auth.POST("/api/wifi/psm/get", service.WifiPsmGetHandler)
		auth.POST("/api/wifi/psm/set", service.WifiPsmSetHandler)
		auth.POST("/api/wifi/state/set", service.WifiStateSetHandler)
		auth.POST("/api/net/ambr/get", service.NetAmbrGetHandler)
	}

	{ // SSH 连接配置
		auth.GET("/api/conn_conf", service.ConfFindAll)
		auth.GET("/api/conn_conf/:id", service.ConfFindByID)
		auth.POST("/api/conn_conf", service.ConfCreate)
		auth.PUT("/api/conn_conf", service.ConfUpdateById)
		auth.DELETE("/api/conn_conf/:id", service.ConfDeleteById)
	}

	{ // 命令收藏
		auth.GET("/api/cmd_note", service.CmdNoteFindAll)
		auth.GET("/api/cmd_note/:id", service.CmdNoteFindByID)
		auth.POST("/api/cmd_note", service.CmdNoteCreate)
		auth.PUT("/api/cmd_note", service.CmdNoteUpdateById)
		auth.DELETE("/api/cmd_note/:id", service.CmdNoteDeleteById)
	}

	{ // 策略配置
		auth.GET("/api/policy_conf", service.PolicyConfFindAll)
		auth.GET("/api/policy_conf/:id", service.PolicyConfFindByID)
		auth.POST("/api/policy_conf", service.PolicyConfCreate)
		auth.PUT("/api/policy_conf", service.PolicyConfUpdateById)
		auth.DELETE("/api/policy_conf/:id", service.PolicyConfDeleteById)
	}

	{ // 访问控制
		auth.GET("/api/net_filter", service.NetFilterFindAll)
		auth.GET("/api/net_filter/:id", service.NetFilterFindByID)
		auth.POST("/api/net_filter", service.NetFilterCreate)
		auth.PUT("/api/net_filter", service.NetFilterUpdateById)
		auth.DELETE("/api/net_filter/:id", service.NetFilterDeleteById)
	}

	{ // Web用户管理
		auth.GET("/api/user", service.UserFindAll)
		auth.GET("/api/user/:id", service.UserFindByID)
		auth.POST("/api/user", service.UserCreate)
		auth.PUT("/api/user", service.UserUpdateById)
		auth.DELETE("/api/user/:id", service.UserDeleteById)
		auth.PATCH("/api/user/check_name_exists", service.CheckUserNameExists)
		auth.PATCH("/api/user/pwd", service.ModifyPasswd)
	}

	{ // SSHD用户管理
		auth.GET("/api/sshd_user", service.SshdUserFindAll)
		auth.GET("/api/sshd_user/:id", service.SshdUserFindByID)
		auth.POST("/api/sshd_user", service.SshdUserCreate)
		auth.PUT("/api/sshd_user", service.SshdUserUpdateById)
		auth.DELETE("/api/sshd_user/:id", service.SshdUserDeleteById)
		auth.PATCH("/api/sshd_user/check_name_exists", service.CheckSshdUserNameExists)
	}

	{ // SSHD证书管理
		auth.GET("/api/sshd_cert", service.SshdCertFindAll)
		auth.GET("/api/sshd_cert_text", service.GetSshdCertAuthorizedKeys)
		auth.GET("/api/sshd_cert/:id", service.SshdCertFindByID)
		auth.POST("/api/sshd_cert", service.SshdCertCreate)
		auth.PUT("/api/sshd_cert", service.SshdCertUpdateById)
		auth.DELETE("/api/sshd_cert/:id", service.SshdCertDeleteById)
		auth.PATCH("/api/sshd_cert/check_name_exists", service.CheckSshdCertNameExists)
	}

	{ // 审计日志
		auth.POST("/api/login_audit", service.LoginAuditSearch)
	}

	{ // SSH链接
		auth.GET("/api/conn_manage/online_client", service.GetOnlineClient)
		auth.PUT("/api/conn_manage/refresh_conn_time", service.RefreshConnTime)
		auth.POST("/api/sftp/create_dir", service.SftpCreateDir)
		auth.POST("/api/sftp/create_file", service.SftpCreateFile)
		auth.POST("/api/sftp/list", service.SftpList)
		auth.GET("/api/sftp/download", service.SftpDownLoad)
		auth.PUT("/api/sftp/upload", service.SftpUpload)
		auth.DELETE("/api/sftp/delete", service.SftpDelete)
		auth.PATCH("/api/sftp/rename", service.SftpRename)
		auth.PATCH("/api/sftp/chmod", service.SftpChmod)
		auth.POST("/api/sftp/read", service.SftpReadFile)
		auth.PUT("/api/sftp/save", service.SftpSaveFile)
		auth.POST("/api/sftp/compress", service.SftpCompressDir)
		auth.POST("/api/sftp/extract", service.SftpExtractArchive)
		auth.GET("/api/ssh/conn", service.NewSshConn)
		auth.PATCH("/api/ssh/conn", service.ResizeWindow)
		auth.POST("/api/ssh/exec", service.ExecCommand)
		auth.POST("/api/ssh/disconnect", service.Disconnect)
		auth.POST("/api/ssh/create_session", service.CreateSessionId)
	}

	{ // 系统配置
		auth.GET("/api/sys/config", service.GetRunConf)
		auth.POST("/api/sys/config", service.SetRunConf)
	}

	{ // 系统更新
		auth.GET("/api/update/version", UpdateVersionHandler)
		auth.GET("/api/update/status", UpdateStatusHandler)
		auth.POST("/api/update/run", UpdateRunHandler)
		auth.POST("/api/update/cancel", UpdateCancelHandler)
	}
	{
		// 开启 ADB 等调试端口
		auth.POST("/api/openadb", OpenAdbHandler)
	}

	{ // Mihomo 透明代理管理
		auth.GET("/api/mihomo/status", service.MihomoStatusHandler)
		auth.GET("/api/mihomo/dir", service.MihomoGetDirHandler)
		auth.POST("/api/mihomo/control", service.MihomoControlHandler)
		auth.GET("/api/mihomo/data/version", service.MihomoDataVersionHandler)
		auth.POST("/api/mihomo/data/update", service.MihomoDataUpdateHandler)
		auth.GET("/api/mihomo/data/update/status", service.MihomoDataUpdateStatusHandler)
		auth.POST("/api/mihomo/data/update/cancel", service.MihomoDataUpdateCancelHandler)
		// 二进制安装管理
		auth.GET("/api/mihomo/binary/version", service.MihomoCheckBinaryVersionHandler)
		auth.GET("/api/mihomo/install/status", service.MihomoInstallStatusHandler)
		auth.POST("/api/mihomo/install/cancel", service.MihomoInstallCancelHandler)
		auth.POST("/api/mihomo/install", service.MihomoInstallHandler)
		auth.POST("/api/mihomo/uninstall", service.MihomoUninstallHandler)
		// 配置文件管理
		auth.GET("/api/mihomo/config", service.MihomoGetConfigHandler)
		auth.PUT("/api/mihomo/config", service.MihomoSaveConfigHandler)
		// 开机自启
		auth.GET("/api/mihomo/autostart", service.MihomoGetAutostartHandler)
		auth.POST("/api/mihomo/autostart", service.MihomoSetAutostartHandler)
	}

	address := fmt.Sprintf("%s:%s", config.DefaultConfig.Address, config.DefaultConfig.Port)
	_, certErr := os.Open(config.DefaultConfig.CertFile)
	_, keyErr := os.Open(config.DefaultConfig.KeyFile)

	// 如果证书和私钥文件存在,就使用https协议,否则使用http协议
	if certErr == nil && keyErr == nil {
		slog.Info("https_server_start", "address", address)
		err := engine.RunTLS(address, config.DefaultConfig.CertFile, config.DefaultConfig.KeyFile)
		if err != nil {
			slog.Error("RunServeTLSError:", "msg", err.Error())
			os.Exit(1)
			return
		}
	} else {
		slog.Info("http_server_start", "address", address)
		err := engine.Run(address)
		if err != nil {
			slog.Error("RunServeError:", "msg", err.Error())
			os.Exit(1)
			return
		}
	}
}
