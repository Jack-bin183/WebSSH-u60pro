package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type privateProbeSummary struct {
	Target             targetKind  `json:"target"`
	Mode               privateMode `json:"mode"`
	Label              string      `json:"label"`
	Started            time.Time   `json:"started"`
	Completed          time.Time   `json:"completed"`
	ColdStateConfirmed bool        `json:"cold_state_confirmed"`
	PriorPrivateState  string      `json:"prior_private_state,omitempty"`
	Record             runRecord   `json:"record"`
	PrivateStateAtExit string      `json:"private_state_at_exit"`
	StateWarning       string      `json:"state_warning"`
}

func parsePrivateProbeMode(value string) (privateMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "both":
		return privateBoth, nil
	case "9001", "0x9001", "0x44/0x9001":
		return private9001, nil
	case "0004", "0x0004", "0x55/0x0004":
		return private0004, nil
	default:
		return "", fmt.Errorf("invalid private mode %q (want both, 9001, or 0004)", value)
	}
}

func runPrivateProbeCommand(args []string) error {
	flags := flag.NewFlagSet("private-probe", flag.ContinueOnError)
	common := addCommonDeviceFlags(flags)
	targetValue := flags.String("target", "nr", "nr, lte, or combined")
	modeValue := flags.String("mode", "", "both, 9001, or 0004")
	label := flags.String("label", "cold-private", "unique cold-boot/run label")
	window := flags.Duration("window", 10*time.Second, "capture window")
	cold := flags.Bool("confirm-cold-private-state", false, "confirm no private qtrace command has run since modem/device restart")
	prior := flags.String("prior-private-state", "", "known private command already sent in this same boot (sequential positive control)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	priorState := strings.TrimSpace(*prior)
	if !*cold && priorState == "" {
		return errors.New("private-probe requires either --confirm-cold-private-state or an explicit --prior-private-state")
	}
	if *cold && priorState != "" {
		return errors.New("--confirm-cold-private-state and --prior-private-state are mutually exclusive")
	}
	if *window <= 0 || strings.TrimSpace(*label) == "" {
		return errors.New("window must be positive and label must not be empty")
	}
	target, err := parseTarget(*targetValue)
	if err != nil {
		return err
	}
	mode, err := parsePrivateProbeMode(*modeValue)
	if err != nil {
		return err
	}
	runner, err := common.makeRunner()
	if err != nil {
		return err
	}
	defer runner.Reports.Close()
	if err := runner.acquire(); err != nil {
		return err
	}
	defer emergencyRelease(runner)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stage := "cold-private-probe"
	effectiveState := string(mode)
	warning := "Only the first candidate capture belongs to the confirmed cold private-command state. Repeating another quadrant requires another modem/device restart."
	if priorState != "" {
		stage = "sequential-private-control"
		effectiveState = fmt.Sprintf("prior %s + sent %s", priorState, mode)
		warning = "This is a same-boot sequential positive control, not an independent cold-state quadrant."
	}
	summary := privateProbeSummary{
		Target: target, Mode: mode, Label: *label, Started: time.Now().UTC(), ColdStateConfirmed: *cold,
		PriorPrivateState:  priorState,
		PrivateStateAtExit: fmt.Sprintf("%s; persistence/disable semantics unknown", effectiveState),
		StateWarning:       warning,
	}
	record, runErr := runner.testCandidate(ctx, stage, *label, target, nil, false, mode, effectiveState, 1, false, *window)
	summary.Record = record
	summary.Completed = time.Now().UTC()
	name := fmt.Sprintf("private-probe-%s-summary.json", sanitizeName(*label))
	if err := runner.Reports.WriteSummary(name, summary); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(data))
	return runErr
}
