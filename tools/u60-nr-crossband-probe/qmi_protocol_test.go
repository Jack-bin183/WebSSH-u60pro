package main

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestAdvancedRequestCarriesOneARFCN(t *testing.T) {
	request, err := makeNRIncrementalScanRequest(1, []uint32{504990}, 2)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString("1d0500019eb40700")
	if !bytes.Contains(request, want) {
		t.Fatalf("request=%x missing TLV=%x", request, want)
	}
	_, _, message, tlvs, err := parseQMIPacket(request)
	if err != nil || message != 0x0085 || len(tlvs) != 6 {
		t.Fatalf("message=%04x tlvs=%d err=%v", message, len(tlvs), err)
	}
}

func TestQCRILOneShotDropsBandAndChannel(t *testing.T) {
	request, err := makeQCRILOneShotExpectedRequest(1, 0x10)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, tlvs, err := parseQMIPacket(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(tlvs) != 1 || tlvs[0].Type != 0x10 || !bytes.Equal(tlvs[0].Value, []byte{0x10}) {
		t.Fatalf("unexpected TLVs: %#v", tlvs)
	}
}

func TestRejectUnvalidatedScanTypes(t *testing.T) {
	for _, scanType := range []uint32{1, 3, 4, 5} {
		if _, err := makeNRIncrementalScanRequest(1, []uint32{504990}, scanType); err == nil {
			t.Fatalf("scan type %d unexpectedly accepted", scanType)
		}
	}
}
