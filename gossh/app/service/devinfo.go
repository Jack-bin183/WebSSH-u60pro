package service

import (
	"bufio"
	"fmt"
	"gossh/app/utils"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	devInfoPath         = "/tmp/devinfo"
	devInfoPollInterval = time.Second
)

type cpuTimes struct {
	total uint64
	idle  uint64
}

type devInfoValues struct {
	temperature int
	cpuUsage    int
	ramUsage    int
	qci         int
}

// InitDevInfoWriter starts the device status feed consumed by the screen UI.
// The file is replaced atomically so readers never observe a partial update.
func InitDevInfoWriter() {
	go runDevInfoWriter(devInfoPath, devInfoPollInterval)
}

func runDevInfoWriter(path string, interval time.Duration) {
	previousCPU, _ := readCPUTimes("/proc/stat")
	values := devInfoValues{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		fallbackCPUUsage := values.cpuUsage
		currentCPU, err := readCPUTimes("/proc/stat")
		if err == nil {
			if usage, ok := calculateCPUUsage(previousCPU, currentCPU); ok {
				fallbackCPUUsage = usage
			}
			previousCPU = currentCPU
		}

		if cpuUsage, ramUsage, err := readFrontendDeviceUsage(); err == nil {
			values.cpuUsage = cpuUsage
			values.ramUsage = ramUsage
		} else {
			values.cpuUsage = fallbackCPUUsage
			if ramUsage, err := readRAMUsage("/proc/meminfo"); err == nil {
				values.ramUsage = ramUsage
			}
		}
		if temperature, err := readCPUTemperature(); err == nil {
			values.temperature = temperature
		}
		if qci, err := readLatestQCI(); err == nil && qci > 0 {
			values.qci = qci
		}

		_ = writeAtomicFile(path, formatDevInfo(values))
	}
}

func readFrontendDeviceUsage() (int, int, error) {
	data, err := utils.GetDataFromUbus("zwrt_mc.device.manager", "get_device_info", map[string]interface{}{})
	if err != nil {
		return 0, 0, err
	}
	return parseFrontendDeviceUsage(data)
}

func parseFrontendDeviceUsage(data map[string]interface{}) (int, int, error) {
	cpuInfo, ok := data["cpuinfo"].([]interface{})
	if !ok || len(cpuInfo) == 0 {
		return 0, 0, fmt.Errorf("cpuinfo is missing")
	}
	totalCPU, ok := cpuInfo[0].(map[string]interface{})
	if !ok {
		return 0, 0, fmt.Errorf("aggregate cpuinfo is invalid")
	}
	idle, err := numericValue(totalCPU["idle"])
	if err != nil {
		return 0, 0, fmt.Errorf("CPU idle is invalid: %w", err)
	}

	memInfo, ok := data["meminfo"].(map[string]interface{})
	if !ok {
		return 0, 0, fmt.Errorf("meminfo is missing")
	}
	totalRAM, err := numericValue(memInfo["total"])
	if err != nil || totalRAM <= 0 {
		return 0, 0, fmt.Errorf("RAM total is invalid")
	}
	availableValue := memInfo["avaliable"]
	if availableValue == nil {
		availableValue = memInfo["available"]
	}
	availableRAM, err := numericValue(availableValue)
	if err != nil {
		return 0, 0, fmt.Errorf("RAM available is invalid: %w", err)
	}

	cpuUsage := clampUsage(math.Round(100 - idle))
	ramUsage := clampUsage(math.Round((totalRAM - availableRAM) * 100 / totalRAM))
	return cpuUsage, ramUsage, nil
}

func numericValue(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(typed), 64)
	default:
		return 0, fmt.Errorf("unsupported numeric value %v", value)
	}
}

func readCPUTimes(path string) (cpuTimes, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cpuTimes{}, err
	}
	return parseCPUTimes(string(data))
}

