package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCSVReportColumnsStayAligned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.csv")
	record := experimentRecord{
		RunID: "test", Experiment: experimentA, StartedAt: time.Now().UTC(),
		Target: targetSpec{Band: 41, ARFCN: 504990}, QMITraceComplete: true,
	}
	if err := appendCSVReport(path, record); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(rows[0]) != len(rows[1]) {
		t.Fatalf("rows=%v", rows)
	}
}
