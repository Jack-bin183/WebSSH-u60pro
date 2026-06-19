package service

import (
	"encoding/json"
	"fmt"
	"gossh/app/model"
	"gossh/app/utils"
	"gossh/gin"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var validNetSelect = map[string]bool{
	"WL_AND_5G":     true,
	"Only_5G":       true,
	"LTE_AND_5G":    true,
	"WCDMA_AND_LTE": true,
	"Only_LTE":      true,
	"Only_WCDMA":    true,
}

var allowed4GBands = map[int]bool{
	1: true, 2: true, 3: true, 4: true, 5: true, 7: true, 8: true, 18: true, 19: true, 20: true, 26: true, 28: true,
	29: true, 32: true, 34: true, 38: true, 39: true, 40: true, 41: true, 42: true, 43: true, 48: true, 66: true, 71: true,
}

var allowed5GBands = map[int]bool{
	1: true, 2: true, 3: true, 5: true, 7: true, 8: true, 18: true, 20: true, 26: true, 28: true, 29: true,
	38: true, 40: true, 41: true, 48: true, 66: true, 71: true, 75: true, 77: true, 78: true, 79: true,
}

type PersistedDeviceSettings struct {
	Wifi24Enabled   *bool  `json:"wifi24_enabled,omitempty"`
	Wifi5Enabled    *bool  `json:"wifi5_enabled,omitempty"`
	WifiTxPower     string `json:"wifi_txpower"`
	Wifi24TxPower   string `json:"wifi24_txpower"`
	Wifi5TxPower    string `json:"wifi5_txpower"`
	WifiCountry     string `json:"wifi_country"`
	Wifi24Country   string `json:"wifi24_country"`
	Wifi5Country    string `json:"wifi5_country"`
	WifiPerformance string `json:"wifi_performance"`
}

func DeviceSettingsGetHandler(c *gin.Context) {
	uid := c.GetUint("uid")
	var s model.UserSetting
	setting, err := s.FindByUid(uid)
	if err != nil || strings.TrimSpace(setting.Value) == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": PersistedDeviceSettings{}})
		return
	}

	var data PersistedDeviceSettings
	if err := json.Unmarshal([]byte(setting.Value), &data); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "读取用户设置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func DeviceSettingsSaveHandler(c *gin.Context) {
	uid := c.GetUint("uid")
	var req PersistedDeviceSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}

	if err := validatePersistedSettings(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	data, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 3, "msg": "序列化用户设置失败"})
		return
	}

	var s model.UserSetting
	if err := s.SaveForUid(uid, string(data)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 4, "msg": "保存用户设置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已保存", "data": req})
}

func NetworkModeSetHandler(c *gin.Context) {
	var req struct {
		NetSelect string `json:"net_select" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !validNetSelect[req.NetSelect] {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "网络模式不合法"})
		return
	}
	data, err := utils.GetDataFromUbus("zte_nwinfo_api", "nwinfo_set_netselect", map[string]interface{}{"net_select": req.NetSelect})
	writeUbusResult(c, data, err)
}

func NetworkLTEBandLockHandler(c *gin.Context) {
	var req struct {
		Bands []int `json:"bands"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	mask, err := lteBandMask(req.Bands)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
		return
	}
	data, err := utils.GetDataFromUbus("zte_nwinfo_api", "nwinfo_set_gwl_bandlock", map[string]interface{}{
		"is_gw_band":    "0",
		"gw_band_mask":  "0",
		"is_lte_band":   "1",
		"lte_band_mask": mask,
	})
	writeUbusResult(c, data, err)
}

func NetworkNRBandLockHandler(c *gin.Context) {
	var req struct {
		Bands []int `json:"bands"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	bands, err := nrBandString(req.Bands)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
		return
	}
	data, err := utils.GetDataFromUbus("zte_nwinfo_api", "nwinfo_set_nrbandlock", map[string]interface{}{
		"nr5g_type": "SA",
		"nr5g_band": bands,
	})
	writeUbusResult(c, data, err)
}

func NetworkLTECellLockHandler(c *gin.Context) {
	var req struct {
		PCI    string `json:"pci"`
		EARFCN string `json:"earfcn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !isNumericOrZero(req.PCI) || !isNumericOrZero(req.EARFCN) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "4G 小区参数不合法"})
		return
	}
	data, err := utils.GetDataFromUbus("zte_nwinfo_api", "nwinfo_lock_lte_cell", map[string]interface{}{
		"lock_lte_pci":    req.PCI,
		"lock_lte_earfcn": req.EARFCN,
	})
	writeUbusResult(c, data, err)
}

