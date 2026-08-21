package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gossh/app/model"
	"gossh/gin"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ddnsGoDir             = "/data/plugins/ddns-go"
	ddnsGoBinaryName      = "ddns-go"
	ddnsGoConfigName      = ".ddns_go_config.yaml"
	ddnsGoManagedName     = "webssh-config.json"
	ddnsGoPIDName         = "ddns-go.pid"
	ddnsGoLogName         = "ddns-go.log"
	ddnsGoAutostartMarker = ".autostart"
	ddnsGoListen          = "127.0.0.1:9876"
	ddnsGoUpstream        = "http://127.0.0.1:9876"
	ddnsGoReleaseAPI      = "https://api.github.com/repos/jeessy2/ddns-go/releases/latest"
)

var ddnsGoMu sync.Mutex

type ddnsGoStatus struct {
	Installed        bool   `json:"installed"`
	Running          bool   `json:"running"`
	PID              int    `json:"pid"`
	Version          string `json:"version"`
	Dir              string `json:"dir"`
	ConfigPath       string `json:"config_path"`
	AutostartEnabled bool   `json:"autostart_enabled"`
	Listen           string `json:"listen"`
}

type ddnsGoRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// ddnsGoManagedConfig mirrors DDNS-GO's /save payload while deliberately
// excluding its username and password. Credentials are always supplied by the
// authenticated WebSSH user on the server side.
type ddnsGoManagedConfig struct {
	Name               string `json:"Name"`
	DnsName            string `json:"DnsName"`
	DnsID              string `json:"DnsID"`
	DnsSecret          string `json:"DnsSecret"`
	DnsExtParam        string `json:"DnsExtParam"`
	HttpInterface      string `json:"HttpInterface"`
	Ipv4Cmd            string `json:"Ipv4Cmd"`
	Ipv4Domains        string `json:"Ipv4Domains"`
	Ipv4Enable         bool   `json:"Ipv4Enable"`
	Ipv4GetType        string `json:"Ipv4GetType"`
	Ipv4NetInterface   string `json:"Ipv4NetInterface"`
	Ipv4URL            string `json:"Ipv4Url"`
	Ipv6Cmd            string `json:"Ipv6Cmd"`
	Ipv6Domains        string `json:"Ipv6Domains"`
	Ipv6Enable         bool   `json:"Ipv6Enable"`
	Ipv6GetType        string `json:"Ipv6GetType"`
	Ipv6NetInterface   string `json:"Ipv6NetInterface"`
	Ipv6Reg            string `json:"Ipv6Reg"`
	Ipv6URL            string `json:"Ipv6Url"`
	TTL                string `json:"TTL"`
	WebhookURL         string `json:"WebhookURL"`
	WebhookRequestBody string `json:"WebhookRequestBody"`
	WebhookHeaders     string `json:"WebhookHeaders"`
}

type ddnsGoYAMLConfig struct {
	DnsConf []struct {
		Name string `yaml:"name"`
		Ipv4 struct {
			Enable       bool     `yaml:"enable"`
			GetType      string   `yaml:"gettype"`
			URL          string   `yaml:"url"`
			NetInterface string   `yaml:"netinterface"`
			Cmd          string   `yaml:"cmd"`
			Domains      []string `yaml:"domains"`
		} `yaml:"ipv4"`
		Ipv6 struct {
			Enable       bool     `yaml:"enable"`
			GetType      string   `yaml:"gettype"`
			URL          string   `yaml:"url"`
			NetInterface string   `yaml:"netinterface"`
			Cmd          string   `yaml:"cmd"`
			Ipv6Reg      string   `yaml:"ipv6reg"`
			Domains      []string `yaml:"domains"`
		} `yaml:"ipv6"`
		DNS struct {
			Name     string `yaml:"name"`
			ID       string `yaml:"id"`
			Secret   string `yaml:"secret"`
			ExtParam string `yaml:"extparam"`
		} `yaml:"dns"`
		TTL           string `yaml:"ttl"`
		HttpInterface string `yaml:"httpinterface"`
	} `yaml:"dnsconf"`
	Webhook struct {
		WebhookURL         string `yaml:"webhookurl"`
		WebhookRequestBody string `yaml:"webhookrequestbody"`
		WebhookHeaders     string `yaml:"webhookheaders"`
	} `yaml:"webhook"`
}

