package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"gossh/app/model"
	"gossh/gorm"
	"log/slog"
	"strings"
	"time"
)

const (
	wifiSettingsAutostartInitialDelay = 8 * time.Second
	wifiSettingsAutostartMaxWait      = 2 * time.Minute
	wifiSettingsAutostartRetry        = 5 * time.Second
	wifiSettingsStatePollInterval     = 12 * time.Second
)

// InitWifiSettingsAutostart reapplies saved WiFi runtime settings after boot.
// The wireless UCI settings such as country and txpower persist naturally; the
// radio link state and power-save mode are runtime state and need replay.
func InitWifiSettingsAutostart() {
	go func() {
		time.Sleep(wifiSettingsAutostartInitialDelay)

		settings, ok, err := loadLatestPersistedDeviceSettings()
		if err != nil {
			slog.Warn("wifi settings autostart: load settings failed", "err", err)
			return
		}
		if !ok || !hasWifiRuntimeSettings(settings) {
			return
		}

		if err := waitWifiInterfaces([]string{"wlan0", "wlan2"}, wifiSettingsAutostartMaxWait); err != nil {
			slog.Warn("wifi settings autostart: wifi interfaces not ready", "err", err)
			return
		}

		if err := applyPersistedWifiRuntimeSettings(settings); err != nil {
			slog.Warn("wifi settings autostart: apply failed", "err", err)
			return
		}
		slog.Info("wifi settings autostart: applied")
	}()

	go pollPersistedWifiState()
}

func loadLatestPersistedDeviceSettings() (PersistedDeviceSettings, bool, error) {
	var s model.UserSetting
	setting, err := s.FindLatestNonEmpty()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PersistedDeviceSettings{}, false, nil
		}
		return PersistedDeviceSettings{}, false, err
	}

	var data PersistedDeviceSettings
	if err := json.Unmarshal([]byte(setting.Value), &data); err != nil {
		return PersistedDeviceSettings{}, false, err
	}
	return data, true, nil
}

func hasWifiRuntimeSettings(settings PersistedDeviceSettings) bool {
	return settings.Wifi24Enabled != nil ||
		settings.Wifi5Enabled != nil ||
		strings.TrimSpace(settings.WifiPerformance) != ""
}

func waitWifiInterfaces(ifaces []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for {
		ready := true
		for _, iface := range ifaces {
			if _, err := getLinkStatus(iface); err != nil {
				ready = false
				lastErr = err
				break
			}
		}
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("wifi interfaces not ready")
		}
		time.Sleep(wifiSettingsAutostartRetry)
	}
}

func applyPersistedWifiRuntimeSettings(settings PersistedDeviceSettings) error {
	if settings.Wifi24Enabled != nil && *settings.Wifi24Enabled {
		if err := setWifiState("wlan0", true); err != nil {
			return fmt.Errorf("enable wlan0 failed: %w", err)
		}
	}
	if settings.Wifi5Enabled != nil && *settings.Wifi5Enabled {
		if err := setWifiState("wlan2", true); err != nil {
			return fmt.Errorf("enable wlan2 failed: %w", err)
		}
	}

	if mode := wifiPowerSaveMode(settings.WifiPerformance); mode != "" {
		for _, iface := range []string{"wlan0", "wlan1", "wlan2", "wlan3"} {
			if err := setPowerSave(iface, mode); err != nil {
				slog.Warn("wifi settings autostart: set power save failed", "iface", iface, "mode", mode, "err", err)
			}
		}
	}

	if settings.Wifi24Enabled != nil {
		if err := setWifiState("wlan0", *settings.Wifi24Enabled); err != nil {
			return fmt.Errorf("set wlan0 state failed: %w", err)
		}
	}
	if settings.Wifi5Enabled != nil {
		if err := setWifiState("wlan2", *settings.Wifi5Enabled); err != nil {
			return fmt.Errorf("set wlan2 state failed: %w", err)
		}
	}
	return nil
}

func pollPersistedWifiState() {
	ticker := time.NewTicker(wifiSettingsStatePollInterval)
	defer ticker.Stop()

	for range ticker.C {
		settings, ok, err := loadLatestPersistedDeviceSettings()
		if err != nil {
			slog.Warn("wifi state poll: load settings failed", "err", err)
			continue
		}
		if !ok || !hasPersistedWifiState(settings) {
			continue
		}

		if err := reconcilePersistedWifiState(settings); err != nil {
			slog.Warn("wifi state poll: reconcile failed", "err", err)
		}
	}
}

func hasPersistedWifiState(settings PersistedDeviceSettings) bool {
	return settings.Wifi24Enabled != nil || settings.Wifi5Enabled != nil
}

func reconcilePersistedWifiState(settings PersistedDeviceSettings) error {
	if settings.Wifi24Enabled != nil {
		if err := reconcileWifiIfaceState("wlan0", *settings.Wifi24Enabled); err != nil {
			return fmt.Errorf("reconcile wlan0 failed: %w", err)
		}
	}
	if settings.Wifi5Enabled != nil {
		if err := reconcileWifiIfaceState("wlan2", *settings.Wifi5Enabled); err != nil {
			return fmt.Errorf("reconcile wlan2 failed: %w", err)
		}
	}
	return nil
}

func reconcileWifiIfaceState(iface string, expectedUp bool) error {
	status, err := getLinkStatus(iface)
	if err != nil {
		return err
	}

	currentUp := status == "up"
	if status != "up" && status != "down" {
		return fmt.Errorf("%s status unknown: %s", iface, status)
	}
	if currentUp == expectedUp {
		return nil
	}

	if err := setWifiState(iface, expectedUp); err != nil {
		return err
	}
	slog.Info("wifi state poll: corrected iface state", "iface", iface, "expected_up", expectedUp)
	return nil
}

func wifiPowerSaveMode(performance string) string {
	switch strings.TrimSpace(performance) {
	case "high":
		return "off"
	case "power_save":
		return "on"
	default:
		return ""
	}
}