func parseCPUTimes(data string) (cpuTimes, error) {
	line, _, _ := strings.Cut(data, "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, fmt.Errorf("invalid aggregate CPU statistics")
	}

	// /proc/stat includes guest time inside user/nice already, so only the
	// first eight counters take part in the total.
	limit := len(fields)
	if limit > 9 {
		limit = 9
	}
	var counters []uint64
	for _, field := range fields[1:limit] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("invalid CPU counter %q: %w", field, err)
		}
		counters = append(counters, value)
	}

	var total uint64
	for _, value := range counters {
		total += value
	}
	idle := counters[3]
	if len(counters) > 4 {
		idle += counters[4]
	}
	return cpuTimes{total: total, idle: idle}, nil
}

func calculateCPUUsage(previous, current cpuTimes) (int, bool) {
	if current.total <= previous.total || current.idle < previous.idle {
		return 0, false
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if idleDelta > totalDelta {
		return 0, false
	}
	return clampUsage(math.Round(float64(totalDelta-idleDelta) * 100 / float64(totalDelta))), true
}

func readRAMUsage(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}

	total := values["MemTotal"]
	if total == 0 {
		return 0, fmt.Errorf("MemTotal is missing")
	}
	available, ok := values["MemAvailable"]
	if !ok {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if available > total {
		available = total
	}
	return clampUsage(math.Round(float64(total-available) * 100 / float64(total))), nil
}

func readCPUTemperature() (int, error) {
	if temperature, err := readCPUTemperatureFromUbus(); err == nil {
		return temperature, nil
	}
	return readCPUSysfsTemperature("/sys/class/thermal")
}

func readCPUSysfsTemperature(thermalRoot string) (int, error) {
	zones, err := filepath.Glob(filepath.Join(thermalRoot, "thermal_zone*"))
	if err != nil {
		return 0, err
	}

	var hottestMilliCelsius int64
	found := false
	for _, zone := range zones {
		typeData, err := os.ReadFile(filepath.Join(zone, "type"))
		if err != nil || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(typeData))), "cpuss") {
			continue
		}

		tempData, err := os.ReadFile(filepath.Join(zone, "temp"))
		if err != nil {
			continue
		}
		temperature, err := strconv.ParseInt(strings.TrimSpace(string(tempData)), 10, 64)
		if err != nil || temperature < 0 || temperature > 200000 {
			continue
		}
		if !found || temperature > hottestMilliCelsius {
			hottestMilliCelsius = temperature
			found = true
		}
	}
	if !found {
		return 0, fmt.Errorf("cpuss thermal zone is missing")
	}
	return int(math.Round(float64(hottestMilliCelsius) / 1000)), nil
}

func readCPUTemperatureFromUbus() (int, error) {
	data, err := utils.GetDataFromUbus("zwrt_bsp.thermal", "get_cpu_temp", map[string]interface{}{})
	if err != nil {
		return 0, err
	}
	value, ok := data["cpuss_temp"]
	if !ok {
		return 0, fmt.Errorf("cpuss_temp is missing")
	}

	switch typed := value.(type) {
	case float64:
		return int(math.Round(typed)), nil
	case string:
		temperature, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, err
		}
		return int(math.Round(temperature)), nil
	default:
		return 0, fmt.Errorf("unsupported cpuss_temp value %v", value)
	}
}

func readLatestQCI() (int, error) {
	line, err := getLatestQci()
	if err != nil {
		return 0, err
	}
	qci1, qci2 := parseQci(line)
	if qci2 > 0 {
		return qci2, nil
	}
	if qci1 > 0 {
		return qci1, nil
	}
	return 0, fmt.Errorf("QCI is missing")
}

func formatDevInfo(values devInfoValues) []byte {
	return []byte(fmt.Sprintf("%d°\nCPU %d%%\nRAM %d%%\nQCI %d\n",
		values.temperature,
		values.cpuUsage,
		values.ramUsage,
		values.qci,
	))
}

func clampUsage(value float64) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int(value)
}