func defaultDDNSGoManagedConfig() ddnsGoManagedConfig {
	return ddnsGoManagedConfig{
		Name: "默认", DnsName: "cloudflare", TTL: "300",
		Ipv4Enable: true, Ipv4GetType: "url",
		Ipv4URL:     "https://myip.ipip.net, https://ddns.oray.com/checkip",
		Ipv6GetType: "netInterface",
	}
}

func loadDDNSGoManagedConfig() (ddnsGoManagedConfig, error) {
	config := defaultDDNSGoManagedConfig()
	if data, err := os.ReadFile(ddnsGoPath(ddnsGoConfigName)); err == nil {
		var current ddnsGoYAMLConfig
		if err := yaml.Unmarshal(data, &current); err != nil {
			return config, fmt.Errorf("读取 DDNS-GO 当前配置失败: %w", err)
		}
		if len(current.DnsConf) > 0 {
			item := current.DnsConf[0]
			config.Name, config.DnsName = item.Name, item.DNS.Name
			config.DnsID, config.DnsSecret, config.DnsExtParam = item.DNS.ID, item.DNS.Secret, item.DNS.ExtParam
			config.TTL, config.HttpInterface = item.TTL, item.HttpInterface
			config.Ipv4Enable, config.Ipv4GetType = item.Ipv4.Enable, item.Ipv4.GetType
			config.Ipv4URL, config.Ipv4NetInterface, config.Ipv4Cmd = item.Ipv4.URL, item.Ipv4.NetInterface, item.Ipv4.Cmd
			config.Ipv4Domains = strings.Join(item.Ipv4.Domains, "\n")
			config.Ipv6Enable, config.Ipv6GetType = item.Ipv6.Enable, item.Ipv6.GetType
			config.Ipv6URL, config.Ipv6NetInterface, config.Ipv6Cmd = item.Ipv6.URL, item.Ipv6.NetInterface, item.Ipv6.Cmd
			config.Ipv6Reg, config.Ipv6Domains = item.Ipv6.Ipv6Reg, strings.Join(item.Ipv6.Domains, "\n")
			config.WebhookURL = current.Webhook.WebhookURL
			config.WebhookRequestBody = current.Webhook.WebhookRequestBody
			config.WebhookHeaders = current.Webhook.WebhookHeaders
			return config, nil
		}
	} else if !os.IsNotExist(err) {
		return config, err
	}
	data, err := os.ReadFile(ddnsGoPath(ddnsGoManagedName))
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("读取 DDNS-GO 管理配置失败: %w", err)
	}
	return config, nil
}

func saveDDNSGoManagedConfig(config ddnsGoManagedConfig) error {
	if err := os.MkdirAll(ddnsGoDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ddnsGoPath(ddnsGoManagedName), data, 0600)
}

func ddnsGoPath(name string) string { return filepath.Join(ddnsGoDir, name) }

func ddnsGoProcess() (int, bool) {
	data, err := os.ReadFile(ddnsGoPath(ddnsGoPIDName))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	process, err := os.FindProcess(pid)
	if err != nil || process.Signal(syscall.Signal(0)) != nil {
		_ = os.Remove(ddnsGoPath(ddnsGoPIDName))
		return 0, false
	}
	if executable, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe")); err == nil {
		if filepath.Clean(executable) != filepath.Clean(ddnsGoPath(ddnsGoBinaryName)) {
			_ = os.Remove(ddnsGoPath(ddnsGoPIDName))
			return 0, false
		}
	}
	return pid, true
}

func getDDNSGoStatus() ddnsGoStatus {
	_, binErr := os.Stat(ddnsGoPath(ddnsGoBinaryName))
	pid, running := ddnsGoProcess()
	version := ""
	if binErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, ddnsGoPath(ddnsGoBinaryName), "-v").CombinedOutput(); err == nil {
			version = strings.TrimSpace(string(out))
		}
	}
	_, autoErr := os.Stat(ddnsGoPath(ddnsGoAutostartMarker))
	return ddnsGoStatus{
		Installed: binErr == nil, Running: running, PID: pid, Version: version,
		Dir: ddnsGoDir, ConfigPath: ddnsGoPath(ddnsGoConfigName),
		AutostartEnabled: autoErr == nil, Listen: ddnsGoListen,
	}
}

