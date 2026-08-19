package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type runRecord struct {
	RunID                 string            `json:"run_id"`
	Timestamp             time.Time         `json:"timestamp"`
	Stage                 string            `json:"stage"`
	ControlGroup          string            `json:"control_group,omitempty"`
	Target                targetKind        `json:"target"`
	Attempt               int               `json:"attempt"`
	CandidateFrameIDs     []int             `json:"candidate_frame_ids"`
	CandidateSSIDs        []uint16          `json:"candidate_ssids"`
	CandidateMaskBits     []int             `json:"candidate_mask_bits"`
	PrivateCommandState   string            `json:"private_command_state"`
	PositiveControlStatus bool              `json:"positive_control_status"`
	CaptureDurationMS     int64             `json:"capture_duration_ms"`
	FirstHitLatencyMS     *int64            `json:"first_hit_latency_ms,omitempty"`
	FirstHitIsUpperBound  bool              `json:"first_hit_is_upper_bound,omitempty"`
	QSHFrameCount         int               `json:"qsh_frame_count"`
	QSHTotalBytes         int64             `json:"qsh_total_bytes"`
	TargetHashCount       int               `json:"target_hash_count"`
	ParseSuccessCount     int               `json:"parse_success_count"`
	ParseErrorCount       int               `json:"parse_error_count"`
	NRCellCount           int               `json:"nr_cell_count"`
	LTECellCount          int               `json:"lte_cell_count"`
	CapturedBytes         int64             `json:"captured_bytes"`
	MalformedFrames       int               `json:"malformed_frames"`
	Result                bool              `json:"result"`
	FailureReason         string            `json:"failure_reason,omitempty"`
	Cells                 []parsedCell      `json:"cells,omitempty"`
	CaptureDirectory      string            `json:"capture_directory,omitempty"`
	DiagExit              string            `json:"diag_exit,omitempty"`
	MaskApplyReference    string            `json:"mask_apply_reference"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

type reportWriter struct {
	mu      sync.Mutex
	dir     string
	jsonl   *os.File
	csvFile *os.File
	csv     *csv.Writer
	serial  int
}

func newReportWriter(dir string) (*reportWriter, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	jsonl, err := os.OpenFile(filepath.Join(dir, "runs.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	csvPath := filepath.Join(dir, "runs.csv")
	info, statErr := os.Stat(csvPath)
	writeHeader := statErr != nil || info.Size() == 0
	csvFile, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		_ = jsonl.Close()
		return nil, err
	}
	writer := &reportWriter{dir: dir, jsonl: jsonl, csvFile: csvFile, csv: csv.NewWriter(csvFile)}
	if writeHeader {
		if err := writer.csv.Write(runCSVHeader()); err != nil {
			writer.Close()
			return nil, err
		}
		writer.csv.Flush()
	}
	return writer, nil
}

func (writer *reportWriter) nextID(prefix string) string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.serial++
	return fmt.Sprintf("%s-%s-%03d", time.Now().UTC().Format("20060102T150405.000Z"), prefix, writer.serial)
}

func (writer *reportWriter) Write(record runRecord) error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := writer.jsonl.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := writer.jsonl.Sync(); err != nil {
		return err
	}
	if err := writer.csv.Write(record.csvRow()); err != nil {
		return err
	}
	writer.csv.Flush()
	if err := writer.csv.Error(); err != nil {
		return err
	}
	return writer.csvFile.Sync()
}

func (writer *reportWriter) WriteSummary(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(writer.dir, name), append(data, '\n'), 0600)
}

func (writer *reportWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.csv != nil {
		writer.csv.Flush()
	}
	var first error
	if writer.csvFile != nil {
		first = writer.csvFile.Close()
	}
	if writer.jsonl != nil {
		if err := writer.jsonl.Close(); first == nil {
			first = err
		}
	}
	return first
}

func runCSVHeader() []string {
	return []string{
		"run_id", "timestamp", "stage", "control_group", "target", "attempt",
		"candidate_frame_ids", "candidate_ssids", "candidate_mask_bits", "private_command_state",
		"positive_control_status", "capture_duration_ms", "first_hit_latency_ms", "first_hit_is_upper_bound",
		"qsh_frame_count", "qsh_total_bytes", "target_hash_count", "parse_success_count", "parse_error_count",
		"nr_cell_count", "lte_cell_count", "captured_bytes", "malformed_frames", "result", "failure_reason",
	}
}

func (record runRecord) csvRow() []string {
	latency := ""
	if record.FirstHitLatencyMS != nil {
		latency = strconv.FormatInt(*record.FirstHitLatencyMS, 10)
	}
	return []string{
		record.RunID, record.Timestamp.Format(time.RFC3339Nano), record.Stage, record.ControlGroup,
		string(record.Target), strconv.Itoa(record.Attempt), joinInts(record.CandidateFrameIDs),
		joinUint16s(record.CandidateSSIDs), joinInts(record.CandidateMaskBits), record.PrivateCommandState,
		strconv.FormatBool(record.PositiveControlStatus), strconv.FormatInt(record.CaptureDurationMS, 10), latency,
		strconv.FormatBool(record.FirstHitIsUpperBound), strconv.Itoa(record.QSHFrameCount),
		strconv.FormatInt(record.QSHTotalBytes, 10), strconv.Itoa(record.TargetHashCount),
		strconv.Itoa(record.ParseSuccessCount), strconv.Itoa(record.ParseErrorCount), strconv.Itoa(record.NRCellCount),
		strconv.Itoa(record.LTECellCount), strconv.FormatInt(record.CapturedBytes, 10), strconv.Itoa(record.MalformedFrames),
		strconv.FormatBool(record.Result), record.FailureReason,
	}
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return strings.Join(parts, ";")
}

func joinUint16s(values []uint16) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatUint(uint64(value), 10)
	}
	return strings.Join(parts, ";")
}
