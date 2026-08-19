package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagArgumentsNeverUseGlobalCleanMask(t *testing.T) {
	options := deviceOptions{FileSizeMB: 4, FileCount: 32}
	arguments := options.diagArguments("/tmp/candidate.cfg", "/tmp/capture")
	for _, argument := range arguments {
		if argument == "-c" || argument == "--cleanmask" {
			t.Fatalf("diag_mdlog arguments contain global cleanup: %v", arguments)
		}
	}
	want := []string{"-f", "/tmp/candidate.cfg", "-o", "/tmp/capture", "-s", "4", "-n", "32", "-d"}
	if len(arguments) != len(want) {
		t.Fatalf("arguments=%v, want=%v", arguments, want)
	}
	for index := range want {
		if arguments[index] != want[index] {
			t.Fatalf("arguments=%v, want=%v", arguments, want)
		}
	}
}

func TestReducerLockRecoversAfterPreviousOwnerExits(t *testing.T) {
	config, _, err := parseQTraceConfig(originalQTraceConfig)
	if err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(t.TempDir(), "work")
	newRunner := func(reportName string) *deviceOptions {
		reports, reportErr := newReportWriter(filepath.Join(t.TempDir(), reportName))
		if reportErr != nil {
			t.Fatal(reportErr)
		}
		t.Cleanup(func() { _ = reports.Close() })
		return &deviceOptions{Config: config, WorkDir: workDir, Reports: reports}
	}
	first := newRunner("first-reports")
	second := newRunner("second-reports")
	if err := first.acquire(); err != nil {
		t.Fatal(err)
	}
	if err := second.acquire(); err == nil || !strings.Contains(err.Error(), "another reducer owns") {
		t.Fatalf("second acquire error = %v, want live-lock rejection", err)
	}
	first.release()
	if err := second.acquire(); err != nil {
		t.Fatalf("acquire after prior owner release: %v", err)
	}
	second.release()
}
