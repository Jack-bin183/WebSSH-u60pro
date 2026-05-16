package main

import (
	"embed"
	"encoding/json"
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
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var version = "dev"

const GithubRepo = "Jack-bin183/WebSSH-u60pro"

type GithubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GithubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Name    string        `json:"name"`
	Body    string        `json:"body"`
	Assets  []GithubAsset `json:"assets"`
}

type UpdateVersionInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseURL     string `json:"release_url"`
	ReleaseName    string `json:"release_name"`
	ReleaseBody    string `json:"release_body"`
	AssetName      string `json:"asset_name"`
	AssetSize      int64  `json:"asset_size"`
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

func getLatestGithubRelease() (*GithubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GithubRepo)

	client := &http.Client{
		Timeout: 12 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "WebSSH-u60pro-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api 返回异常: %s", resp.Status)
	}

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	if strings.TrimSpace(release.TagName) == "" {
		return nil, fmt.Errorf("github release tag 为空")
	}

	return &release, nil
}
func selectUpdateAsset(release *GithubRelease) (*GithubAsset, error) {
	if release == nil {
		return nil, fmt.Errorf("release 为空")
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)

		if strings.HasPrefix(name, "webssh_") {
			return &asset, nil
		}
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)

		if strings.Contains(name, "webssh") &&
			!strings.HasSuffix(name, ".txt") &&
			!strings.HasSuffix(name, ".sha256") &&
			!strings.HasSuffix(name, ".json") {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("没有找到可用的 webssh 二进制资产")
}
func UpdateVersionHandler(c *gin.Context) {
	release, err := getLatestGithubRelease()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "获取 GitHub 最新版本失败: " + err.Error(),
		})
		return
	}

	currentVersion := strings.TrimSpace(version)
	latestVersion := strings.TrimSpace(release.TagName)

	asset, assetErr := selectUpdateAsset(release)

	info := UpdateVersionInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      currentVersion != latestVersion,
		ReleaseURL:     release.HTMLURL,
		ReleaseName:    release.Name,
		ReleaseBody:    release.Body,
	}

	if assetErr == nil && asset != nil {
		info.AssetName = asset.Name
		info.AssetSize = asset.Size
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": info,
	})
}

var PROXIES = []string{
	"https://v6.gh-proxy.org/",
	"https://gh-proxy.org/",
	"https://hk.gh-proxy.org/",
	"https://cdn.gh-proxy.org/",
	"https://edgeone.gh-proxy.org/",
	"https://fastgit.cc/",
	"https://git.yylx.win/",
	"https://gh.llkk.cc/",
	"https://ghfast.top/",
}

// downloadFile 支持直连和代理，每次请求单独超时
func downloadFile(url string, savePath string) error {
	// 先构建尝试 URL 列表
	tryURLs := append([]string{url}, func() []string {
		var proxied []string
		for _, p := range PROXIES {
			trimmed := strings.TrimPrefix(url, "https://")
			if !strings.HasSuffix(p, "/") {
				p += "/"
			}
			proxied = append(proxied, p+trimmed)
		}
		return proxied
	}()...)

	var lastErr error
	for _, u := range tryURLs {
		client := &http.Client{Timeout: 15 * time.Second} // 每次单独 15 秒

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
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		out, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			resp.Body.Close()
			return err
		}

		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			lastErr = err
			continue
		}

		info, err := os.Stat(savePath)
		if err != nil || info.Size() <= 0 {
			lastErr = fmt.Errorf("下载文件为空")
			continue
		}

		// 成功下载
		return nil
	}

	return fmt.Errorf("下载失败，尝试直连和代理都失败: %v", lastErr)
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
func UpdateRunHandler(c *gin.Context) {
	release, err := getLatestGithubRelease()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "获取 GitHub 最新版本失败: " + err.Error(),
		})
		return
	}

	currentVersion := strings.TrimSpace(version)
	latestVersion := strings.TrimSpace(release.TagName)

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

	asset, err := selectUpdateAsset(release)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  err.Error(),
		})
		return
	}

	currentBin, err := os.Executable()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "获取当前二进制路径失败: " + err.Error(),
		})
		return
	}

	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "解析当前二进制真实路径失败: " + err.Error(),
		})
		return
	}

	tmpNewBin := filepath.Join(os.TempDir(), fmt.Sprintf("webssh_%s_%s.new", runtime.GOARCH, latestVersion))
	logFile := filepath.Join(os.TempDir(), "webssh_update.log")

	if err := downloadFile(asset.BrowserDownloadURL, tmpNewBin); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "下载新版本失败: " + err.Error(),
		})
		return
	}

	if err := os.Chmod(tmpNewBin, 0755); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "设置新二进制权限失败: " + err.Error(),
		})
		return
	}

	scriptPath, err := createTempUpdateScript(currentBin, tmpNewBin, logFile, os.Args)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "创建临时更新脚本失败: " + err.Error(),
		})
		return
	}

	cmd := exec.Command("/bin/sh", scriptPath)

	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  "启动临时更新脚本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "已开始更新，程序即将重启",
		"data": gin.H{
			"current_version": currentVersion,
			"latest_version":  latestVersion,
			"asset_name":      asset.Name,
			"asset_size":      asset.Size,
			"log_file":        logFile,
			"script":          scriptPath,
		},
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

	engine.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, config.DefaultConfig.WebBaseDir+"/app")
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
		auth.POST("/api/sftp/list", service.SftpList)
		auth.GET("/api/sftp/download", service.SftpDownLoad)
		auth.PUT("/api/sftp/upload", service.SftpUpload)
		auth.DELETE("/api/sftp/delete", service.SftpDelete)
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
		auth.POST("/api/update/run", UpdateRunHandler)
	}
	{
		// 开启 ADB 等调试端口
		auth.POST("/api/openadb", OpenAdbHandler)
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
