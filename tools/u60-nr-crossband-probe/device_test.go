package main

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRestoreTransactionOrderAndJournalRetention(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "ubus.log")
	ubusPath := filepath.Join(directory, "ubus")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\nprintf '{}\\n'\n"
	if err := os.WriteFile(ubusPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(directory, "state.json")
	state := &recoveryState{
		Schema: stateSchema, ToolVersion: toolVersion, RunID: "test", Experiment: experimentE,
		CreatedAt: time.Now().UTC(), Status: "running", Stage: "mutated",
		Snapshot: networkSnapshot{
			Raw: map[string]any{}, NetSelect: "WL_AND_5G", NRBandSA: "1,28,41", NRBandNSA: "41",
			NRCellLockRaw: "123,504990,41", NRCellLockPCI: "123", NRCellLockARFCN: "504990", NRCellLockBand: "41",
		},
	}
	controller := &deviceController{UBUS: ubusPath, StatePath: statePath}
	if err := controller.writeState(state); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := controller.restoreNetwork(ctx, state); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("recovery state was removed before trace recovery completed: %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	wantOrder := []string{"nwinfo_lock_nr_cell", `"nr5g_type":"SA"`, `"nr5g_type":"NSA"`, "nwinfo_set_netselect", `"lock_nr_pci":"123"`}
	position := -1
	for _, wanted := range wantOrder {
		next := strings.Index(log[position+1:], wanted)
		if next < 0 {
			t.Fatalf("%q missing after position %d in:\n%s", wanted, position, log)
		}
		position += next + 1
	}
}

func TestSplitCellLock(t *testing.T) {
	values := splitCellLock("123, 504990;41")
	if strings.Join(values, ",") != "123,504990,41" {
		t.Fatalf("values=%v", values)
	}
}

func TestRestoreProductionTraceFromJournal(t *testing.T) {
	directory := t.TempDir()
	productionPath := filepath.Join("..", "..", "gossh", "app", "service", "embed", "qtrace.cfg")
	production, err := os.ReadFile(productionPath)
	if err != nil {
		t.Fatal(err)
	}
	diagPath := filepath.Join(directory, "diag_mdlog")
	if err := os.WriteFile(diagPath, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	state := &recoveryState{
		TraceRestoreRequired:   true,
		ProductionQTraceBase64: base64.StdEncoding.EncodeToString(production),
		DiagMDLog:              diagPath,
		WorkDir:                directory,
		StopGraceMS:            100,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := restoreProductionTraceFromState(ctx, state, ""); err != nil {
		t.Fatal(err)
	}
	if state.TraceRestoreRequired {
		t.Fatal("trace restore flag remains set")
	}
}
