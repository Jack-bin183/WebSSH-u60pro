package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gossh/gin"
)

const defaultTrafficSpeedTestURL = "https://autopatchcn.yuanshen.com/client_app/download/pc_zip/20211117173857_8JkfDHNPmqKi67qR/YuanShen_2.3.0.zip"

const (
	speedTestMaxConcurrent = 8
	speedTestCopyBufSize   = 512 * 1024
)

var speedTestLimiter = make(chan struct{}, speedTestMaxConcurrent)

var trafficSpeedTestClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect too many times")
		}
		return validateSpeedTestURL(req.URL.String())
	},
}

func SpeedTestHandler(c *gin.Context) {
	target := strings.TrimSpace(c.Query("url"))
	if target == "" {
		target = defaultTrafficSpeedTestURL
	}
	if err := validateSpeedTestURL(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	select {
	case speedTestLimiter <- struct{}{}:
		defer func() { <-speedTestLimiter }()
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 1, "msg": "测速请求过多，请稍后再试"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "测速地址不合法"})
		return
	}
	req.Header.Set("User-Agent", "WebSSH-u60pro-SpeedTest/1.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := trafficSpeedTestClient.Do(req)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"code": 1, "msg": "测速地址请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.JSON(http.StatusBadGateway, gin.H{"code": 1, "msg": fmt.Sprintf("测速地址返回 HTTP %d", resp.StatusCode)})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	if resp.ContentLength > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	c.Header("Content-Disposition", "attachment; filename=traffic-speedtest.bin")
	c.Header("Content-Encoding", "identity")
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Writer.WriteHeaderNow()

	buf := make([]byte, speedTestCopyBufSize)
	if _, err := io.CopyBuffer(c.Writer, resp.Body, buf); err != nil && !isContextCanceled(c.Request.Context()) {
		return
	}
}

func validateSpeedTestURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("测速地址不合法")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("测速地址只支持 http/https")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("测速地址缺少主机名")
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return errors.New("测速地址解析失败")
	}
	for _, ip := range ips {
		if isBlockedSpeedTestIP(ip) {
			return errors.New("测速地址不能指向本机或内网地址")
		}
	}
	return nil
}

func isBlockedSpeedTestIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsMulticast()
}

func isContextCanceled(ctx context.Context) bool {
	return ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded
}