func startDDNSGo() error {
	ddnsGoMu.Lock()
	defer ddnsGoMu.Unlock()
	if _, running := ddnsGoProcess(); running {
		return nil
	}
	bin := ddnsGoPath(ddnsGoBinaryName)
	if _, err := os.Stat(bin); err != nil {
		return errors.New("ddns-go 尚未安装")
	}
	if err := os.MkdirAll(ddnsGoDir, 0755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(ddnsGoPath(ddnsGoLogName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("打开 ddns-go 日志失败: %w", err)
	}
	cmd := exec.Command(bin, "-l", ddnsGoListen, "-c", ddnsGoPath(ddnsGoConfigName))
	cmd.Dir = ddnsGoDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("启动 ddns-go 失败: %w", err)
	}
	_ = logFile.Close()
	if err := os.WriteFile(ddnsGoPath(ddnsGoPIDName), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	pid := cmd.Process.Pid
	go func() {
		_ = cmd.Wait()
		ddnsGoMu.Lock()
		defer ddnsGoMu.Unlock()
		data, err := os.ReadFile(ddnsGoPath(ddnsGoPIDName))
		if err == nil && strings.TrimSpace(string(data)) == strconv.Itoa(pid) {
			_ = os.Remove(ddnsGoPath(ddnsGoPIDName))
		}
	}()
	return nil
}

func stopDDNSGo() error {
	ddnsGoMu.Lock()
	defer ddnsGoMu.Unlock()
	pid, running := ddnsGoProcess()
	if !running {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err == nil {
		_ = process.Signal(os.Interrupt)
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if _, alive := ddnsGoProcess(); !alive {
				return nil
			}
		}
		_ = process.Kill()
	}
	_ = os.Remove(ddnsGoPath(ddnsGoPIDName))
	return nil
}

func latestDDNSGoAsset() (string, string, error) {
	resp, err := mihomoHTTPClient().Get(ddnsGoReleaseAPI)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub releases 返回 HTTP %d", resp.StatusCode)
	}
	var release ddnsGoRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, "_linux_arm64.tar.gz") {
			return release.TagName, asset.URL, nil
		}
	}
	return "", "", errors.New("最新版本中未找到 linux_arm64 安装包")
}

func extractDDNSGoBinary(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != ddnsGoBinaryName {
			continue
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return errors.New("安装包内未找到 ddns-go 二进制")
}

func DDNSGoStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": getDDNSGoStatus()})
}

func DDNSGoUpdateCheckHandler(c *gin.Context) {
	status := getDDNSGoStatus()
	if !status.Installed {
		c.JSON(200, gin.H{"code": 1, "msg": "DDNS-GO 尚未安装"})
		return
	}
	latest, _, err := latestDDNSGoAsset()
	if err != nil {
		c.JSON(200, gin.H{"code": 2, "msg": "检查更新失败: " + err.Error()})
		return
	}
	currentVersion := strings.TrimPrefix(strings.TrimSpace(status.Version), "v")
	latestVersion := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"current": status.Version, "latest": latest, "has_update": currentVersion != latestVersion,
	}})
}

func DDNSGoLogsHandler(c *gin.Context) {
	file, err := os.Open(ddnsGoPath(ddnsGoLogName))
	if os.IsNotExist(err) {
		c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"logs": []string{}}})
		return
	}
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "读取日志失败: " + err.Error()})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "读取日志失败: " + err.Error()})
		return
	}
	const maxTail = int64(256 << 10)
	start := info.Size() - maxTail
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "读取日志失败: " + err.Error()})
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "读取日志失败: " + err.Error()})
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{}
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"logs": lines}})
}