func NetworkNRCellLockHandler(c *gin.Context) {
	var req struct {
		PCI    string `json:"pci"`
		EARFCN string `json:"earfcn"`
		Band   string `json:"band"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !isNumericOrZero(req.PCI) || !isNumericOrZero(req.EARFCN) || !isNumericOrZero(req.Band) {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "5G 小区参数不合法"})
		return
	}
	data, err := utils.GetDataFromUbus("zte_nwinfo_api", "nwinfo_lock_nr_cell", map[string]interface{}{
		"lock_nr_pci":       req.PCI,
		"lock_nr_earfcn":    req.EARFCN,
		"lock_nr_cell_band": req.Band,
	})
	writeUbusResult(c, data, err)
}

func WifiUciGetHandler(c *gin.Context) {
	r0, err0 := utils.GetDataFromUbus("uci", "get", map[string]interface{}{"config": "wireless", "section": "wifi0"})
	r1, err1 := utils.GetDataFromUbus("uci", "get", map[string]interface{}{"config": "wireless", "section": "wifi1"})
	if err0 != nil || err1 != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": fmt.Sprintf("读取 WiFi 配置失败: %v %v", err0, err1)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"wifi0": r0, "wifi1": r1}})
}

func WifiSettingsSetHandler(c *gin.Context) {
	var req struct {
		Wifi0 map[string]string `json:"wifi0"`
		Wifi1 map[string]string `json:"wifi1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	params := map[string]interface{}{}
	if len(req.Wifi0) > 0 {
		v, err := sanitizeWifiAttrs(req.Wifi0)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
			return
		}
		params["wifi0"] = v
	}
	if len(req.Wifi1) > 0 {
		v, err := sanitizeWifiAttrs(req.Wifi1)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 2, "msg": err.Error()})
			return
		}
		params["wifi1"] = v
	}
	if len(params) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "没有可应用的 WiFi 设置"})
		return
	}
	data, err := utils.GetDataFromUbus("zwrt_wlan", "set", params)
	writeUbusResult(c, data, err)
}

func lteBandMask(bands []int) (string, error) {
	mask := big.NewInt(0)
	seen := map[int]bool{}
	for _, band := range bands {
		if !allowed4GBands[band] {
			return "", fmt.Errorf("不支持的 4G 频段: B%d", band)
		}
		if seen[band] {
			continue
		}
		seen[band] = true
		mask.Or(mask, new(big.Int).Lsh(big.NewInt(1), uint(band-1)))
	}
	return mask.String(), nil
}

func nrBandString(bands []int) (string, error) {
	parts := make([]string, 0, len(bands))
	seen := map[int]bool{}
	for _, band := range bands {
		if !allowed5GBands[band] {
			return "", fmt.Errorf("不支持的 5G 频段: N%d", band)
		}
		if seen[band] {
			continue
		}
		seen[band] = true
		parts = append(parts, strconv.Itoa(band))
	}
	return strings.Join(parts, ","), nil
}

func sanitizeWifiAttrs(attrs map[string]string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	for key, value := range attrs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "txpowerpercent":
			n, err := strconv.Atoi(value)
			if err != nil || (n != 40 && n != 80 && n != 100) {
				return nil, fmt.Errorf("WiFi 发射功率只能是 40、80、100")
			}
		case "country":
			if !regexp.MustCompile(`^[A-Za-z]{2}$`).MatchString(value) {
				return nil, fmt.Errorf("WiFi 国家码必须是 2 位字母")
			}
			value = strings.ToUpper(value)
		default:
			return nil, fmt.Errorf("不支持的 WiFi 设置项: %s", key)
		}
		out[key] = value
	}
	return out, nil
}

func validatePersistedSettings(s PersistedDeviceSettings) error {
	for _, v := range []string{s.WifiTxPower, s.Wifi24TxPower, s.Wifi5TxPower} {
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || (n != 40 && n != 80 && n != 100) {
			return fmt.Errorf("WiFi 发射功率只能是 40、80、100")
		}
	}
	for _, v := range []string{s.WifiCountry, s.Wifi24Country, s.Wifi5Country} {
		if v == "" {
			continue
		}
		if !regexp.MustCompile(`^[A-Za-z]{2}$`).MatchString(v) {
			return fmt.Errorf("WiFi 国家码必须是 2 位字母")
		}
	}
	if s.WifiPerformance != "" && s.WifiPerformance != "high" && s.WifiPerformance != "power_save" {
		return fmt.Errorf("WiFi 性能模式不合法")
	}
	return nil
}

func isNumericOrZero(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	_, err := strconv.Atoi(v)
	return err == nil
}

func writeUbusResult(c *gin.Context, data map[string]interface{}, err error) {
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}
