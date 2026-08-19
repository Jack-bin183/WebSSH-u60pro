package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const reducerVersion = "0.2.2"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("missing command")
	}
	switch args[0] {
	case "dry-run":
		return runDryCommand(args[1:])
	case "inspect-qmdl":
		return runInspectQMDLCommand(args[1:])
	case "cleanup":
		return runCleanupCommand(args[1:])
	case "controls":
		return runControlsCommand(args[1:])
	case "private-probe":
		return runPrivateProbeCommand(args[1:])
	case "ddmin":
		return runDDMinCommand(args[1:])
	case "version", "--version", "-version":
		fmt.Println("u60-qtrace-reducer", reducerVersion)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `u60-qtrace-reducer %s

Commands:
  dry-run   Parse and validate qtrace.cfg; generate zero/control configs only.
  inspect-qmdl  Analyze existing QMDL files without touching the device.
  cleanup   Explicitly zero only the 23 original message-mask ranges.
  controls  Run the four state-aware control groups on a U60 device.
  private-probe  Run exactly one cold-state private-command quadrant.
  ddmin     Calibrate positive controls and run whole-frame ddmin.

Run "u60-qtrace-reducer <command> -h" for command flags.
`, reducerVersion)
}

func runCleanupCommand(args []string) error {
	flags := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	common := addCommonDeviceFlags(flags)
	if err := flags.Parse(args); err != nil {
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
	defer runner.release()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runner.cleanup(ctx, "manual"); err != nil {
		return err
	}
	fmt.Println("explicit zero-mask cleanup completed for all 23 original ranges")
	return nil
}

func runInspectQMDLCommand(args []string) error {
	flags := flag.NewFlagSet("inspect-qmdl", flag.ContinueOnError)
	targetValue := flags.String("target", "combined", "nr, lte, or combined")
	timeout := flags.Duration("timeout", 30*time.Second, "analysis timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("inspect-qmdl requires at least one .qmdl path")
	}
	target, err := parseTarget(*targetValue)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	metrics, err := analyzeQMDLFiles(ctx, flags.Args(), nil, target)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(metrics, "", "  ")
	fmt.Println(string(data))
	return nil
}

func runDryCommand(args []string) error {
	flags := flag.NewFlagSet("dry-run", flag.ContinueOnError)
	configPath := flags.String("config", "qtrace.cfg", "source qtrace.cfg")
	outputDir := flags.String("out", "./u60-qtrace-dry-run", "output directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	manifest, err := writeDryRun(*configPath, *outputDir)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	fmt.Println(string(data))
	return nil
}

type commonDeviceFlags struct {
	configPath *string
	workDir    *string
	diagMDLog  *string
	confirm    *bool
	settle     *time.Duration
	drain      *time.Duration
	poll       *time.Duration
	stopGrace  *time.Duration
	keep       *bool
	fileSize   *int
	fileCount  *int
}

func addCommonDeviceFlags(flags *flag.FlagSet) commonDeviceFlags {
	return commonDeviceFlags{
		configPath: flags.String("config", "qtrace.cfg", "source qtrace.cfg"),
		workDir:    flags.String("work-dir", "/tmp/u60-qtrace-reducer", "device work/report directory"),
		diagMDLog:  flags.String("diag-mdlog", "diag_mdlog", "diag_mdlog executable"),
		confirm:    flags.Bool("confirm-device", false, "confirm this command may change runtime DIAG masks"),
		settle:     flags.Duration("settle", time.Second, "wait after applying candidate before measuring"),
		drain:      flags.Duration("drain", 1500*time.Millisecond, "zero-mask drain interval before/after every run"),
		poll:       flags.Duration("poll", 250*time.Millisecond, "QMDL polling interval"),
		stopGrace:  flags.Duration("stop-grace", 2*time.Second, "SIGTERM grace period for diag_mdlog"),
		keep:       flags.Bool("keep-captures", false, "retain QMDL captures instead of only structured metrics"),
		fileSize:   flags.Int("file-size-mb", 4, "diag_mdlog rotation size"),
		fileCount:  flags.Int("file-count", 32, "diag_mdlog rotation count"),
	}
}

func (common commonDeviceFlags) makeRunner() (*deviceOptions, error) {
	if !*common.confirm {
		return nil, errors.New("refusing device execution without --confirm-device")
	}
	if *common.settle < 0 || *common.drain <= 0 || *common.poll <= 0 || *common.stopGrace <= 0 {
		return nil, errors.New("settle must be non-negative; drain, poll, and stop-grace must be positive")
	}
	if *common.fileSize <= 0 || *common.fileCount <= 0 {
		return nil, errors.New("file-size-mb and file-count must be positive")
	}
	config, _, err := parseQTraceConfig(*common.configPath)
	if err != nil {
		return nil, err
	}
	reports, err := newReportWriter(filepath.Join(*common.workDir, "reports"))
	if err != nil {
		return nil, err
	}
	return &deviceOptions{
		Config: config, WorkDir: *common.workDir, DiagMDLog: *common.diagMDLog,
		Settle: *common.settle, Drain: *common.drain, Poll: *common.poll,
		StopGrace: *common.stopGrace, KeepCaptures: *common.keep,
		FileSizeMB: *common.fileSize, FileCount: *common.fileCount, Reports: reports,
	}, nil
}

func runControlsCommand(args []string) error {
	flags := flag.NewFlagSet("controls", flag.ContinueOnError)
	common := addCommonDeviceFlags(flags)
	targetValue := flags.String("target", "combined", "nr, lte, or combined")
	window := flags.Duration("window", 10*time.Second, "fixed control capture window")
	repeats := flags.Int("repeats", 3, "runs per control group")
	minimumHits := flags.Int("positive-min-hits", 0, "required positive hits (default: all repeats)")
	cold := flags.Bool("confirm-cold-private-state", false, "confirm modem/device was restarted after the last private-command experiment")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *repeats <= 0 || *window <= 0 || *minimumHits < 0 || *minimumHits > *repeats {
		return errors.New("repeats/window must be positive and positive-min-hits must be within 0..repeats")
	}
	if !*cold {
		return errors.New("no-private controls require a modem/device restart and --confirm-cold-private-state")
	}
	target, err := parseTarget(*targetValue)
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
	summary, err := (controlOptions{
		Runner: runner, Target: target, Repeats: *repeats, MinimumPositiveHit: *minimumHits,
		ColdStateConfirmed: *cold, Window: *window,
	}).run(ctx)
	if summary != nil {
		_ = runner.Reports.WriteSummary(fmt.Sprintf("controls-%s-partial.json", target), summary)
	}
	return err
}

func runDDMinCommand(args []string) error {
	flags := flag.NewFlagSet("ddmin", flag.ContinueOnError)
	common := addCommonDeviceFlags(flags)
	targetValue := flags.String("target", "all", "nr, lte, combined, or all")
	controlsReport := flags.String("controls-report", "", "completed cold-state controls summary JSON (required)")
	calibrationRuns := flags.Int("calibration-runs", 10, "positive-control runs used for P95")
	positiveMin := flags.Int("positive-min-hits", 0, "required calibration hits (default: all runs)")
	candidateRepeats := flags.Int("candidate-repeats", 3, "runs for every ddmin candidate")
	candidateMin := flags.Int("candidate-min-hits", 2, "hits required for a ddmin candidate")
	minimumWindow := flags.Duration("minimum-window", 3*time.Second, "minimum candidate capture window")
	maximumWindow := flags.Duration("maximum-window", 15*time.Second, "calibration and maximum candidate window")
	windowMargin := flags.Duration("window-margin", 2*time.Second, "margin added to positive-control P95")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *calibrationRuns <= 0 || *candidateRepeats <= 0 || *candidateMin <= 0 || *candidateMin > *candidateRepeats {
		return errors.New("calibration-runs/candidate-repeats must be positive and candidate-min-hits must be within 1..candidate-repeats")
	}
	if *positiveMin < 0 || *positiveMin > *calibrationRuns {
		return errors.New("positive-min-hits must be within 0..calibration-runs")
	}
	if *minimumWindow <= 0 || *maximumWindow <= 0 || *minimumWindow > *maximumWindow || *windowMargin < 0 {
		return errors.New("capture windows must be positive, minimum-window <= maximum-window, and window-margin non-negative")
	}
	targets, err := parseTargetList(*targetValue)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if err := validateControlsReport(*controlsReport, target); err != nil {
			return err
		}
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
	for _, target := range targets {
		summary, runErr := (ddminOptions{
			Runner: runner, Target: target, CalibrationRuns: *calibrationRuns,
			ControlsReport:     *controlsReport,
			PositiveMinimumHit: *positiveMin, CandidateRepeats: *candidateRepeats,
			CandidateMinHits: *candidateMin, MinimumWindow: *minimumWindow,
			MaximumWindow: *maximumWindow, WindowMargin: *windowMargin,
		}).run(ctx)
		if summary != nil {
			_ = runner.Reports.WriteSummary(fmt.Sprintf("ddmin-%s-partial.json", target), summary)
		}
		if runErr != nil {
			return fmt.Errorf("%s ddmin: %w", target, runErr)
		}
	}
	return nil
}

func parseTargetList(value string) ([]targetKind, error) {
	if strings.EqualFold(strings.TrimSpace(value), "all") {
		return []targetKind{targetNR, targetLTE, targetCombined}, nil
	}
	target, err := parseTarget(value)
	if err != nil {
		return nil, err
	}
	return []targetKind{target}, nil
}

func emergencyRelease(runner *deviceOptions) {
	runner.stopActive()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), runner.Drain+runner.StopGrace+5*time.Second)
	_ = runner.cleanup(cleanupCtx, "process-exit")
	cancel()
	runner.release()
}
