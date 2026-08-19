package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultStatePath = "/tmp/u60-nr-crossband-probe/state.json"

type deviceController struct {
	UBUS      string
	StatePath string
	Actions   *[]actionEvent
	QMIEvents *[]qmiEvent
	mu        sync.Mutex
}

func (controller *deviceController) appendQMI(event qmiEvent) {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.QMIEvents != nil {
		*controller.QMIEvents = append(*controller.QMIEvents, event)
	}
}

func (controller *deviceController) snapshot(ctx context.Context) (networkSnapshot, error) {
	raw, err := controller.callUBUS(ctx, "nwinfo_get_netinfo", map[string]any{})
	if err != nil {
		return networkSnapshot{}, err
	}
	snapshot := networkSnapshot{
		CapturedAt: time.Now().UTC(), Raw: raw,
		NetSelect: stringValue(raw, "net_select"), NetworkType: stringValue(raw, "network_type"),
		NRBandSA: stringValue(raw, "nr5g_sa_band_lock"), NRBandNSA: stringValue(raw, "nr5g_nsa_band_lock"),
		NRCellLockRaw: stringValue(raw, "lock_nr_cell"), ServingBand: stringValue(raw, "nr5g_action_band"),
		ServingARFCN: stringValue(raw, "nr5g_action_channel"), ServingPCI: stringValue(raw, "nr5g_pci"),
	}
	lock := splitCellLock(snapshot.NRCellLockRaw)
	if len(lock) > 0 {
		snapshot.NRCellLockPCI = lock[0]
	}
	if len(lock) > 1 {
		snapshot.NRCellLockARFCN = lock[1]
	}
	if len(lock) > 2 {
		snapshot.NRCellLockBand = lock[2]
	}
	return snapshot, nil
}

func (controller *deviceController) setNRBand(ctx context.Context, kind, bands string) error {
	if kind == "" {
		kind = "SA"
	}
	_, err := controller.callAndRecord(ctx, "set-nr-band-"+strings.ToLower(kind), "nwinfo_set_nrbandlock", map[string]any{
		"nr5g_type": strings.ToUpper(kind), "nr5g_band": bands,
	})
	return err
}

func (controller *deviceController) setNetSelect(ctx context.Context, mode string) error {
	if mode == "" {
		return errors.New("cannot set empty net_select")
	}
	_, err := controller.callAndRecord(ctx, "set-net-select", "nwinfo_set_netselect", map[string]any{"net_select": mode})
	return err
}

func (controller *deviceController) setNRCellLock(ctx context.Context, pci, arfcn, band string) error {
	_, err := controller.callAndRecord(ctx, "set-nr-cell-lock", "nwinfo_lock_nr_cell", map[string]any{
		"lock_nr_pci": pci, "lock_nr_earfcn": arfcn, "lock_nr_cell_band": band,
	})
	return err
}

func (controller *deviceController) callAndRecord(ctx context.Context, action, method string, request map[string]any) (map[string]any, error) {
	response, err := controller.callUBUS(ctx, method, request)
	event := actionEvent{At: time.Now().UTC(), Action: action, Request: request, Response: response}
	if err != nil {
		event.Error = err.Error()
	}
	if controller.Actions != nil {
		*controller.Actions = append(*controller.Actions, event)
	}
	return response, err
}

func (controller *deviceController) callUBUS(ctx context.Context, method string, request map[string]any) (map[string]any, error) {
	ubus := controller.UBUS
	if ubus == "" {
		ubus = "ubus"
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	command := exec.CommandContext(callCtx, ubus, "call", "zte_nwinfo_api", method, string(payload))
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ubus %s failed: %w: %s", method, err, strings.TrimSpace(string(output)))
	}
	result := map[string]any{}
	if len(strings.TrimSpace(string(output))) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("decode ubus %s response: %w: %s", method, err, strings.TrimSpace(string(output)))
	}
	return result, nil
}

func (controller *deviceController) writeState(state *recoveryState) error {
	if controller.StatePath == "" {
		controller.StatePath = defaultStatePath
	}
	state.UpdatedAt = time.Now().UTC()
	return writeJSONAtomic(controller.StatePath, state, 0600)
}

