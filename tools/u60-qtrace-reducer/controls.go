package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type controlGroupResult struct {
	Name            string      `json:"name"`
	ExpectedMatch   bool        `json:"expected_match"`
	ObservedHits    int         `json:"observed_hits"`
	ObservedNRRuns  int         `json:"observed_nr_runs"`
	ObservedLTERuns int         `json:"observed_lte_runs"`
	UnexpectedRuns  int         `json:"unexpected_runs"`
	Runs            int         `json:"runs"`
	ExpectationMet  bool        `json:"expectation_met"`
	Records         []runRecord `json:"records"`
}

func validateControlsReport(path string, target targetKind) error {
	if path == "" {
		return errors.New("ddmin requires -controls-report from a completed cold-state control run")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read controls report: %w", err)
	}
	var report controlsSummary
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("decode controls report: %w", err)
	}
	if !report.ColdStateConfirmed || !report.PositiveControlStatus || report.Completed.IsZero() {
		return errors.New("controls report is incomplete, not cold-state confirmed, or has failed positive control")
	}
	if (target == targetCombined && report.Target != targetCombined) ||
		(target == targetNR && report.Target != targetNR && report.Target != targetCombined) ||
		(target == targetLTE && report.Target != targetLTE && report.Target != targetCombined) {
		return fmt.Errorf("controls target %s does not cover ddmin target %s", report.Target, target)
	}
	want := map[string]bool{"all-off": false, "message-only": false, "positive": false, "zero-mask-private": false, "positive-end": false}
	for _, group := range report.Groups {
		if _, ok := want[group.Name]; ok {
			want[group.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			return fmt.Errorf("controls report is missing group %s", name)
		}
	}
	return nil
}

type controlsSummary struct {
	Target                targetKind           `json:"target"`
	Started               time.Time            `json:"started"`
	Completed             time.Time            `json:"completed"`
	ColdStateConfirmed    bool                 `json:"cold_state_confirmed"`
	ExecutionOrder        []string             `json:"execution_order"`
	Groups                []controlGroupResult `json:"groups"`
	PositiveControlStatus bool                 `json:"positive_control_status"`
	PrivateStateAtExit    string               `json:"private_state_at_exit"`
	StateWarning          string               `json:"state_warning"`
}

type controlOptions struct {
	Runner             *deviceOptions
	Target             targetKind
	Repeats            int
	MinimumPositiveHit int
	ColdStateConfirmed bool
	Window             time.Duration
}

func (options controlOptions) run(ctx context.Context) (*controlsSummary, error) {
	if options.Runner == nil {
		return nil, errors.New("control runner is nil")
	}
	if !options.ColdStateConfirmed {
		return nil, errors.New("no-private controls require a known cold private-command state; restart the modem/device, then pass --confirm-cold-private-state")
	}
	if options.Repeats <= 0 {
		options.Repeats = 3
	}
	if options.MinimumPositiveHit <= 0 {
		options.MinimumPositiveHit = options.Repeats
	}
	allIDs := options.Runner.Config.allMessageFrameIDs()
	summary := &controlsSummary{
		Target: options.Target, Started: time.Now().UTC(), ColdStateConfirmed: true,
		ExecutionOrder:     []string{"all-off", "message-only", "positive", "zero-mask-private", "positive-end"},
		PrivateStateAtExit: "both private commands sent; persistence/disable semantics unknown",
		StateWarning:       "The tool explicitly zeroes only the original 23 message-mask ranges. It does not claim to disable either private command.",
	}

	// The two no-private groups must precede the first positive control. Running
	// positive first would itself contaminate the cold-state experiment.
	allOff, err := options.runGroup(ctx, "all-off", false, allIDs, true, privateNone, options.Repeats, false)
	if err != nil {
		return summary, err
	}
	summary.Groups = append(summary.Groups, allOff)
	messageOnly, err := options.runGroup(ctx, "message-only", false, allIDs, false, privateNone, options.Repeats, false)
	if err != nil {
		return summary, err
	}
	summary.Groups = append(summary.Groups, messageOnly)
	positive, err := options.runGroup(ctx, "positive", true, allIDs, false, privateBoth, options.Repeats, false)
	if err != nil {
		return summary, err
	}
	summary.Groups = append(summary.Groups, positive)
	if positive.ObservedHits < options.MinimumPositiveHit {
		return summary, fmt.Errorf("positive control hit %d/%d; need %d, so the control batch is invalid", positive.ObservedHits, positive.Runs, options.MinimumPositiveHit)
	}
	summary.PositiveControlStatus = true
	zeroPrivate, err := options.runGroup(ctx, "zero-mask-private", false, allIDs, true, privateBoth, options.Repeats, true)
	if err != nil {
		return summary, err
	}
	summary.Groups = append(summary.Groups, zeroPrivate)
	positiveEnd, err := options.runGroup(ctx, "positive-end", true, allIDs, false, privateBoth, options.Repeats, true)
	if err != nil {
		return summary, err
	}
	summary.Groups = append(summary.Groups, positiveEnd)
	if positiveEnd.ObservedHits < options.MinimumPositiveHit {
		summary.PositiveControlStatus = false
		return summary, fmt.Errorf("batch-end positive control hit %d/%d; control conclusions are invalid", positiveEnd.ObservedHits, positiveEnd.Runs)
	}
	summary.Completed = time.Now().UTC()
	if err := options.Runner.Reports.WriteSummary(fmt.Sprintf("controls-%s-summary.json", options.Target), summary); err != nil {
		return summary, err
	}
	return summary, nil
}

func (options controlOptions) runGroup(
	ctx context.Context,
	name string,
	expectedMatch bool,
	frameIDs []int,
	zeroMasks bool,
	private privateMode,
	runs int,
	positiveOK bool,
) (controlGroupResult, error) {
	result := controlGroupResult{Name: name, ExpectedMatch: expectedMatch, Runs: runs}
	for attempt := 1; attempt <= runs; attempt++ {
		record, err := options.Runner.testCandidate(
			ctx, "control", name, options.Target, frameIDs, zeroMasks, private,
			string(private), attempt, positiveOK, options.Window,
		)
		result.Records = append(result.Records, record)
		if record.Result {
			result.ObservedHits++
		}
		if record.NRCellCount > 0 {
			result.ObservedNRRuns++
		}
		if record.LTECellCount > 0 {
			result.ObservedLTERuns++
		}
		if !expectedMatch && controlHasUnexpectedTarget(record, options.Target) {
			result.UnexpectedRuns++
		}
		if err != nil {
			return result, err
		}
	}
	if expectedMatch {
		result.ExpectationMet = result.ObservedHits == runs
	} else {
		result.ExpectationMet = result.UnexpectedRuns == 0
	}
	return result, nil
}

func controlHasUnexpectedTarget(record runRecord, target targetKind) bool {
	switch target {
	case targetNR:
		return record.NRCellCount > 0
	case targetLTE:
		return record.LTECellCount > 0
	case targetCombined:
		// A combined negative control is intentionally stricter than the
		// combined positive predicate: neither RAT may leak through.
		return record.NRCellCount > 0 || record.LTECellCount > 0
	default:
		return false
	}
}
