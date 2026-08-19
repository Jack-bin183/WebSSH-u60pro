package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCombinedNegativeControlRejectsEitherRATLeak(t *testing.T) {
	if !controlHasUnexpectedTarget(runRecord{NRCellCount: 1}, targetCombined) {
		t.Fatal("NR-only leak was accepted by combined negative control")
	}
	if !controlHasUnexpectedTarget(runRecord{LTECellCount: 1}, targetCombined) {
		t.Fatal("LTE-only leak was accepted by combined negative control")
	}
	if controlHasUnexpectedTarget(runRecord{}, targetCombined) {
		t.Fatal("empty negative control was rejected")
	}
}

func TestValidateControlsReportRequiresColdCompletePositiveBatch(t *testing.T) {
	report := controlsSummary{
		Target: targetCombined, ColdStateConfirmed: true, PositiveControlStatus: true,
		Completed: time.Now(),
	}
	for _, name := range []string{"all-off", "message-only", "positive", "zero-mask-private", "positive-end"} {
		report.Groups = append(report.Groups, controlGroupResult{Name: name})
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "controls.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []targetKind{targetNR, targetLTE, targetCombined} {
		if err := validateControlsReport(path, target); err != nil {
			t.Fatalf("combined controls should cover %s: %v", target, err)
		}
	}
	report.PositiveControlStatus = false
	data, _ = json.Marshal(report)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateControlsReport(path, targetNR); err == nil {
		t.Fatal("failed positive control report was accepted")
	}
}
