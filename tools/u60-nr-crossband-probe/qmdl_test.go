package main

import (
	"encoding/binary"
	"testing"
)

func TestDecodeActiveResult(t *testing.T) {
	rsrpQ7, rsrpInteger := int32(-95*128), int32(-95)
	sinrQ7, rsrqQ7 := int32(12*128), int32(-11*128)
	words := []uint32{123, 504990, 1, uint32(rsrpQ7), uint32(rsrpInteger), uint32(sinrQ7), 12, uint32(rsrqQ7), 44}
	result, ok := decodeActiveResult(words, targetSpec{Band: 41, ARFCN: 504990})
	if !ok {
		t.Fatal("valid result rejected")
	}
	if result.PCI != 123 || result.Band != 41 || result.RSRP != -95 || result.RSRQ == nil || *result.RSRQ != -11 {
		t.Fatalf("result=%+v", result)
	}
}

func TestConsumeActiveQSHFrame(t *testing.T) {
	rsrpQ7, rsrpInteger := int32(-80*128), int32(-80)
	sinrQ7, rsrqQ7 := int32(9*128), int32(-10*128)
	words := []uint32{266, 504990, 1, uint32(rsrpQ7), uint32(rsrpInteger), uint32(sinrQ7), 9, uint32(rsrqQ7), 1}
	payload := make([]byte, 16+len(words)*4)
	payload[0] = 0x9d
	payload[4] = byte(0x13 + len(words))
	binary.LittleEndian.PutUint32(payload[12:16], activeResultHash)
	for index, word := range words {
		binary.LittleEndian.PutUint32(payload[16+index*4:], word)
	}
	crc := diagCRC16(payload)
	frame := append(payload, byte(crc), byte(crc>>8))
	metrics := qmdlMetrics{}
	consumeQMDLFrame(frame, false, targetSpec{Band: 41, ARFCN: 504990}, &metrics)
	if metrics.QSHFrameCount != 1 || metrics.ActiveHashCount != 1 || len(metrics.Results) != 1 {
		t.Fatalf("metrics=%+v", metrics)
	}
}

func TestCountTargetResults(t *testing.T) {
	target := targetSpec{Band: 41, ARFCN: 504990}
	results := []ml1Result{{ARFCN: 152650, PCI: 1}, {ARFCN: 504990, PCI: 2}, {ARFCN: 504990, PCI: 3}}
	if count := countTargetResults(results, target); count != 2 {
		t.Fatalf("count=%d", count)
	}
	pci := uint32(3)
	target.PCI = &pci
	if count := countTargetResults(results, target); count != 1 {
		t.Fatalf("PCI-filtered count=%d", count)
	}
}