func (controller *deviceController) restoreNetwork(ctx context.Context, state *recoveryState) error {
	state.Status = "restoring"
	state.Stage = "restore-started"
	state.RestoreAttempts++
	if err := controller.writeState(state); err != nil {
		return err
	}
	var restoreErrors []error
	record := func(stage string, err error) {
		state.Stage = stage
		if err != nil {
			restoreErrors = append(restoreErrors, err)
			state.LastError = err.Error()
		}
		_ = controller.writeState(state)
	}

	// Remove any experimental lock before restoring broad preferences.
	record("restore-unlock-cell", controller.setNRCellLock(ctx, "0", "0", "0"))
	record("restore-sa-band", controller.setNRBand(ctx, "SA", state.Snapshot.NRBandSA))
	record("restore-nsa-band", controller.setNRBand(ctx, "NSA", state.Snapshot.NRBandNSA))
	if state.Snapshot.NetSelect != "" {
		record("restore-net-select", controller.setNetSelect(ctx, state.Snapshot.NetSelect))
	}
	if isActiveCellLock(state.Snapshot) {
		record("restore-cell-lock", controller.setNRCellLock(ctx,
			state.Snapshot.NRCellLockPCI, state.Snapshot.NRCellLockARFCN, state.Snapshot.NRCellLockBand))
	}
	if len(restoreErrors) != 0 {
		state.Status = "restore-failed"
		state.Stage = "restore-incomplete"
		state.LastError = errors.Join(restoreErrors...).Error()
		_ = controller.writeState(state)
		return errors.Join(restoreErrors...)
	}
	state.Status = "restoring"
	state.Stage = "network-restored"
	state.LastError = ""
	return controller.writeState(state)
}

func isActiveCellLock(snapshot networkSnapshot) bool {
	for _, value := range []string{snapshot.NRCellLockPCI, snapshot.NRCellLockARFCN, snapshot.NRCellLockBand} {
		if parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64); err == nil && parsed > 0 {
			return true
		}
	}
	return false
}

func loadRecoveryState(path string) (*recoveryState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &recoveryState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	if state.Schema != stateSchema || state.Snapshot.Raw == nil {
		return nil, fmt.Errorf("unsupported or incomplete recovery state")
	}
	return state, nil
}

func pendingRecovery(path string) (*recoveryState, bool, error) {
	state, err := loadRecoveryState(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return state, true, nil
}

func runRecoverCommand(args []string) error {
	flags := newFlagSet("recover")
	statePath := flags.String("state", defaultStatePath, "recovery journal path")
	ubus := flags.String("ubus", "ubus", "ubus executable")
	diagMDLog := flags.String("diag-mdlog", "", "optional diag_mdlog executable override")
	confirm := flags.Bool("confirm-network-change", false, "confirm restoring saved network settings")
	if err := flags.Parse(args); err != nil {
		return err
	}
	state, err := loadRecoveryState(*statePath)
	if err != nil {
		return err
	}
	if state.NetworkRestoreRequired && !*confirm {
		return errors.New("recover requires -confirm-network-change because this journal contains temporary network changes")
	}
	controller := &deviceController{UBUS: *ubus, StatePath: *statePath}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	traceErr := restoreProductionTraceFromState(ctx, state, *diagMDLog)
	var networkErr error
	if state.NetworkRestoreRequired {
		networkErr = controller.restoreNetwork(ctx, state)
	} else {
		state.Stage = "network-unchanged"
		networkErr = controller.writeState(state)
	}
	combined := errors.Join(traceErr, networkErr)
	if combined != nil {
		state.Status = "restore-failed"
		state.Stage = "restore-incomplete"
		state.LastError = combined.Error()
		_ = controller.writeState(state)
		return combined
	}
	state.Status = "restored"
	state.Stage = "complete"
	state.LastError = ""
	if err := controller.writeState(state); err != nil {
		return err
	}
	return os.Remove(*statePath)
}

func splitCellLock(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '/' || r == ' '
	})
}

func stringValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, mode)
}

func readMemAvailable() (uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kilobytes, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return kilobytes * 1024, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("MemAvailable not found")
}

func workFreeBytes(path string) (uint64, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return 0, err
	}
	return stats.Bavail * uint64(stats.Bsize), nil
}

func directoryBytes(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}
