package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "dry-run":
		return runDryRun(args[1:])
	case "run":
		return runExperimentCommand(args[1:])
	case "recover":
		return runRecoverCommand(args[1:])
	case "inspect-qmdl":
		return runInspectQMDL(args[1:])
	case "diff-qmi":
		return runDiffQMI(args[1:])
	case "version", "--version", "-version":
		fmt.Printf("%s %s\n", toolName, toolVersion)
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: u60-nr-crossband-probe <dry-run|run|recover|inspect-qmdl|diff-qmi|version> [options]")
}

func targetFlags(flags *flag.FlagSet) (*uint, *uint, *string) {
	band := flags.Uint("target-band", 41, "target NR band number")
	arfcn := flags.Uint("target-arfcn", 504990, "target NR SSB ARFCN observed from ML1")
	pci := flags.String("target-pci", "", "known target PCI; required by experiment F")
	return band, arfcn, pci
}

func parseTarget(bandValue, arfcnValue uint, pciValue string) (targetSpec, error) {
	if bandValue == 0 || bandValue > 1024 {
		return targetSpec{}, fmt.Errorf("invalid target band n%d", bandValue)
	}
	if arfcnValue == 0 || arfcnValue > 3279165 {
		return targetSpec{}, fmt.Errorf("invalid target NR-ARFCN %d", arfcnValue)
	}
	target := targetSpec{Band: uint32(bandValue), ARFCN: uint32(arfcnValue)}
	if strings.TrimSpace(pciValue) != "" {
		value, err := strconv.ParseUint(strings.TrimSpace(pciValue), 10, 32)
		if err != nil || value > 1007 {
			return targetSpec{}, fmt.Errorf("invalid target PCI %q", pciValue)
		}
		pci := uint32(value)
		target.PCI = &pci
	}
	return target, nil
}

func runDryRun(args []string) error {
	flags := flag.NewFlagSet("dry-run", flag.ContinueOnError)
	configPath := flags.String("config", "qtrace.cfg", "production minimal qtrace.cfg")
	outputDir := flags.String("out", "/tmp/u60-nr-crossband-dry-run", "output directory")
	band, arfcn, pci := targetFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	target, err := parseTarget(*band, *arfcn, *pci)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outputDir, 0700); err != nil {
		return err
	}
	activePath := filepath.Join(*outputDir, "qtrace-active-id96-bit15.cfg")
	patchResult, err := buildActiveQTrace(*configPath, activePath)
	if err != nil {
		return err
	}
	manual, err := makeNRIncrementalScanRequest(1, []uint32{target.ARFCN}, 2)
	if err != nil {
		return err
	}
	oneshoot, err := makeQCRILOneShotExpectedRequest(1, 0x10)
	if err != nil {
		return err
	}
	manifest := dryRunManifest{
		ToolVersion: toolVersion, SourceConfig: *configPath,
		SourceSHA256: patchResult.SourceSHA256, SourceBytes: patchResult.SourceBytes,
		ActiveConfig: activePath, ActiveSHA256: patchResult.ActiveSHA256, ActiveBytes: patchResult.ActiveBytes,
		QSHID: 96, OriginalMask: fmt.Sprintf("0x%04x", patchResult.OriginalMask),
		ActiveMask: fmt.Sprintf("0x%04x", patchResult.ActiveMask), RoundTripValid: true,
		Target: target, Manual0085Raw: fmt.Sprintf("%x", manual),
		QCRILOneShotExpectedRaw:    fmt.Sprintf("%x", oneshoot),
		QCRILOneShotCarriesChannel: false, QCRILAdvancedCarriesChannel: true,
		SetChannelsQMIMessage:     "NAS 0x0033 Set System Selection Preference",
		SetChannelsCarriesChannel: false, Experiments: defaultExperimentPlans(),
	}
	manifestPath := filepath.Join(*outputDir, "manifest.json")
	if err := writeJSONAtomic(manifestPath, manifest, 0600); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	fmt.Println(string(data))
	return nil
}

func runInspectQMDL(args []string) error {
	flags := flag.NewFlagSet("inspect-qmdl", flag.ContinueOnError)
	band, arfcn, pci := targetFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("inspect-qmdl requires one or more .qmdl paths")
	}
	target, err := parseTarget(*band, *arfcn, *pci)
	if err != nil {
		return err
	}
	metrics, err := analyzeQMDLFiles(flags.Args(), target)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(metrics, "", "  ")
	fmt.Println(string(data))
	return nil
}

func defaultExperimentPlans() []experimentPlan {
	return []experimentPlan{
		{ID: experimentA, ControlPath: "raw NAS 0x0085 advanced request with NGRAN + one ARFCN", SuccessCondition: "ML1 result hash 0xd8f582a8 reports the target ARFCN"},
		{ID: experimentB, ControlPath: "complete QCRIL startNetworkScan through an external socket/HAL driver", RequiresQCRILDriver: true, SuccessCondition: "captured QCRIL QMI differs materially from A and ML1 searches the target", Limitation: "reverse engineering shows ONE_SHOT drops Band/channel before NAS 0x0085"},
		{ID: experimentC, ControlPath: "temporarily restrict NR to the target Band; do not explicitly force acquisition", NetworkDisruption: true, SuccessCondition: "ML1 searches the target ARFCN without an explicit acquisition restart", Limitation: "U60 QCRIL setSystemSelectionChannels maps Band only; channel is ignored; the vendor Band setter may itself trigger reselection"},
		{ID: experimentD, ControlPath: "target Band restriction followed by automatic reselection", NetworkDisruption: true, SuccessCondition: "first valid target ML1 result after reselection"},
		{ID: experimentE, ControlPath: "target Band only, briefly remove NR service, then enable NR; stop at first hit", NetworkDisruption: true, SuccessCondition: "first valid target PCI/RSRP before full registration"},
		{ID: experimentF, ControlPath: "known PCI+ARFCN+Band cell lock and NR registration positive control", NetworkDisruption: true, SuccessCondition: "known target becomes serving or produces a valid ML1 result"},
	}
}

func parseExperiment(value string) (experimentID, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch experimentID(value) {
	case experimentA, experimentB, experimentC, experimentD, experimentE, experimentF:
		return experimentID(value), nil
	default:
		return "", fmt.Errorf("invalid experiment %q (want A, B, C, D, E, or F)", value)
	}
}

func nowRunID(experiment experimentID) string {
	return fmt.Sprintf("%s-%s", strings.ToLower(string(experiment)), time.Now().UTC().Format("20060102T150405.000000000Z"))
}
