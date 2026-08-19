package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchProductionQTrace(t *testing.T) {
	path := filepath.Join("..", "..", "gossh", "app", "service", "embed", "qtrace.cfg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	active, result, err := patchActiveQTrace(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.OriginalMask != 0x0007 || result.ActiveMask != 0x8007 {
		t.Fatalf("masks original=%04x active=%04x", result.OriginalMask, result.ActiveMask)
	}
	frames, err := decodeQTraceFrames(active)
	if err != nil {
		t.Fatal(err)
	}
	payload := frames[0][:len(frames[0])-2]
	count := int(binary.LittleEndian.Uint32(payload[16:20]))
	found := false
	for index := 0; index < count; index++ {
		offset := 20 + index*4
		if binary.LittleEndian.Uint16(payload[offset:]) == 96 {
			found = binary.LittleEndian.Uint16(payload[offset+2:]) == 0x8007
		}
	}
	if !found {
		t.Fatal("QSH ID 96 mask 0x8007 not found")
	}
	sum := sha256.Sum256(active)
	if got := hex.EncodeToString(sum[:]); got != "b1db1e7a4c82bd94513d080a4410b596634b5d546a348e8921c4104485539d54" {
		t.Fatalf("active SHA-256=%s", got)
	}
}

func TestRejectUnknownQTrace(t *testing.T) {
	if _, _, err := patchActiveQTrace([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected source hash rejection")
	}
}