func DDNSGoInstallHandler(c *gin.Context) {
	if err := os.MkdirAll(ddnsGoDir, 0755); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	version, assetURL, err := latestDDNSGoAsset()
	if err != nil {
		c.JSON(200, gin.H{"code": 2, "msg": "获取最新版本失败: " + err.Error()})
		return
	}
	tmp := ddnsGoPath("ddns-go.download.tar.gz")
	defer os.Remove(tmp)
	if err := mihomoDownloadFile(c.Request.Context(), assetURL, tmp, nil); err != nil {
		c.JSON(200, gin.H{"code": 3, "msg": "下载安装包失败: " + err.Error()})
		return
	}
	wasRunning := getDDNSGoStatus().Running
	_ = stopDDNSGo()
	tmpBin := ddnsGoPath("ddns-go.install.tmp")
	defer os.Remove(tmpBin)
	if err := extractDDNSGoBinary(tmp, tmpBin); err != nil {
		c.JSON(200, gin.H{"code": 4, "msg": "解压安装包失败: " + err.Error()})
		return
	}
	if err := os.Rename(tmpBin, ddnsGoPath(ddnsGoBinaryName)); err != nil {
		c.JSON(200, gin.H{"code": 5, "msg": "安装二进制失败: " + err.Error()})
		return
	}
	configWasEmpty := true
	if info, err := os.Stat(ddnsGoPath(ddnsGoConfigName)); err == nil && info.Size() > 0 {
		configWasEmpty = false
	}
	if _, err := os.Stat(ddnsGoPath(ddnsGoConfigName)); os.IsNotExist(err) {
		if err := os.WriteFile(ddnsGoPath(ddnsGoConfigName), nil, 0600); err != nil {
			c.JSON(200, gin.H{"code": 6, "msg": "创建配置文件失败: " + err.Error()})
			return
		}
	}
	if wasRunning || configWasEmpty {
		if err := startDDNSGo(); err != nil {
			c.JSON(200, gin.H{"code": 7, "msg": err.Error()})
			return
		}
	}
	if configWasEmpty {
		var user model.WebUser
		u, err := user.FindByID(c.GetUint("uid"))
		if err != nil {
			_ = stopDDNSGo()
			c.JSON(200, gin.H{"code": 8, "msg": "安装完成，但读取当前用户失败"})
			return
		}
		if _, err := loginDDNSGo(u.Name, u.Pwd); err != nil {
			_ = stopDDNSGo()
			c.JSON(200, gin.H{"code": 9, "msg": "安装完成，但同步主程序账号失败: " + err.Error()})
			return
		}
		if !wasRunning {
			_ = stopDDNSGo()
		}
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ddns-go " + version + " 安装完成", "data": getDDNSGoStatus()})
}

func DDNSGoControlHandler(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	var err error
	switch req.Action {
	case "start":
		err = startDDNSGo()
	case "stop":
		err = stopDDNSGo()
	case "restart":
		err = stopDDNSGo()
		if err == nil {
			err = startDDNSGo()
		}
	default:
		err = errors.New("不支持的操作")
	}
	if err != nil {
		c.JSON(200, gin.H{"code": 2, "msg": err.Error(), "data": getDDNSGoStatus()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": getDDNSGoStatus()})
}

func DDNSGoAutostartHandler(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	if err := os.MkdirAll(ddnsGoDir, 0755); err != nil {
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}
	marker := ddnsGoPath(ddnsGoAutostartMarker)
	var err error
	if req.Enabled {
		err = os.WriteFile(marker, []byte(""), 0644)
	} else {
		err = os.Remove(marker)
		if os.IsNotExist(err) {
			err = nil
		}
	}
	if err != nil {
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": getDDNSGoStatus()})
}

func InitDDNSGoAutostart() {
	if _, err := os.Stat(ddnsGoPath(ddnsGoAutostartMarker)); err != nil {
		return
	}
	go func() {
		if err := startDDNSGo(); err != nil {
			slog.Warn("ddns-go autostart failed", "err", err)
		}
	}()
}

// SyncDDNSGoPassword keeps ddns-go's own credential aligned without exposing
// the clear-text password to the browser. The binary performs its own hashing.
func SyncDDNSGoPassword(username, password string) error {
	status := getDDNSGoStatus()
	if !status.Installed {
		return nil
	}
	cmd := exec.Command(ddnsGoPath(ddnsGoBinaryName), "-resetPassword", password, "-c", ddnsGoPath(ddnsGoConfigName))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("同步 ddns-go 用户 %s 的密码失败: %s: %w", username, strings.TrimSpace(string(out)), err)
	}
	result := string(out)
	if !strings.Contains(result, "重置成功") && !strings.Contains(result, "reset successfully") {
		return fmt.Errorf("ddns-go 拒绝同步密码: %s", strings.TrimSpace(result))
	}
	if !status.Running {
		return nil
	}
	if err := stopDDNSGo(); err != nil {
		return err
	}
	return startDDNSGo()
}

func loginDDNSGo(username, password string) (*http.Cookie, error) {
	body, _ := json.Marshal(map[string]string{"Username": username, "Password": password})
	client := &http.Client{Timeout: 5 * time.Second}
	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		var req *http.Request
		req, err = http.NewRequest(http.MethodPost, ddnsGoUpstream+"/loginFunc", bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Referer", ddnsGoUpstream+"/")
			resp, err = client.Do(req)
		}
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "token" {
			return cookie, nil
		}
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return nil, fmt.Errorf("ddns-go 登录失败: %s", strings.TrimSpace(string(data)))
}

func DDNSGoConfigHandler(c *gin.Context) {
	config, err := loadDDNSGoManagedConfig()
	if err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": config})
}

func DDNSGoSaveConfigHandler(c *gin.Context) {
	var config ddnsGoManagedConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(200, gin.H{"code": 1, "msg": "配置格式错误"})
		return
	}
	config.Name = strings.TrimSpace(config.Name)
	config.DnsName = strings.TrimSpace(config.DnsName)
	if config.Name == "" || config.DnsName == "" {
		c.JSON(200, gin.H{"code": 2, "msg": "请填写配置名称并选择 DNS 服务商"})
		return
	}
	if !config.Ipv4Enable && !config.Ipv6Enable {
		c.JSON(200, gin.H{"code": 2, "msg": "IPv4 与 IPv6 至少启用一项"})
		return
	}
	status := getDDNSGoStatus()
	if !status.Installed {
		c.JSON(200, gin.H{"code": 3, "msg": "请先安装 DDNS-GO"})
		return
	}
	temporaryStart := !status.Running
	if temporaryStart {
		if err := startDDNSGo(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": err.Error()})
			return
		}
		defer stopDDNSGo()
	}
	var user model.WebUser
	u, err := user.FindByID(c.GetUint("uid"))
	if err != nil {
		c.JSON(200, gin.H{"code": 5, "msg": "读取当前用户失败"})
		return
	}
	cookie, err := loginDDNSGo(u.Name, u.Pwd)
	if err != nil {
		c.JSON(200, gin.H{"code": 6, "msg": err.Error()})
		return
	}
	payload := map[string]any{
		"Username": u.Name, "Password": u.Pwd, "Lang": "zh-cn",
		"NotAllowWanAccess": true,
		"WebhookURL":        config.WebhookURL, "WebhookRequestBody": config.WebhookRequestBody,
		"WebhookHeaders": config.WebhookHeaders, "DnsConf": []ddnsGoManagedConfig{config},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.JSON(200, gin.H{"code": 7, "msg": err.Error()})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, ddnsGoUpstream+"/save", bytes.NewReader(body))
	if err != nil {
		c.JSON(200, gin.H{"code": 7, "msg": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", ddnsGoUpstream+"/")
	req.AddCookie(cookie)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		c.JSON(200, gin.H{"code": 8, "msg": "写入 DDNS-GO 配置失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	responseData, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Result string `json:"result"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(responseData, &result) != nil || result.Result != "ok" {
		c.JSON(200, gin.H{"code": 9, "msg": "DDNS-GO 拒绝保存配置: " + strings.TrimSpace(string(responseData))})
		return
	}
	if err := saveDDNSGoManagedConfig(config); err != nil {
		c.JSON(200, gin.H{"code": 10, "msg": "配置已生效，但保存管理副本失败: " + err.Error()})
		return
	}
	message := "DDNS-GO 配置已保存"
	if !temporaryStart {
		if err := stopDDNSGo(); err != nil {
			c.JSON(200, gin.H{"code": 11, "msg": "配置已保存，但停止 DDNS-GO 失败: " + err.Error()})
			return
		}
		if err := startDDNSGo(); err != nil {
			c.JSON(200, gin.H{"code": 11, "msg": "配置已保存，但重启 DDNS-GO 失败: " + err.Error()})
			return
		}
		message = "DDNS-GO 配置已保存并重启"
	}
	c.JSON(200, gin.H{"code": 0, "msg": message, "data": config})
}
