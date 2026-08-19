package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

const (
	originalQTraceConfig   = "testdata/qtrace-original-25frames.cfg"
	productionQTraceConfig = "../../gossh/app/service/embed/qtrace.cfg"
)

func TestProductionConfigRoundTripAndZeroMasks(t *testing.T) {
	config, source, err := parseQTraceConfig(originalQTraceConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Frames) != 25 || len(config.MessageFrames) != 23 || len(config.PrivateFrames) != 2 {
		t.Fatalf("unexpected frame layout: total=%d message=%d private=%d", len(config.Frames), len(config.MessageFrames), len(config.PrivateFrames))
	}
	if got := len(config.allSSIDs(config.allMessageFrameIDs())); got != 466 {
		t.Fatalf("SSID count=%d, want 466", got)
	}
	for _, frame := range config.MessageFrames {
		for index, mask := range frame.Masks {
			if mask != 0xffffffff {
				t.Fatalf("frame %d SSID %d mask=%08x", frame.Frame.ID, int(frame.First)+index, mask)
			}
		}
	}
	positive, err := config.selectFrames(config.allMessageFrameIDs(), false, privateBoth)
	if err != nil {
		t.Fatal(err)
	}
	if encoded := encodeConfig(positive); !bytes.Equal(encoded, source) {
		sum := sha256.Sum256(encoded)
		t.Fatalf("round-trip differs: got sha256=%s want=%s", hex.EncodeToString(sum[:]), config.SourceSHA256)
	}
	zero, err := config.zeroMessageFrames()
	if err != nil {
		t.Fatal(err)
	}
	parsedZero, err := parseQTraceConfigBytes(encodeConfig(zero))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsedZero.MessageFrames) != 23 || len(parsedZero.PrivateFrames) != 0 {
		t.Fatalf("unexpected zero config layout: %+v", parsedZero)
	}
	for _, frame := range parsedZero.MessageFrames {
		if frame.First != config.MessageFrames[frame.Frame.ID].First || frame.Last != config.MessageFrames[frame.Frame.ID].Last {
			t.Fatalf("zero frame %d changed range", frame.Frame.ID)
		}
		for _, mask := range frame.Masks {
			if mask != 0 {
				t.Fatalf("zero frame %d contains mask %08x", frame.Frame.ID, mask)
			}
		}
	}
}

func TestCommittedNRMinimumConfig(t *testing.T) {
	assertCommitted9001MinimumConfig(t, "nr-neighbor-min.cfg", "NR")
}

func TestCommittedLTEMinimumConfig(t *testing.T) {
	assertCommitted9001MinimumConfig(t, "lte-neighbor-min.cfg", "LTE")
}

func TestCommittedCombinedMinimumConfig(t *testing.T) {
	assertCommitted9001MinimumConfig(t, "cell-neighbor-combined-min.cfg", "combined")
}

func TestProductionConfigMatchesCommittedCombinedMinimum(t *testing.T) {
	production, err := os.ReadFile(productionQTraceConfig)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := os.ReadFile(filepath.Join("configs", "cell-neighbor-combined-min.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(production, combined) {
		t.Fatal("production qtrace.cfg differs from the validated combined minimum")
	}
}

func assertCommitted9001MinimumConfig(t *testing.T, name, rat string) {
	t.Helper()
	path := filepath.Join("configs", name)
	config, source, err := parseQTraceConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(source) != 196 || len(config.MessageFrames) != 0 || len(config.PrivateFrames) != 1 {
		t.Fatalf("unexpected %s minimum layout: bytes=%d messages=%d private=%d", rat, len(source), len(config.MessageFrames), len(config.PrivateFrames))
	}
	payload := config.PrivateFrames[0].Decoded[:len(config.PrivateFrames[0].Decoded)-2]
	if payload[0] != 0x4b || payload[1] != 0x44 || binary.LittleEndian.Uint16(payload[2:4]) != 0x9001 {
		t.Fatalf("unexpected private command: %x", payload[:4])
	}
	sum := sha256.Sum256(source)
	if got := hex.EncodeToString(sum[:]); got != "15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58" {
		t.Fatalf("%s minimum SHA-256=%s", rat, got)
	}
}

func TestDryRunGeneratesFourValidatedControls(t *testing.T) {
	dir := t.TempDir()
	manifest, err := writeDryRun(originalQTraceConfig, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.RoundTripExact || !manifest.AllMasksFFFFFFFF || manifest.ContainsGlobalClear {
		t.Fatalf("unexpected manifest flags: %+v", manifest)
	}
	want := map[string]struct {
		messages int
		private  int
		zero     bool
	}{
		"positive.cfg":          {23, 2, false},
		"zero-mask-private.cfg": {23, 2, true},
		"message-only.cfg":      {23, 0, false},
		"all-off.cfg":           {23, 0, true},
		"private-both-only.cfg": {0, 2, false},
		"private-9001-only.cfg": {0, 1, false},
		"private-0004-only.cfg": {0, 1, false},
	}
	for name, expectation := range want {
		data, err := os.ReadFile(filepath.Join(dir, "controls", name))
		if err != nil {
			t.Fatal(err)
		}
		config, err := parseQTraceConfigBytes(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(config.MessageFrames) != expectation.messages || len(config.PrivateFrames) != expectation.private {
			t.Fatalf("%s layout: message=%d private=%d", name, len(config.MessageFrames), len(config.PrivateFrames))
		}
		for _, frame := range config.MessageFrames {
			for _, mask := range frame.Masks {
				if expectation.zero && mask != 0 {
					t.Fatalf("%s contains nonzero mask %08x", name, mask)
				}
				if !expectation.zero && mask != 0xffffffff {
					t.Fatalf("%s contains non-full mask %08x", name, mask)
				}
			}
		}
	}
}

func TestCandidateOrderRemainsOriginal(t *testing.T) {
	config, _, err := parseQTraceConfig(originalQTraceConfig)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := config.selectFrames([]int{20, 2, 9}, false, privateBoth)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int, len(frames))
	for index, frame := range frames {
		got[index] = frame.ID
	}
	want := []int{2, 9, 20, 23, 24}
	if !equalInts(got, want) {
		t.Fatalf("frame order=%v, want=%v", got, want)
	}
}

func TestPrivateFrameModesSelectOnlyVerifiedCommands(t *testing.T) {
	config, _, err := parseQTraceConfig(originalQTraceConfig)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		mode privateMode
		want []int
	}{
		{privateNone, nil},
		{private9001, []int{23}},
		{private0004, []int{24}},
		{privateBoth, []int{23, 24}},
	}
	for _, test := range tests {
		frames, selectErr := config.selectFrames(nil, false, test.mode)
		if selectErr != nil {
			t.Fatal(selectErr)
		}
		got := make([]int, len(frames))
		for index := range frames {
			got[index] = frames[index].ID
		}
		if !equalInts(got, test.want) {
			t.Fatalf("mode %s selected IDs %v, want %v", test.mode, got, test.want)
		}
	}
}

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
