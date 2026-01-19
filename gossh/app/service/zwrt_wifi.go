package service

import (
	// "gossh/app/utils"
	// "log/slog"
	// "gossh/gin"
	// "net/http"
	// "sort"
	// "sync"
	"fmt"
	"os/exec"
	"strings"
	"gossh/gin"
)

func WifiPsmGetHandler(c *gin.Context) {
	var req struct {
		Ifaces []string `json:"ifaces"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "bad request"})
		return
	}
	data, _ := BatchGetPowerSave(req.Ifaces)
	c.JSON(200, gin.H{
		"code": 0,
		"data": data,
	})
}
func BatchGetPowerSave(ifaces []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, iface := range ifaces {
		// power save
		psm, err := getPowerSave(iface)
		if err != nil {
			psm = "unknown"
		}
		result[iface+"_psm"] = psm
		// link status
		status, err := getLinkStatus(iface)
		if err != nil {
			status = "unknown"
		}
		result[iface+"_status"] = status
	}
	return result, nil
}
func getPowerSave(iface string) (string, error) {
	cmd := exec.Command("iw", "dev", iface, "get", "power_save")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("iw error: %s", string(out))
	}
	// 输出一般是：Power save: on
	fields := strings.Fields(string(out))
	return fields[len(fields)-1], nil
}
func getLinkStatus(iface string) (string, error) {
	cmd := exec.Command("ip", "link", "show", iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip link error: %s", string(out))
	}
	output := string(out)
	if strings.Contains(output, "state UP") {
		return "up", nil
	}
	if strings.Contains(output, "state DOWN") {
		return "down", nil
	}
	return "unknown", nil
}

func WifiPsmSetHandler(c *gin.Context) {
	var req struct {
		Ifaces []string `json:"ifaces"`
		Mode   string   `json:"mode"` // on / off
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "bad request"})
		return
	}
	result := BatchSetPowerSave(req.Ifaces, req.Mode)
	c.JSON(200, gin.H{
		"code": 0,
		"result": result,
	})
}
func BatchSetPowerSave(ifaces []string, mode string) map[string]string {
	result := make(map[string]string)

	for _, iface := range ifaces {
		if err := setPowerSave(iface, mode); err != nil {
			result[iface] = "fail"
		} else {
			result[iface] = "ok"
		}
	}

	return result
}
func setPowerSave(iface, mode string) error {
	if mode != "on" && mode != "off" {
		return fmt.Errorf("invalid mode")
	}
	cmd := exec.Command("iw", "dev", iface, "set", "power_save", mode)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iw error: %s", string(out))
	}
	return nil
}



func setWifiState(iface string, up bool) error {
	state := "down"
	if up {
		state = "up"
	}

	cmd := exec.Command("ifconfig", iface, state)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ifconfig %s %s failed: %s", iface, state, string(out))
	}
	return nil
}
func BatchSetWifiState(ifaces []string, up bool) map[string]string {
	result := make(map[string]string)

	for _, iface := range ifaces {
		if err := setWifiState(iface, up); err != nil {
			result[iface] = "fail"
		} else {
			result[iface] = "ok"
		}
	}

	return result
}
func WifiStateSetHandler(c *gin.Context) {
	var req struct {
		Ifaces []string `json:"ifaces"`
		Up     bool     `json:"up"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "bad request"})
		return
	}

	result := BatchSetWifiState(req.Ifaces, req.Up)

	c.JSON(200, gin.H{
		"code":   0,
		"result": result,
	})
}

