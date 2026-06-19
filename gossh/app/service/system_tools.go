package service

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gossh/app/utils"
	"gossh/gin"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const rcLocalPath = "/etc/rc.local"
const smsForwardDefaultDir = "/data/kano_plugins/sms_forward"
const smsForwardConfigName = "config.json"
const smsForwardAutostartMarker = ".autostart"
const smsForwardPollInterval = 3 * time.Second
const smsForwardCompleteQuietWindow = 3 * time.Second
const smsForwardCompleteMaxWait = 9 * time.Second

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

var smsForwardMu sync.Mutex
var smsForwardStop chan struct{}
var smsForwardStatus smsForwardRuntimeStatus

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
			if err := sendBark(req.BarkURL, title, text); err != nil {
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
		if err := sendBark(cfg.BarkURL, title, text); err != nil {
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

func sendBark(rawURL string, title string, body string) error {
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
