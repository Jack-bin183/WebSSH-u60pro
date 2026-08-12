package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"testing"
)

func TestQMDLParserMatchesNativeServingAndNRCorrelation(t *testing.T) {
	data := joinQMDLFrames(
		buildQMDLFrame(0xdcdda754, 0, 0, 0, 0, 504990),
		buildQMDLFrame(0xdcdda85c, 305419896, 208, -1009, -125, 83),
		buildQMDLFrame(0xd8fc0f04, 41, 504990, 936, 17),
		buildQMDLFrame(0xda01a364, 0, 0, 0, 936, -12851, 111, -13000, 0, 0, 0, 0, 0),
	)
	parser := &qmdlParser{}
	if err := parser.parseReader(context.Background(), bytes.NewReader(data), 0); err != nil {
		t.Fatal(err)
	}
	result := parser.result()
	if result.Engine != neighborGoParserEngine || result.Frames != 4 || result.Malformed != 0 {
		t.Fatalf("unexpected parser metadata: %+v", result)
	}
	if len(result.Serving) != 1 {
		t.Fatalf("serving count = %d; want 1", len(result.Serving))
	}
	serving := result.Serving[0]
	if serving.Seq != 1 || serving.GCI != 305419896 || serving.PCI != 208 || serving.ARFCN == nil || *serving.ARFCN != 504990 {
		t.Fatalf("unexpected serving cell: %+v", serving)
	}
	assertFloatPointer(t, "serving RSRP", serving.RSRPDBM, -100.9)
	assertFloatPointer(t, "serving RSRQ", serving.RSRQDB, -12.5)
	assertFloatPointer(t, "serving SINR", serving.SINRDB, 8.3)

	if len(result.Neighbors) != 1 {
		t.Fatalf("neighbor count = %d; want 1: %+v", len(result.Neighbors), result.Neighbors)
	}
	neighbor := result.Neighbors[0]
	if neighbor.RAT != "NR" || neighbor.PCI != 936 || neighbor.ARFCN == nil || *neighbor.ARFCN != 504990 || neighbor.Band == nil || *neighbor.Band != 41 {
		t.Fatalf("unexpected NR identity: %+v", neighbor)
	}
	if neighbor.Samples != 1 || neighbor.DirectHits != 1 || neighbor.FirstSeq != 2 || neighbor.LastSeq != 3 || neighbor.PlausibleSamples != 1 {
		t.Fatalf("unexpected NR counters: %+v", neighbor)
	}
	assertFloatPointer(t, "neighbor median", neighbor.RSRPMedian, -100.4)
}

func TestQMDLParserLTEFormatsAndMedian(t *testing.T) {
	data := joinQMDLFrames(
		buildQMDLFrame(0xd8fe54a0, 1650, 101, -1009, 0),
		buildQMDLFrame(0xd8fe54a0, 1650, 101, -100, 0),
		buildQMDLFrame(0xd936846c, 1650, 101),
	)
	parser := &qmdlParser{}
	if err := parser.parseReader(context.Background(), bytes.NewReader(data), 0); err != nil {
		t.Fatal(err)
	}
	result := parser.result()
	if len(result.Neighbors) != 1 {
		t.Fatalf("neighbor count = %d; want 1", len(result.Neighbors))
	}
	neighbor := result.Neighbors[0]
	if neighbor.RAT != "LTE" || neighbor.PCI != 101 || neighbor.ARFCN == nil || *neighbor.ARFCN != 1650 {
		t.Fatalf("unexpected LTE identity: %+v", neighbor)
	}
	if neighbor.Samples != 3 || neighbor.DirectHits != 0 || neighbor.PlausibleSamples != 2 {
		t.Fatalf("unexpected LTE counters: %+v", neighbor)
	}
	assertFloatPointer(t, "LTE median", neighbor.RSRPMedian, -100.45)
}

