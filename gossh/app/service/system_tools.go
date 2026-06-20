package service

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gossh/app/utils"
	"gossh/gin"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"math"
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
	"unicode/utf16"
)

const rcLocalPath = "/etc/rc.local"
const smsForwardDefaultDir = "/data/plugins/SMS_Forward"
const smsForwardConfigName = "config.json"
const smsForwardAutostartMarker = ".autostart"
const smsForwardPollInterval = 3 * time.Second
const smsForwardCompleteQuietWindow = 3 * time.Second
const smsForwardCompleteMaxWait = 9 * time.Second
const devuiDefaultDir = "/data/kano_plugins/devui"
const devuiBinaryName = "zte_topsw_devui.patched"
const devuiDownloadURL = "https://raw.githubusercontent.com/Jack-bin183/WebSSH-u60pro/zte_topsw_devui/zte_topsw_devui.patched"
const devuiAutostartMarker = ".autostart"
const devuiMountTarget = "/usr/bin/zte_topsw_devui"
const devuiHomeCardsPath = "/tmp/zte_home_cards"
const devuiSpeedPath = "/tmp/zte_speed"
const devuiWallpaperWidth = 320
const devuiWallpaperHeight = 456
const devuiWallpaperColorFmt = 5
const devuiWallpaperDescOff = 0xA51500
const devuiWallpaperDataVA = 0xC1484A
const devuiWallpaperDataOff = 0x81484A
const devuiWallpaperDataCap = 0x6AE00

type smsMessage struct {
	ID       int    `json:"id"`
	Number   string `json:"number"`
	Date     string `json:"date"`
	Content  string `json:"content"`
	RawHex   string `json:"raw_hex"`
	Tag      string `json:"tag"`
	MemStore string `json:"mem_store"`
}

type smsForwardConfig struct {
	BarkEnabled bool   `json:"bark_enabled"`
	BarkURL     string `json:"bark_url"`
	BarkGroup   string `json:"bark_group"`
	TgEnabled   bool   `json:"tg_enabled"`
	TgBotToken  string `json:"tg_bot_token"`
	TgChatID    string `json:"tg_chat_id"`
	LastID      int    `json:"last_id"`
}

type smsForwardRuntimeStatus struct {
	Running   bool   `json:"running"`
	StartedAt string `json:"started_at"`
	LastError string `json:"last_error"`
	SentCount int    `json:"sent_count"`
	LastID    int    `json:"last_id"`
}

type smsForwardPendingBatch struct {
	Number    string
	Message   smsMessage
	signature string
	FirstSeen time.Time
	LastSeen  time.Time
}

type smsForwardPendingState struct {
	byNumber  map[string]*smsForwardPendingBatch
	completed map[int]struct{}
}

type devuiRuntimeStatus struct {
	Running          bool   `json:"running"`
	AutostartEnabled bool   `json:"autostart_enabled"`
	BinaryExists     bool   `json:"binary_exists"`
	Mounted          bool   `json:"mounted"`
	DataReady        bool   `json:"data_ready"`
	DataError        string `json:"data_error"`
	LastError        string `json:"last_error"`
}

