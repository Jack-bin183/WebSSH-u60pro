package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestQMDLRequiresCompleteNRIdentityAndSignal(t *testing.T) {
	data := joinTestQMDLFrames(
		buildTestQMDLFrame(0xd8fc0f04, 41, 504990, 936, 17),
		buildTestQMDLFrame(0xda01a364, 0, 0, 0, 936, -12851, 111, -13000, 0, 0, 0, 0, 0),
	)
	metrics := analyzeTestQMDL(t, data, targetNR, nil)
	if !metrics.matches(targetNR) || metrics.NRCellCount != 1 || metrics.TargetHashCount != 1 || metrics.ParseSuccessCount != 1 {
		t.Fatalf("unexpected NR metrics: %+v", metrics)
	}
	cell := metrics.Cells[0]
	if cell.PCI != 936 || cell.ARFCN != 504990 || cell.RSRP > -100 || cell.RSRP < -101 {
		t.Fatalf("unexpected NR cell: %+v", cell)
	}
}

func TestQMDLHashOnlyIsNotAHit(t *testing.T) {
	// The report hash and valid signal are present, but the candidate carrying
	// ARFCN is absent. A hash-only detector would incorrectly pass this case.
	data := buildTestQMDLFrame(0xda01a364, 0, 0, 0, 936, -12851, 111, -13000, 0, 0, 0, 0, 0)
	metrics := analyzeTestQMDL(t, data, targetNR, nil)
	if metrics.TargetHashCount != 1 || metrics.ParseSuccessCount != 1 {
		t.Fatalf("report should decode before correlation: %+v", metrics)
	}
	if metrics.matches(targetNR) || metrics.NRCellCount != 0 {
		t.Fatalf("hash without ARFCN must not pass: %+v", metrics)
	}
}

func TestQMDLRejectsInvalidSignalAndServingCell(t *testing.T) {
	invalid := joinTestQMDLFrames(
		buildTestQMDLFrame(0xd8fc0f04, 41, 504990, 936, 17),
		buildTestQMDLFrame(0xda01a364, 0, 0, 0, 936, 0x7fffffff, 111, -13000, 0, 0, 0, 0, 0),
	)
	metrics := analyzeTestQMDL(t, invalid, targetNR, nil)
	if metrics.matches(targetNR) || metrics.ParseErrorCount == 0 {
		t.Fatalf("invalid signal passed: %+v", metrics)
	}
	valid := joinTestQMDLFrames(
		buildTestQMDLFrame(0xd8fc0f04, 41, 504990, 936, 17),
		buildTestQMDLFrame(0xda01a364, 0, 0, 0, 936, -12851, 111, -13000, 0, 0, 0, 0, 0),
	)
	serving := []servingIdentity{{RAT: "NR", PCI: 936, ARFCN: 504990}}
	metrics = analyzeTestQMDL(t, valid, targetNR, serving)
	if metrics.matches(targetNR) {
		t.Fatalf("serving cell must not count as neighbor: %+v", metrics)
	}
}

func TestQMDLRejectsDefaultLTEQualityWhenLayoutDefinesRSRQ(t *testing.T) {
	data := buildTestQMDLFrame(0xd8fe54a0, 1300, 41, -943, 0)
	metrics := analyzeTestQMDL(t, data, targetLTE, nil)
	if metrics.matches(targetLTE) || metrics.ParseErrorCount == 0 {
		t.Fatalf("default LTE RSRQ passed: %+v", metrics)
	}
}

func TestQMDLCombinedRequiresBothRATs(t *testing.T) {
	nr := joinTestQMDLFrames(
		buildTestQMDLFrame(0xd8fc0f04, 41, 504990, 936, 17),
		buildTestQMDLFrame(0xda01a364, 0, 0, 0, 936, -12851, 111, -13000, 0, 0, 0, 0, 0),
	)
	metrics := analyzeTestQMDL(t, nr, targetCombined, nil)
	if metrics.matches(targetCombined) {
		t.Fatal("NR alone passed combined target")
	}
	lte := buildTestQMDLFrame(0xd8fe54a0, 1300, 41, -943, -120)
	metrics = analyzeTestQMDL(t, append(nr, lte...), targetCombined, nil)
	if !metrics.matches(targetCombined) || metrics.NRCellCount != 1 || metrics.LTECellCount != 1 {
		t.Fatalf("combined target did not pass: %+v", metrics)
	}
}

func TestQMDLWindowExcludesSettleBytesAndPartialFrame(t *testing.T) {
	oldFrame := buildTestQMDLFrame(0xd8fe54a0, 1300, 41, -943, -120)
	newFrame := buildTestQMDLFrame(0xd8fe54a0, 1300, 42, -955, -125)
	dir := t.TempDir()
	path := filepath.Join(dir, "diag_log_window.qmdl")
	data := append(append([]byte(nil), oldFrame...), newFrame...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	metrics, err := analyzeQMDLWindow(context.Background(), []string{path}, map[string]int64{path: int64(len(oldFrame))}, nil, targetLTE)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.LTECellCount != 1 || len(metrics.Cells) != 1 || metrics.Cells[0].PCI != 42 {
		t.Fatalf("settle frame leaked into window: %+v", metrics)
	}
	// Starting in the middle of oldFrame must discard its remainder through the
	// first delimiter and still accept the next complete frame.
	metrics, err = analyzeQMDLWindow(context.Background(), []string{path}, map[string]int64{path: int64(len(oldFrame) / 2)}, nil, targetLTE)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.LTECellCount != 1 || metrics.Cells[0].PCI != 42 {
		t.Fatalf("partial pre-window frame was not discarded: %+v", metrics)
	}
}

func analyzeTestQMDL(t *testing.T, data []byte, target targetKind, serving []servingIdentity) captureMetrics {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "diag_log_test.qmdl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	metrics, err := analyzeQMDLFiles(context.Background(), []string{path}, serving, target)
	if err != nil {
		t.Fatal(err)
	}
	return metrics
}

func buildTestQMDLFrame(hash uint32, words ...int32) []byte {
	payload := make([]byte, (len(words)+4)*4)
	payload[0] = 0x9d
	payload[4] = byte(len(words) + 0x13)
	binary.LittleEndian.PutUint32(payload[12:16], hash)
	for index, word := range words {
		binary.LittleEndian.PutUint32(payload[16+index*4:], uint32(word))
	}
	crc := diagCRC16(payload)
	decoded := append(payload, byte(crc), byte(crc>>8))
	return append(hdlcEscape(decoded), 0x7e)
}

func joinTestQMDLFrames(frames ...[]byte) []byte {
	return bytes.Join(frames, nil)
}