func TestQMDLParserMalformedAndEscapedFrames(t *testing.T) {
	valid := buildQMDLFrame(0xd8fe54a0, 1650, 126, -100, 0)
	if !bytes.Contains(valid, []byte{0x7d, 0x5e}) {
		t.Fatal("test frame must exercise HDLC escaping for PCI 126")
	}
	data := append(valid, 0x01, 0x7d, 0x7e)
	parser := &qmdlParser{}
	if err := parser.parseReader(context.Background(), bytes.NewReader(data), 0); err != nil {
		t.Fatal(err)
	}
	result := parser.result()
	if result.Frames != 1 || result.Malformed != 1 || len(result.Neighbors) != 1 {
		t.Fatalf("unexpected malformed-frame result: %+v", result)
	}
}

func TestDecodeQMDLCandidateSignatures(t *testing.T) {
	tests := []struct {
		id    uint32
		words []uint32
	}{
		{0xd8fc0f04, []uint32{41, 504990, 101, 0}},
		{0xda014184, []uint32{0, 0, 101, 0, 504990, 0}},
		{0xd9fca428, []uint32{0, 0, 101, 0, 504990, 0, 0}},
		{0xf93fc16a, []uint32{0, 0, 101, 0, 504990, 0, 0}},
		{0xd9fc7a44, []uint32{0, 101, 0, 504990, 0, 0}},
		{0xf93fc624, []uint32{0, 101, 0, 504990, 0, 0}},
		{0xf9ad1341, []uint32{504990, 101, 0, 0, 0}},
		{0xbcbafaec, []uint32{504990, 101, 0}},
		{0xd8facf74, []uint32{41, 504990, 101, 0}},
		{0xd8f773e8, []uint32{41, 504990, 101, 0}},
		{0xd8f72994, []uint32{101, 504990, 0, 0, 0}},
		{0xd9877e9c, []uint32{101, 504990, 0, 0, 0}},
		{0xd8fc5cc0, []uint32{41, 504990, 101, 0}},
		{0xd8fb8ad0, []uint32{41, 504990, 101, 0}},
		{0xd8f82d98, []uint32{41, 504990, 101, 0}},
		{0xd8f84514, []uint32{41, 504990, 101, 0}},
		{0xd8f7e394, []uint32{101, 504990, 0, 0}},
	}
	for _, test := range tests {
		candidate, ok := decodeQMDLCandidate(test.id, test.words)
		if !ok {
			t.Fatalf("signature %#x was not decoded", test.id)
		}
		if candidate.pci != 101 || candidate.arfcn != 504990 {
			t.Fatalf("signature %#x decoded as %+v", test.id, candidate)
		}
	}
	parser := &qmdlParser{candidates: []qmdlCandidate{{pci: 101, arfcn: 504990, band: 41}}}
	result := parser.result()
	if len(result.Neighbors) != 1 || result.Neighbors[0].Samples != 1 || result.Neighbors[0].DirectHits != 1 {
		t.Fatalf("candidate-only counters do not match native output: %+v", result.Neighbors)
	}
}

func buildQMDLFrame(logID uint32, words ...int32) []byte {
	payload := make([]byte, (len(words)+4)*4)
	payload[0] = 0x9d
	payload[4] = byte(len(words) + 0x13)
	binary.LittleEndian.PutUint32(payload[12:16], logID)
	for index, word := range words {
		binary.LittleEndian.PutUint32(payload[16+index*4:], uint32(word))
	}
	payload = append(payload, 0, 0)
	encoded := []byte{0x7e}
	for _, value := range payload {
		if value == 0x7d || value == 0x7e {
			encoded = append(encoded, 0x7d, value^0x20)
		} else {
			encoded = append(encoded, value)
		}
	}
	return append(encoded, 0x7e)
}

func joinQMDLFrames(frames ...[]byte) []byte {
	return bytes.Join(frames, nil)
}

func assertFloatPointer(t *testing.T, name string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil || math.Abs(*actual-expected) > 0.000001 {
		t.Fatalf("%s = %v; want %v", name, actual, expected)
	}
}
