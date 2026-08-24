package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCPUTimesAndUsage(t *testing.T) {
	previous, err := parseCPUTimes("cpu  100 0 50 850 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	current, err := parseCPUTimes("cpu  130 0 60 910 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	usage, ok := calculateCPUUsage(previous, current)
	if !ok || usage != 40 {
		t.Fatalf("calculateCPUUsage() = %d, %v; want 40, true", usage, ok)
	}
}

func TestReadRAMUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meminfo")
	data := []byte("MemTotal:       1000 kB\nMemFree:         100 kB\nMemAvailable:    390 kB\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	usage, err := readRAMUsage(path)
	if err != nil {
		t.Fatal(err)
	}
	if usage != 61 {
		t.Fatalf("readRAMUsage() = %d; want 61", usage)
	}
}

func TestParseFrontendDeviceUsage(t *testing.T) {
	cpuUsage, ramUsage, err := parseFrontendDeviceUsage(map[string]interface{}{
		"cpuinfo": []interface{}{
			map[string]interface{}{"name": "cpu", "idle": "50"},
		},
		"meminfo": map[string]interface{}{
			"total":     "1000",
			"avaliable": "390",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cpuUsage != 50 || ramUsage != 61 {
		t.Fatalf("parseFrontendDeviceUsage() = CPU %d, RAM %d; want CPU 50, RAM 61", cpuUsage, ramUsage)
	}
}

func TestFormatDevInfo(t *testing.T) {
	got := string(formatDevInfo(devInfoValues{
		temperature: 48,
		cpuUsage:    50,
		ramUsage:    61,
		qci:         6,
	}))
	want := "48°\nCPU 50%\nRAM 61%\nQCI 6\n"
	if got != want {
		t.Fatalf("formatDevInfo() = %q; want %q", got, want)
	}
}

func TestReadCPUSysfsTemperatureUsesHottestCPUZone(t *testing.T) {
	thermalRoot := t.TempDir()
	writeThermalZone := func(name, zoneType, temperature string) {
		t.Helper()
		zone := filepath.Join(thermalRoot, name)
		if err := os.Mkdir(zone, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(zone, "type"), []byte(zoneType+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(zone, "temp"), []byte(temperature+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	writeThermalZone("thermal_zone15", "aoss-0", "60000")
	writeThermalZone("thermal_zone16", "cpuss-0", "49800")
	writeThermalZone("thermal_zone17", "cpuss-1", "49000")
	writeThermalZone("thermal_zone18", "cpuss-2", "-273000")

	temperature, err := readCPUSysfsTemperature(thermalRoot)
	if err != nil {
		t.Fatal(err)
	}
	if temperature != 50 {
		t.Fatalf("readCPUSysfsTemperature() = %d; want 50", temperature)
	}
}

func TestParseQciSingleAndMultipleValues(t *testing.T) {
	for _, test := range []struct {
		name  string
		line  string
		want1 int
		want2 int
	}{
		{name: "single QCI", line: "bearer qci=6", want1: 6},
		{name: "single 5QI", line: "session 5qi: 9", want1: 9},
		{name: "two values", line: "qci=5 qci=6", want1: 5, want2: 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			got1, got2 := parseQci(test.line)
			if got1 != test.want1 || got2 != test.want2 {
				t.Fatalf("parseQci(%q) = %d, %d; want %d, %d", test.line, got1, got2, test.want1, test.want2)
			}
		})
	}
}