type devuiDownloadStatus struct {
	State      string `json:"state"`
	Msg        string `json:"msg"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	UpdatedAt  string `json:"updated_at"`
}

var smsForwardMu sync.Mutex
var smsForwardStop chan struct{}
var smsForwardStatus smsForwardRuntimeStatus
var devuiMu sync.Mutex
var devuiStop chan struct{}
var devuiStatus devuiRuntimeStatus
var devuiDownloadMu sync.Mutex
var devuiDownloadStatusMu sync.RWMutex
var devuiDownloadStatusVar = devuiDownloadStatus{State: "idle", Msg: "暂无下载任务"}

func SystemSmsListHandler(c *gin.Context) {
	messages, err := loadSmsMessages()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"messages": messages}})
}

func SystemSmsForwardStatusHandler(c *gin.Context) {
	cfg, err := loadSmsForwardConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	status := getSmsForwardStatus()
	status.LastID = cfg.LastID
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"config":            cfg,
		"running":           status.Running,
		"started_at":        status.StartedAt,
		"last_error":        status.LastError,
		"sent_count":        status.SentCount,
		"last_id":           cfg.LastID,
		"autostart_enabled": getSmsForwardAutostartEnabled(),
		"poll_interval":     int(smsForwardPollInterval.Seconds()),
	}})
}

func SystemSmsForwardConfigHandler(c *gin.Context) {
	var req smsForwardConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	current, _ := loadSmsForwardConfig()
	if req.LastID == 0 {
		req.LastID = current.LastID
	}
	if err := saveSmsForwardConfig(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": req})
}

func SystemSmsForwardControlHandler(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	switch req.Action {
	case "start":
		if err := startSmsForwardWorker(false); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
			return
		}
	case "stop":
		stopSmsForwardWorker()
	case "restart":
		stopSmsForwardWorker()
		if err := startSmsForwardWorker(false); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "不支持的操作"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": getSmsForwardStatus()})
}

func SystemSmsForwardAutostartHandler(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	if err := setSmsForwardAutostartEnabled(req.Enabled); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"enabled": req.Enabled}})
}

func SystemSmsForwardHandler(c *gin.Context) {
	type body struct {
		BarkEnabled bool   `json:"bark_enabled"`
		BarkURL     string `json:"bark_url"`
		BarkGroup   string `json:"bark_group"`
		TgEnabled   bool   `json:"tg_enabled"`
		TgBotToken  string `json:"tg_bot_token"`
		TgChatID    string `json:"tg_chat_id"`
		LastID      int    `json:"last_id"`
		OnlyLatest  bool   `json:"only_latest"`
		DryRun      bool   `json:"dry_run"`
	}
	var req body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	messages, err := loadSmsMessages()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	targets := make([]smsMessage, 0)
	for _, msg := range messages {
		if msg.ID > req.LastID {
			targets = append(targets, msg)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	if req.OnlyLatest && len(targets) > 1 {
		targets = targets[len(targets)-1:]
	}

	latestID := req.LastID
	for _, msg := range messages {
		if msg.ID > latestID {
			latestID = msg.ID
		}
	}
	if req.DryRun {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"latest_id": latestID, "sent": 0, "messages": targets}})
		return
	}

	sent := 0
	var errs []string
	for _, msg := range targets {
		title := fmt.Sprintf("短信 %s", msg.Number)
		text := fmt.Sprintf("%s\n时间: %s", msg.Content, msg.Date)
		msgSent := 0
		if req.BarkEnabled {
			if err := sendBark(req.BarkURL, req.BarkGroup, title, text); err != nil {
				errs = append(errs, "Bark: "+err.Error())
			} else {
				sent++
				msgSent++
			}
		}
		if req.TgEnabled {
			if err := sendTelegram(req.TgBotToken, req.TgChatID, text); err != nil {
				errs = append(errs, "TG: "+err.Error())
			} else {
				sent++
				msgSent++
			}
		}
		if msgSent > 0 {
			if err := markSmsRead(msg.ID); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(errs) > 0 {
		c.JSON(http.StatusOK, gin.H{"code": 3, "msg": strings.Join(errs, "; "), "data": gin.H{"latest_id": latestID, "sent": sent, "messages": targets}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"latest_id": latestID, "sent": sent, "messages": targets}})
}

func InitSmsForwardAutostart() {
	if !getSmsForwardAutostartEnabled() {
		return
	}
	go func() {
		if err := startSmsForwardWorker(true); err != nil {
			slog.Warn("sms forward autostart failed", "err", err)
		}
	}()
}

func startSmsForwardWorker(fromAutostart bool) error {
	smsForwardMu.Lock()
	if smsForwardStop != nil {
		smsForwardMu.Unlock()
		return nil
	}
	cfg, err := loadSmsForwardConfig()
	if err != nil {
		smsForwardMu.Unlock()
		return err
	}
	if err := validateSmsForwardConfig(cfg); err != nil {
		smsForwardMu.Unlock()
		return err
	}
	stop := make(chan struct{})
	smsForwardStop = stop
	smsForwardStatus = smsForwardRuntimeStatus{
		Running:   true,
		StartedAt: time.Now().Format(time.RFC3339),
		LastID:    cfg.LastID,
	}
	smsForwardMu.Unlock()

	go runSmsForwardWorker(stop, fromAutostart)
	return nil
}

func stopSmsForwardWorker() {
	smsForwardMu.Lock()
	if smsForwardStop != nil {
		close(smsForwardStop)
		smsForwardStop = nil
	}
	smsForwardStatus.Running = false
	smsForwardMu.Unlock()
}

func runSmsForwardWorker(stop <-chan struct{}, fromAutostart bool) {
	defer func() {
		smsForwardMu.Lock()
		if smsForwardStop == stop {
			smsForwardStop = nil
		}
		smsForwardStatus.Running = false
		smsForwardMu.Unlock()
	}()

	if fromAutostart {
		time.Sleep(5 * time.Second)
	}
	ticker := time.NewTicker(smsForwardPollInterval)
	defer ticker.Stop()

	pending := newSmsForwardPendingState()
	if err := smsForwardPollOnce(pending); err != nil {
		setSmsForwardLastError(err.Error())
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := smsForwardPollOnce(pending); err != nil {
				setSmsForwardLastError(err.Error())
			}
		}
	}
}

func smsForwardPollOnce(pending *smsForwardPendingState) error {
	cfg, err := loadSmsForwardConfig()
	if err != nil {
		return err
	}
	if err := validateSmsForwardConfig(cfg); err != nil {
		return err
	}
	messages, err := loadSmsMessages()
	if err != nil {
		return err
	}
	latestID := cfg.LastID
	for _, msg := range messages {
		if msg.ID > latestID {
			latestID = msg.ID
		}
	}
	if cfg.LastID == 0 {
		cfg.LastID = latestID
		if err := saveSmsForwardConfig(cfg); err != nil {
			return err
		}
		setSmsForwardLastID(cfg.LastID)
		return nil
	}

	targets := make([]smsMessage, 0)
	for _, msg := range messages {
		if msg.ID > cfg.LastID {
			targets = append(targets, msg)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })

	now := time.Now()
	for _, msg := range targets {
		pending.add(msg, now)
	}

	ready := pending.ready(now)
	sent := 0
	var errs []string
	for _, batch := range ready {
		batchSent, batchErrs := sendSmsForwardBatch(cfg, batch)
		sent += batchSent
		errs = append(errs, batchErrs...)
		pending.completed[batch.Message.ID] = struct{}{}
	}
	newLastID := pending.nextLastID(cfg.LastID)
	if newLastID > cfg.LastID {
		cfg.LastID = newLastID
		if err := saveSmsForwardConfig(cfg); err != nil {
			return err
		}
		pending.discardCompletedThrough(cfg.LastID)
	}
	setSmsForwardSent(sent, cfg.LastID)
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	setSmsForwardLastError("")
	return nil
}

func newSmsForwardPendingState() *smsForwardPendingState {
	return &smsForwardPendingState{
		byNumber:  make(map[string]*smsForwardPendingBatch),
		completed: make(map[int]struct{}),
	}
}

func (s *smsForwardPendingState) add(msg smsMessage, now time.Time) {
	if _, done := s.completed[msg.ID]; done {
		return
	}
	key := smsForwardPendingKey(msg)
	signature := smsForwardMessageSignature(msg)
	batch := s.byNumber[key]
	if batch == nil {
		batch = &smsForwardPendingBatch{
			Number:    msg.Number,
			Message:   msg,
			signature: signature,
			FirstSeen: now,
			LastSeen:  now,
		}
		s.byNumber[key] = batch
		return
	}
	if msg.ID < batch.Message.ID {
		return
	}
	if msg.ID > batch.Message.ID || signature != batch.signature {
		batch.Number = msg.Number
		batch.Message = msg
		batch.signature = signature
		batch.LastSeen = now
	}
}

func (s *smsForwardPendingState) ready(now time.Time) []*smsForwardPendingBatch {
	ready := make([]*smsForwardPendingBatch, 0)
	for key, batch := range s.byNumber {
		if batch.Message.ID == 0 {
			delete(s.byNumber, key)
			continue
		}
		if now.Sub(batch.LastSeen) >= smsForwardCompleteQuietWindow || now.Sub(batch.FirstSeen) >= smsForwardCompleteMaxWait {
			ready = append(ready, batch)
			delete(s.byNumber, key)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Message.ID < ready[j].Message.ID
	})
	return ready
}

func (s *smsForwardPendingState) nextLastID(current int) int {
	minPendingID := 0
	for _, batch := range s.byNumber {
		if minPendingID == 0 || batch.Message.ID < minPendingID {
			minPendingID = batch.Message.ID
		}
	}
	next := current
	for id := range s.completed {
		if id > next && (minPendingID == 0 || id < minPendingID) {
			next = id
		}
	}
	return next
}

func (s *smsForwardPendingState) discardCompletedThrough(lastID int) {
	for id := range s.completed {
		if id <= lastID {
			delete(s.completed, id)
		}
	}
}

func smsForwardPendingKey(msg smsMessage) string {
	number := strings.TrimSpace(msg.Number)
	if number == "" {
		return fmt.Sprintf("message:%d", msg.ID)
	}
	return "number:" + number
}

func sendSmsForwardBatch(cfg smsForwardConfig, batch *smsForwardPendingBatch) (int, []string) {
	title := fmt.Sprintf("短信 %s", batch.Number)
	text := batch.Message.Content
	sent := 0
	var errs []string
	if cfg.BarkEnabled {
		if err := sendBark(cfg.BarkURL, cfg.BarkGroup, title, text); err != nil {
			errs = append(errs, "Bark: "+err.Error())
		} else {
			sent++
		}
	}
	if cfg.TgEnabled {
		if err := sendTelegram(cfg.TgBotToken, cfg.TgChatID, text); err != nil {
			errs = append(errs, "TG: "+err.Error())
		} else {
			sent++
		}
	}
	if sent > 0 {
		if err := markSmsRead(batch.Message.ID); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return sent, errs
}

// markSmsRead 调用 ubus 将指定短信标记为已读（tag=0），id 取自短信本身
func markSmsRead(id int) error {
	if id <= 0 {
		return nil
	}
	if _, err := utils.GetDataFromUbus("zwrt_wms", "zwrt_wms_modify_tag", map[string]interface{}{
		"id":  fmt.Sprintf("%d;", id),
		"tag": 0,
	}); err != nil {
		return fmt.Errorf("标记已读失败(id=%d): %w", id, err)
	}
	return nil
}

func smsForwardMessageSignature(msg smsMessage) string {
	return fmt.Sprintf("%d\x00%s\x00%s\x00%s", msg.ID, msg.Date, msg.Content, msg.RawHex)
}

func smsForwardDir() string {
	return smsForwardDefaultDir
}

func smsForwardConfigPath() string {
	return filepath.Join(smsForwardDir(), smsForwardConfigName)
}

func loadSmsForwardConfig() (smsForwardConfig, error) {
	var cfg smsForwardConfig
	data, err := os.ReadFile(smsForwardConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("读取短信转发配置失败: %w", err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析短信转发配置失败: %w", err)
	}
	return cfg, nil
}

func saveSmsForwardConfig(cfg smsForwardConfig) error {
	if err := os.MkdirAll(smsForwardDir(), 0755); err != nil {
		return fmt.Errorf("创建短信转发目录失败: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化短信转发配置失败: %w", err)
	}
	if err := os.WriteFile(smsForwardConfigPath(), data, 0600); err != nil {
		return fmt.Errorf("保存短信转发配置失败: %w", err)
	}
	return nil
}

func validateSmsForwardConfig(cfg smsForwardConfig) error {
	if !cfg.BarkEnabled && !cfg.TgEnabled {
		return errors.New("请至少启用 Bark 或 TG Bot")
	}
	if cfg.BarkEnabled && strings.TrimSpace(cfg.BarkURL) == "" {
		return errors.New("请填写 Bark 地址")
	}
	if cfg.TgEnabled && (strings.TrimSpace(cfg.TgBotToken) == "" || strings.TrimSpace(cfg.TgChatID) == "") {
		return errors.New("请填写 TG Bot Token 和 Chat ID")
	}
	return nil
}

func getSmsForwardAutostartEnabled() bool {
	_, err := os.Stat(filepath.Join(smsForwardDir(), smsForwardAutostartMarker))
	return err == nil
}

func setSmsForwardAutostartEnabled(enabled bool) error {
	if err := os.MkdirAll(smsForwardDir(), 0755); err != nil {
		return fmt.Errorf("创建短信转发目录失败: %w", err)
	}
	marker := filepath.Join(smsForwardDir(), smsForwardAutostartMarker)
	if enabled {
		return os.WriteFile(marker, []byte(""), 0644)
	}
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func getSmsForwardStatus() smsForwardRuntimeStatus {
	smsForwardMu.Lock()
	defer smsForwardMu.Unlock()
	status := smsForwardStatus
	status.Running = smsForwardStop != nil
	return status
}

func setSmsForwardLastError(msg string) {
	smsForwardMu.Lock()
	defer smsForwardMu.Unlock()
	smsForwardStatus.LastError = msg
}

func setSmsForwardLastID(lastID int) {
	smsForwardMu.Lock()
	defer smsForwardMu.Unlock()
	smsForwardStatus.LastID = lastID
}

func setSmsForwardSent(sent int, lastID int) {
	smsForwardMu.Lock()
	defer smsForwardMu.Unlock()
	smsForwardStatus.SentCount += sent
	smsForwardStatus.LastID = lastID
}

func SystemDevuiStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": getDevuiStatus()})
}

func SystemDevuiDownloadHandler(c *gin.Context) {
	if getDevuiDownloadStatus().State == "downloading" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已有下载任务正在进行", "data": getDevuiDownloadStatus()})
		return
	}
	setDevuiDownloadStatus(func(s *devuiDownloadStatus) {
		*s = devuiDownloadStatus{State: "downloading", Msg: "正在准备下载...", UpdatedAt: time.Now().Format(time.RFC3339)}
	})
	go runDevuiDownloadTask()
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已开始下载", "data": getDevuiDownloadStatus()})
}

func SystemDevuiDownloadStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": getDevuiDownloadStatus()})
}

func SystemDevuiControlHandler(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	var err error
	if req.Enabled {
		err = startDevuiScreenUpdate()
	} else {
		err = stopDevuiScreenUpdate(true)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error(), "data": getDevuiStatus()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": getDevuiStatus()})
}

func SystemDevuiAutostartHandler(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "参数错误: " + err.Error()})
		return
	}
	if req.Enabled {
		if err := ensureDevuiBinary(); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error(), "data": getDevuiStatus()})
			return
		}
	}
	if err := setDevuiAutostartEnabled(req.Enabled); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 3, "msg": err.Error(), "data": getDevuiStatus()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": getDevuiStatus()})
}

func SystemDevuiWallpaperHandler(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "请选择 jpg/png 图片"})
		return
	}
	if fileHeader.Size > 12*1024*1024 {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": "图片不能超过 12MB"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 3, "msg": "读取图片失败: " + err.Error()})
		return
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 4, "msg": "图片格式不支持，请上传 jpg/png"})
		return
	}
	if err := ensureDevuiBinary(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 5, "msg": err.Error(), "data": getDevuiStatus()})
		return
	}
	status := getDevuiStatus()
	if status.Running || status.Mounted {
		if err := stopDevuiScreenUpdate(true); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 6, "msg": "替换壁纸前关闭屏幕更新失败: " + err.Error(), "data": getDevuiStatus()})
			return
		}
	}
	if err := patchDevuiWallpaper(img); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 7, "msg": err.Error(), "data": getDevuiStatus()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "壁纸已替换", "data": getDevuiStatus()})
}

func InitDevuiAutostart() {
	if !getDevuiAutostartEnabled() {
		return
	}
	go func() {
		time.Sleep(5 * time.Second)
		if err := startDevuiScreenUpdate(); err != nil {
			setDevuiLastError(err.Error())
			slog.Warn("devui autostart failed", "err", err)
		}
	}()
}

func startDevuiScreenUpdate() error {
	if err := ensureDevuiBinary(); err != nil {
		setDevuiLastError(err.Error())
		return err
	}

	devuiMu.Lock()
	if devuiStop == nil {
		stop := make(chan struct{})
		devuiStop = stop
		devuiStatus.Running = true
		devuiStatus.LastError = ""
		go runDevuiHomeCardsWorker(stop)
	}
	devuiMu.Unlock()

	if err := waitDevuiFilesReady(5 * time.Second); err != nil {
		_ = stopDevuiScreenUpdate(false)
		setDevuiLastError(err.Error())
		return err
	}
	if err := bindMountDevuiBinary(); err != nil {
		_ = stopDevuiScreenUpdate(false)
		setDevuiLastError(err.Error())
		return err
	}
	if err := restartDevuiService(); err != nil {
		_ = stopDevuiScreenUpdate(false)
		setDevuiLastError(err.Error())
		return err
	}
	setDevuiLastError("")
	return nil
}

func stopDevuiScreenUpdate(startOriginal bool) error {
	devuiMu.Lock()
	if devuiStop != nil {
		close(devuiStop)
		devuiStop = nil
	}
	devuiStatus.Running = false
	devuiMu.Unlock()

	if isDevuiMounted() {
		if err := stopDevuiService(); err != nil {
			setDevuiLastError(err.Error())
			return err
		}
		if out, err := exec.Command("umount", "-l", devuiMountTarget).CombinedOutput(); err != nil {
			msg := fmt.Sprintf("取消挂载失败: %v %s", err, strings.TrimSpace(string(out)))
			setDevuiLastError(msg)
			return errors.New(msg)
		}
	}
	if startOriginal {
		if err := startDevuiService(); err != nil {
			setDevuiLastError(err.Error())
			return err
		}
	}
	setDevuiLastError("")
	return nil
}

func runDevuiHomeCardsWorker(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	devuiWriteScreenFiles()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			devuiWriteScreenFiles()
		}
	}
}

func devuiWriteScreenFiles() {
	updateDevuiSpeed()
	buildDevuiHomeCards()
}

func getDevuiStatus() devuiRuntimeStatus {
	devuiMu.Lock()
	status := devuiStatus
	devuiMu.Unlock()
	status.AutostartEnabled = getDevuiAutostartEnabled()
	status.BinaryExists = fileExists(devuiBinaryPath())
	status.Mounted = isDevuiMounted()
	status.Running = status.Mounted
	status.DataReady, status.DataError = devuiDataInterfaceStatus()
	return status
}

func devuiDataInterfaceStatus() (bool, string) {
	missing := make([]string, 0, 2)
	if !fileExists(devuiSpeedPath) {
		missing = append(missing, devuiSpeedPath+" 异常")
	}
	if !fileExists(devuiHomeCardsPath) {
		missing = append(missing, devuiHomeCardsPath+" 异常")
	}
	if len(missing) > 0 {
		return false, strings.Join(missing, "；")
	}
	return true, ""
}

func setDevuiLastError(msg string) {
	devuiMu.Lock()
	defer devuiMu.Unlock()
	devuiStatus.LastError = msg
}

func getDevuiDownloadStatus() devuiDownloadStatus {
	devuiDownloadStatusMu.RLock()
	status := devuiDownloadStatusVar
	devuiDownloadStatusMu.RUnlock()
	if status.State == "done" && !fileExists(devuiBinaryPath()) {
		status = devuiDownloadStatus{State: "idle", Msg: "补丁文件未下载", UpdatedAt: time.Now().Format(time.RFC3339)}
		devuiDownloadStatusMu.Lock()
		devuiDownloadStatusVar = status
		devuiDownloadStatusMu.Unlock()
	}
	return status
}

func setDevuiDownloadStatus(fn func(*devuiDownloadStatus)) {
	devuiDownloadStatusMu.Lock()
	defer devuiDownloadStatusMu.Unlock()
	fn(&devuiDownloadStatusVar)
	devuiDownloadStatusVar.UpdatedAt = time.Now().Format(time.RFC3339)
	if devuiDownloadStatusVar.Total > 0 {
		percent := int(devuiDownloadStatusVar.Downloaded * 100 / devuiDownloadStatusVar.Total)
		if percent > 100 {
			percent = 100
		}
		devuiDownloadStatusVar.Percent = percent
	}
}

func runDevuiDownloadTask() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := downloadDevuiBinaryFile(ctx, true, func(downloaded, total int64) {
		setDevuiDownloadStatus(func(s *devuiDownloadStatus) {
			s.Msg = "正在下载 devui 补丁文件..."
			s.Downloaded = downloaded
			s.Total = total
		})
	}); err != nil {
		setDevuiDownloadStatus(func(s *devuiDownloadStatus) {
			s.State = "failed"
			s.Msg = "下载失败: " + err.Error()
		})
		return
	}
	setDevuiDownloadStatus(func(s *devuiDownloadStatus) {
		s.State = "done"
		s.Msg = "devui 补丁文件已下载"
		s.Percent = 100
	})
}

func devuiDir() string {
	return devuiDefaultDir
}

func devuiBinaryPath() string {
	return filepath.Join(devuiDir(), devuiBinaryName)
}

func devuiAutostartPath() string {
	return filepath.Join(devuiDir(), devuiAutostartMarker)
}

func getDevuiAutostartEnabled() bool {
	return fileExists(devuiAutostartPath())
}

func setDevuiAutostartEnabled(enabled bool) error {
	if err := os.MkdirAll(devuiDir(), 0755); err != nil {
		return fmt.Errorf("创建屏幕更新目录失败: %w", err)
	}
	if enabled {
		return os.WriteFile(devuiAutostartPath(), []byte(""), 0644)
	}
	if err := os.Remove(devuiAutostartPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除屏幕更新自启标志失败: %w", err)
	}
	return nil
}

func ensureDevuiBinary() error {
	path := devuiBinaryPath()
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		if err := os.Chmod(path, 0755); err != nil {
			return fmt.Errorf("设置 devui 文件权限失败: %w", err)
		}
		return nil
	}
	return downloadDevuiBinaryFile(context.Background(), false, nil)
}

func downloadDevuiBinaryFile(ctx context.Context, force bool, onProgress func(downloaded, total int64)) error {
	devuiDownloadMu.Lock()
	defer devuiDownloadMu.Unlock()
	path := devuiBinaryPath()
	if !force {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return os.Chmod(path, 0755)
		}
	}
	if err := os.MkdirAll(devuiDir(), 0755); err != nil {
		return fmt.Errorf("创建屏幕更新目录失败: %w", err)
	}
	tmp := path + ".download"
	if err := mihomoDownloadFile(ctx, devuiDownloadURL, tmp, onProgress); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("下载 devui 文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("安装 devui 文件失败: %w", err)
	}
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("设置 devui 文件权限失败: %w", err)
	}
	return nil
}

func waitDevuiFilesReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if fileExists(devuiSpeedPath) && fileExists(devuiHomeCardsPath) {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("等待 /tmp/zte_speed 和 /tmp/zte_home_cards 生成超时")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func bindMountDevuiBinary() error {
	if isDevuiMounted() {
		return nil
	}
	out, err := exec.Command("mount", "--bind", devuiBinaryPath(), devuiMountTarget).CombinedOutput()
	if err != nil {
		return fmt.Errorf("挂载 devui 补丁失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isDevuiMounted() bool {
	out, err := exec.Command("mount").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), devuiMountTarget)
}

func restartDevuiService() error {
	out, err := exec.Command("/etc/init.d/zte_topsw_devui", "restart").CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启 zte_topsw_devui 失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func stopDevuiService() error {
	out, err := exec.Command("/etc/init.d/zte_topsw_devui", "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止 zte_topsw_devui 失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startDevuiService() error {
	out, err := exec.Command("/etc/init.d/zte_topsw_devui", "start").CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动 zte_topsw_devui 失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func updateDevuiSpeed() {
	out, err := exec.Command("ubus", "call", "zwrt_data", "get_wwandst", `{"source_module":"web","cid":1,"type":4}`).Output()
	if err != nil {
		return
	}
	var tx, rx string
	for _, line := range strings.Split(string(out), "\n") {
		if devuiRealTxRe.MatchString(line) {
			tx = devuiNonDigitRe.ReplaceAllString(line, "")
		}
		if devuiRealRxRe.MatchString(line) {
			rx = devuiNonDigitRe.ReplaceAllString(line, "")
		}
	}
	if tx != "" && rx != "" {
		_ = writeAtomicFile(devuiSpeedPath, []byte(fmt.Sprintf("%s %s\n", tx, rx)))
	}
}

func buildDevuiHomeCards() {
	out, err := exec.Command("ubus", "call", "zte_nwinfo_api", "nwinfo_get_netinfo", "{}").Output()
	if err != nil {
		return
	}
	jsonText := string(out)
	if jsonText == "" {
		return
	}

	provider := devuiJSONGet(jsonText, "network_provider")
	mode := devuiJSONGet(jsonText, "network_type")
	wanBand := devuiJSONGet(jsonText, "wan_active_band")
	nrca := devuiJSONGet(jsonText, "nrca")
	lteca := devuiJSONGet(jsonText, "lteca")

	modeU := strings.ToUpper(mode)
	wanBandU := strings.ToUpper(wanBand)
	isLTE := strings.Contains(modeU, "LTE") || strings.HasPrefix(wanBandU, "LTE")

	var band, pci, freq, bw string
	var rsrp, rsrq, sinr, rssi string
	if isLTE {
		band = compactBand(wanBand)
		pci = devuiJSONGet(jsonText, "lte_pci")
		freq = devuiJSONGet(jsonText, "wan_active_channel")
		bw = lteBWFromCA(lteca)
		rsrp = devuiJSONGet(jsonText, "lte_rsrp")
		rsrq = devuiJSONGet(jsonText, "lte_rsrq")
		sinr = devuiJSONGet(jsonText, "lte_snr")
		rssi = devuiJSONGet(jsonText, "lte_rssi")
	} else {
		band = devuiJSONGet(jsonText, "nr5g_action_band")
		if band == "" {
			band = wanBand
		}
		band = compactBand(band)
		pci = devuiJSONGet(jsonText, "nr5g_pci")
		freq = devuiJSONGet(jsonText, "nr5g_action_channel")
		bw = devuiJSONGet(jsonText, "nr5g_bandwidth")
		rsrp = devuiJSONGet(jsonText, "nr5g_rsrp")
		rsrq = devuiJSONGet(jsonText, "nr5g_rsrq")
		sinr = devuiJSONGet(jsonText, "nr5g_snr")
		rssi = devuiJSONGet(jsonText, "nr5g_rssi")
	}

	provider = fallbackDash(provider)
	mode = fallbackDash(mode)
	band = fallbackDash(band)
	bw = fallbackDash(bw)
	pci = fallbackDash(pci)
	freq = fallbackDash(freq)
	rsrp = fmtNum(fallbackDash(rsrp))
	rsrq = fmtNum(fallbackDash(rsrq))
	sinr = fmtNum(fallbackDash(sinr))
	rssi = fmtNum(fallbackDash(rssi))

	bwText := "--"
	if bw != "--" {
		bwText = bw + "M"
	}

	var b strings.Builder
	writeLine := func(s string) {
		b.WriteString(s)
		b.WriteByte('\n')
	}
	writeLine("RSRP")
	writeLine(gradeSignal(rsrp, "rsrp"))
	writeLine(rsrp + " dBm")
	writeLine("RSRQ")
	writeLine(gradeSignal(rsrq, "rsrq"))
	writeLine(rsrq + " dB")
	writeLine("SINR")
	writeLine(gradeSignal(sinr, "sinr"))
	writeLine(sinr + " dB")
	writeLine("RSSI")
	writeLine(gradeSignal(rssi, "rssi"))
	writeLine(rssi + " dBm")
	writeLine("PCI")
	writeLine("频段")
	if isLTE {
		writeLine("信道")
	} else {
		writeLine("频点")
	}
	writeLine("带宽")
	writeLine("RSRP")
	writeLine("SINR")
	writeLine(pci)
	writeLine(band)
	writeLine(freq)
	writeLine(bwText)
	writeLine(rsrp)
	writeLine(sinr)

	if isLTE {
		count := 0
		skipped := false
		for _, line := range strings.Split(lteca, ";") {
			if count >= 2 {
				break
			}
			fields := strings.Split(line, ",")
			if len(fields) < 5 {
				continue
			}
			if !skipped && fields[0] == pci && fields[3] == freq {
				skipped = true
				continue
			}
			count++
			writeLine(fields[0])
			writeLine("B" + fields[1])
			writeLine(fields[3])
			writeLine(fields[4] + "M")
			writeLine("-")
			writeLine("-")
		}
	} else {
		count := 0
		for _, line := range strings.Split(nrca, ";") {
			if count >= 2 {
				break
			}
			fields := strings.Split(line, ",")
			if len(fields) < 11 {
				continue
			}
			count++
			writeLine(fields[1])
			writeLine("N" + fields[3])
			writeLine(fields[4])
			writeLine(fields[5] + "M")
			writeLine(devuiStripDotZero(fields[7]))
			writeLine(devuiStripDotZero(fields[9]))
		}
	}

	lines := strings.Count(b.String(), "\n")
	for lines < 36 {
		writeLine("-")
		lines++
	}

	writeLine(strconv.Itoa(signalBarWidth(rsrp, "rsrp")))
	writeLine(strconv.Itoa(signalBarWidth(rsrq, "rsrq")))
	writeLine(strconv.Itoa(signalBarWidth(sinr, "sinr")))
	writeLine(strconv.Itoa(signalBarWidth(rssi, "rssi")))

	_ = provider
	_ = mode
	_ = writeAtomicFile(devuiHomeCardsPath, []byte(b.String()))
}

func patchDevuiWallpaper(img image.Image) error {
	binPath := devuiBinaryPath()
	src, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("读取 devui 文件失败: %w", err)
	}
	if err := verifyDevuiWallpaperDescriptor(src); err != nil {
		return err
	}
	if len(src) < devuiWallpaperDataOff+devuiWallpaperDataCap {
		return fmt.Errorf("devui 文件过小: 壁纸数据区超过文件大小")
	}
	if _, err := os.Stat(binPath + ".bak"); os.IsNotExist(err) {
		if err := copyLocalFile(binPath, binPath+".bak"); err != nil {
			return fmt.Errorf("备份 devui 文件失败: %w", err)
		}
	}
	payload := convertDevuiWallpaper(img)
	patched := append([]byte(nil), src...)
	updateDevuiWallpaperDescriptor(patched)
	copy(patched[devuiWallpaperDataOff:devuiWallpaperDataOff+devuiWallpaperDataCap], payload)
	if err := os.WriteFile(binPath, patched, 0755); err != nil {
		return fmt.Errorf("写入 devui 壁纸失败: %w", err)
	}
	return nil
}

func verifyDevuiWallpaperDescriptor(bin []byte) error {
	if len(bin) < devuiWallpaperDescOff+16 {
		return fmt.Errorf("devui 文件过小: 找不到壁纸描述符")
	}
	d := bin[devuiWallpaperDescOff : devuiWallpaperDescOff+16]
	header := binary.LittleEndian.Uint32(d[0:4])
	size := binary.LittleEndian.Uint32(d[4:8])
	ptr := binary.LittleEndian.Uint64(d[8:16])
	cf := int(header & 0x1F)
	w := int((header >> 10) & 0x7FF)
	h := int((header >> 21) & 0x7FF)
	if cf != devuiWallpaperColorFmt || w != devuiWallpaperWidth || h != devuiWallpaperHeight || size != devuiWallpaperDataCap || ptr != devuiWallpaperDataVA {
		return fmt.Errorf("devui 壁纸描述符不匹配: cf=%d size=%dx%d data_size=0x%x data_ptr=0x%x", cf, w, h, size, ptr)
	}
	return nil
}

func updateDevuiWallpaperDescriptor(bin []byte) {
	d := bin[devuiWallpaperDescOff : devuiWallpaperDescOff+16]
	oldHeader := binary.LittleEndian.Uint32(d[0:4])
	mask := uint32(0x1F) | (uint32(0x7FF) << 10) | (uint32(0x7FF) << 21)
	header := (oldHeader &^ mask) |
		uint32(devuiWallpaperColorFmt) |
		(uint32(devuiWallpaperWidth) << 10) |
		(uint32(devuiWallpaperHeight) << 21)
	binary.LittleEndian.PutUint32(d[0:4], header)
	binary.LittleEndian.PutUint32(d[4:8], uint32(devuiWallpaperDataCap))
	binary.LittleEndian.PutUint64(d[8:16], devuiWallpaperDataVA)
}

func convertDevuiWallpaper(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, devuiWallpaperDataCap)
	if b.Dx() == devuiWallpaperWidth && b.Dy() == devuiWallpaperHeight {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := img.At(x, y).RGBA()
				out = appendDevuiLVGLPixel(out, r, g, bl, a)
			}
		}
		return out
	}
	for y := 0; y < devuiWallpaperHeight; y++ {
		for x := 0; x < devuiWallpaperWidth; x++ {
			r, g, bl, a := sampleDevuiBilinear(img, x, y)
			out = appendDevuiLVGLPixel(out, r, g, bl, a)
		}
	}
	return out
}

func sampleDevuiBilinear(img image.Image, dstX, dstY int) (uint32, uint32, uint32, uint32) {
	b := img.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	if srcW == 1 && srcH == 1 {
		return img.At(b.Min.X, b.Min.Y).RGBA()
	}
	sx := float64(dstX) * float64(srcW-1) / float64(devuiWallpaperWidth-1)
	sy := float64(dstY) * float64(srcH-1) / float64(devuiWallpaperHeight-1)
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	x1 := min(x0+1, srcW-1)
	y1 := min(y0+1, srcH-1)
	tx := sx - float64(x0)
	ty := sy - float64(y0)

	r00, g00, b00, a00 := img.At(b.Min.X+x0, b.Min.Y+y0).RGBA()
	r10, g10, b10, a10 := img.At(b.Min.X+x1, b.Min.Y+y0).RGBA()
	r01, g01, b01, a01 := img.At(b.Min.X+x0, b.Min.Y+y1).RGBA()
	r11, g11, b11, a11 := img.At(b.Min.X+x1, b.Min.Y+y1).RGBA()
	return bilinearUint32(r00, r10, r01, r11, tx, ty),
		bilinearUint32(g00, g10, g01, g11, tx, ty),
		bilinearUint32(b00, b10, b01, b11, tx, ty),
		bilinearUint32(a00, a10, a01, a11, tx, ty)
}

func appendDevuiLVGLPixel(out []byte, r16, g16, b16, a16 uint32) []byte {
	r := uint8(r16 >> 8)
	g := uint8(g16 >> 8)
	bl := uint8(b16 >> 8)
	a := uint8(a16 >> 8)
	rgb565 := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(bl>>3)
	return append(out, byte(rgb565), byte(rgb565>>8), a)
}

func bilinearUint32(v00, v10, v01, v11 uint32, tx, ty float64) uint32 {
	top := float64(v00)*(1-tx) + float64(v10)*tx
	bottom := float64(v01)*(1-tx) + float64(v11)*tx
	return uint32(top*(1-ty) + bottom*ty + 0.5)
}

func writeAtomicFile(path string, data []byte) error {
	tmp := fmt.Sprintf("%s.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func copyLocalFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fallbackDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "--"
	}
	return value
}

var devuiFmtNumRe = regexp.MustCompile(`^-?[0-9]+\.0+$`)
var devuiRealTxRe = regexp.MustCompile(`"real_tx_speed"`)
var devuiRealRxRe = regexp.MustCompile(`"real_rx_speed"`)
var devuiNonDigitRe = regexp.MustCompile(`[^0-9]`)
var devuiDotZeroRe = regexp.MustCompile(`\.0+$`)

func devuiJSONGet(jsonText string, key string) string {
	pattern := `"` + key + `":`
	pos := strings.Index(jsonText, pattern)
	if pos < 0 {
		return ""
	}
	value := strings.TrimLeft(jsonText[pos+len(pattern):], " \t\r\n")
	if len(value) > 0 && value[0] == '"' {
		value = value[1:]
		if end := strings.IndexByte(value, '"'); end >= 0 {
			return value[:end]
		}
		return value
	}
	if end := strings.IndexAny(value, ",}"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func devuiStripDotZero(value string) string {
	return devuiDotZeroRe.ReplaceAllString(value, "")
}

func fmtNum(value string) string {
	if value == "" || value == "--" {
		return "--"
	}
	if devuiFmtNumRe.MatchString(value) {
		if idx := strings.IndexByte(value, '.'); idx >= 0 {
			return value[:idx]
		}
	}
	return value
}

func signalFloat(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "--" {
		return 0, false
	}
	f, err := strconv.ParseFloat(value, 64)
	return f, err == nil
}

func gradeSignal(value string, kind string) string {
	v, ok := signalFloat(value)
	if !ok {
		return "--"
	}
	switch kind {
	case "rsrp":
		if v >= -80 {
			return "优秀"
		}
		if v >= -90 {
			return "良好"
		}
		if v >= -105 {
			return "一般"
		}
	case "rsrq":
		if v >= -10 {
			return "优秀"
		}
		if v >= -15 {
			return "良好"
		}
		if v >= -20 {
			return "一般"
		}
	case "sinr":
		if v >= 20 {
			return "优秀"
		}
		if v >= 10 {
			return "良好"
		}
		if v >= 0 {
			return "一般"
		}
	case "rssi":
		if v >= -65 {
			return "优秀"
		}
		if v >= -75 {
			return "良好"
		}
		if v >= -85 {
			return "一般"
		}
	}
	return "较差"
}

func signalBarWidth(value string, kind string) int {
	v, ok := signalFloat(value)
	if !ok {
		return 2
	}
	lo, hi := -115.0, -65.0
	switch kind {
	case "rsrq":
		lo, hi = -20, -8
	case "sinr":
		lo, hi = -5, 25
	case "rssi":
		lo, hi = -90, -50
	}
	pct := (v - lo) / (hi - lo)
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	w := int(pct*132 + 0.5)
	if w < 2 {
		return 2
	}
	if w > 132 {
		return 132
	}
	return w
}

func compactBand(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "--"
	}
	value = strings.ReplaceAll(value, "_", " ")
	u := strings.ToUpper(value)
	fields := strings.Fields(u)
	if len(fields) == 3 && fields[0] == "LTE" && fields[1] == "BAND" {
		return "B" + fields[2]
	}
	if len(fields) == 2 && fields[0] == "BAND" {
		return "B" + fields[1]
	}
	return u
}

func lteBWFromCA(ca string) string {
	for _, item := range strings.Split(ca, ";") {
		fields := strings.Split(item, ",")
		if len(fields) >= 5 && strings.TrimSpace(fields[4]) != "" {
			return strings.TrimSpace(fields[4])
		}
	}
	return ""
}

func appendLTECA(lines *[]string, lteca, pci, freq string) {
	count := 0
	skipped := false
	for _, item := range strings.Split(lteca, ";") {
		fields := strings.Split(item, ",")
		if len(fields) < 5 {
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if count >= 2 {
			break
		}
		if !skipped && fields[0] == pci && fields[3] == freq {
			skipped = true
			continue
		}
		count++
		*lines = append(*lines, fields[0], "B"+fields[1], fields[3], fields[4]+"M", "-", "-")
	}
}

func appendNRCA(lines *[]string, nrca string) {
	count := 0
	for _, item := range strings.Split(nrca, ";") {
		fields := strings.Split(item, ",")
		if len(fields) < 11 {
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if count >= 2 {
			break
		}
		count++
		rsrp := fmtNum(fields[7])
		sinr := fmtNum(fields[9])
		*lines = append(*lines, fields[1], "N"+fields[3], fields[4], fields[5]+"M", rsrp, sinr)
	}
}

func cleanSpeed(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	dotSeen := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if r == '.' && !dotSeen {
			dotSeen = true
			b.WriteRune(r)
		}
	}
	return strings.TrimSuffix(b.String(), ".")
}

func SystemRcLocalGetHandler(c *gin.Context) {
	content, err := os.ReadFile(rcLocalPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "读取 rc.local 失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"path": rcLocalPath, "content": string(content)}})
}

func SystemRcLocalSaveHandler(c *gin.Context) {
	type body struct {
		Content string `json:"content"`
	}
	var req body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	if len([]byte(req.Content)) > 512*1024 {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": "rc.local 内容超过 512KB"})
		return
	}
	if err := os.WriteFile(rcLocalPath, []byte(req.Content), 0755); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 3, "msg": "保存 rc.local 失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "保存成功"})
}

func loadSmsMessages() ([]smsMessage, error) {
	data, err := utils.GetDataFromUbus("zwrt_wms", "zte_libwms_get_sms_data", map[string]interface{}{
		"page":          0,
		"data_per_page": 500,
		"mem_store":     1,
		"tags":          10,
		"order_by":      "order by id desc",
	})
	if err != nil {
		return nil, fmt.Errorf("读取短信失败: %w", err)
	}
	rawMessages, ok := data["messages"].([]interface{})
	if !ok {
		return []smsMessage{}, nil
	}
	messages := make([]smsMessage, 0, len(rawMessages))
	for _, item := range rawMessages {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rawHex := stringValue(obj["content"])
		messages = append(messages, smsMessage{
			ID:       intValue(obj["id"]),
			Number:   stringValue(obj["number"]),
			Date:     formatSmsDate(stringValue(obj["date"])),
			Content:  decodeUtf16BEHex(rawHex),
			RawHex:   rawHex,
			Tag:      stringValue(obj["tag"]),
			MemStore: stringValue(obj["mem_store"]),
		})
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].ID > messages[j].ID })
	return messages, nil
}

func decodeUtf16BEHex(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	bytesValue, err := hex.DecodeString(value)
	if err != nil || len(bytesValue) < 2 {
		return value
	}
	if len(bytesValue)%2 == 1 {
		bytesValue = bytesValue[:len(bytesValue)-1]
	}
	u16 := make([]uint16, 0, len(bytesValue)/2)
	for i := 0; i+1 < len(bytesValue); i += 2 {
		u16 = append(u16, uint16(bytesValue[i])<<8|uint16(bytesValue[i+1]))
	}
	return string(utf16.Decode(u16))
}

func sendBark(rawURL string, group string, title string, body string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("Bark 地址为空")
	}
	base, err := url.Parse(rawURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return fmt.Errorf("Bark 地址不合法")
	}

	deviceKey, endpoint := barkDeviceKeyAndEndpoint(base)
	if deviceKey == "" {
		return fmt.Errorf("Bark 地址缺少 device key")
	}
	payloadData := map[string]string{
		"device_key": deviceKey,
		"title":      title,
		"body":       body,
	}
	if icon := strings.TrimSpace(base.Query().Get("icon")); icon != "" {
		payloadData["icon"] = icon
	}
	if group = strings.TrimSpace(group); group != "" {
		payloadData["group"] = group
	} else if group = strings.TrimSpace(base.Query().Get("group")); group != "" {
		payloadData["group"] = group
	}
	payload, err := json.Marshal(payloadData)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return notifyRequest(req)
}

func barkDeviceKeyAndEndpoint(base *url.URL) (string, string) {
	parts := strings.Split(strings.Trim(base.Path, "/"), "/")
	deviceKey := ""
	if len(parts) > 0 && parts[0] != "" && parts[0] != "push" {
		deviceKey = parts[0]
	}
	endpoint := *base
	endpoint.Path = "/push"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return deviceKey, endpoint.String()
}

func sendTelegram(token string, chatID string, text string) error {
	token = strings.TrimSpace(token)
	chatID = strings.TrimSpace(chatID)
	if token == "" || chatID == "" {
		return fmt.Errorf("TG Bot Token 或 Chat ID 为空")
	}
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)
	form.Set("disable_web_page_preview", "true")
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return notifyRequest(req)
}

func notifyRequest(req *http.Request) error {
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func stringValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func formatSmsDate(value string) string {
	parts := strings.Split(value, ",")
	if len(parts) < 6 {
		return value
	}
	nums := make([]int, 6)
	for i := 0; i < 6; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return value
		}
		nums[i] = n
	}
	year := nums[0]
	if year < 100 {
		year += 2000
	}
	return fmt.Sprintf("%04d 年 %02d 月 %02d 日 %02d:%02d:%02d", year, nums[1], nums[2], nums[3], nums[4], nums[5])
}

func intValue(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}
