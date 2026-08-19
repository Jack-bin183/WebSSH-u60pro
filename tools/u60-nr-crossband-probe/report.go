package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func appendExperimentReports(workDir string, record experimentRecord) error {
	reportDir := filepath.Join(workDir, "reports")
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		return err
	}
	jsonPath := filepath.Join(reportDir, "experiments.jsonl")
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	jsonFile, err := os.OpenFile(jsonPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	_, writeErr := jsonFile.Write(append(data, '\n'))
	closeErr := jsonFile.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	if err := appendCSVReport(filepath.Join(reportDir, "experiments.csv"), record); err != nil {
		return err
	}
	prettyPath := filepath.Join(reportDir, record.RunID+".json")
	if err := writeJSONAtomic(prettyPath, record, 0600); err != nil {
		return err
	}
	output, _ := json.MarshalIndent(record, "", "  ")
	fmt.Println(string(output))
	return nil
}

func appendCSVReport(path string, record experimentRecord) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	if info.Size() == 0 {
		if err := writer.Write([]string{
			"run_id", "experiment", "started_at", "serving_band", "serving_arfcn", "serving_pci",
			"target_band", "target_arfcn", "target_pci", "qmi_event_count", "qsh_frame_count",
			"qmi_trace_complete", "qmi_trace_error",
			"active_hash_count", "valid_target_count", "first_target_hit_ms", "capture_duration_ms",
			"qmdl_bytes", "minimum_mem_available_bytes", "network_interrupted", "restore_succeeded",
			"result", "failure_reason",
		}); err != nil {
			return err
		}
	}
	targetPCI := ""
	if record.Target.PCI != nil {
		targetPCI = strconv.FormatUint(uint64(*record.Target.PCI), 10)
	}
	firstHit := ""
	if record.FirstTargetHitMS != nil {
		firstHit = strconv.FormatInt(*record.FirstTargetHitMS, 10)
	}
	minimumMemory := uint64(0)
	for _, sample := range record.ResourceSamples {
		if minimumMemory == 0 || sample.MemAvailableBytes < minimumMemory {
			minimumMemory = sample.MemAvailableBytes
		}
	}
	row := []string{
		record.RunID, string(record.Experiment), record.StartedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		record.ServingBefore.ServingBand, record.ServingBefore.ServingARFCN, record.ServingBefore.ServingPCI,
		strconv.FormatUint(uint64(record.Target.Band), 10), strconv.FormatUint(uint64(record.Target.ARFCN), 10), targetPCI,
		strconv.Itoa(len(record.QMIEvents)), strconv.Itoa(record.QSHFrameCount),
		strconv.FormatBool(record.QMITraceComplete), strings.ReplaceAll(record.QMITraceError, "\n", " "),
		strconv.Itoa(record.ActiveHashCount),
		strconv.Itoa(countTargetResults(record.ML1Results, record.Target)), firstHit, strconv.FormatInt(record.CaptureDurationMS, 10),
		strconv.FormatInt(record.QMDLBytes, 10), strconv.FormatUint(minimumMemory, 10),
		strconv.FormatBool(record.NetworkInterrupted), strconv.FormatBool(record.RestoreSucceeded),
		record.Result, strings.ReplaceAll(record.FailureReason, "\n", " "),
	}
	if err := writer.Write(row); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func countTargetResults(results []ml1Result, target targetSpec) int {
	count := 0
	for _, result := range results {
		if matchesTargetResult(result, target) {
			count++
		}
	}
	return count
}
