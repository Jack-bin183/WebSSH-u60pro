package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	activeResultHash uint32 = 0xd8f582a8
	foundCellHash    uint32 = 0xd8fa712c
	arfcnCountHash   uint32 = 0xf9f48cf1
	nrARFCNHash      uint32 = 0xf9f48e26
	nr2nrStartHash1  uint32 = 0xfa16e120
	nr2nrStartHash2  uint32 = 0xfa16e20f
	nr2nrStartHash3  uint32 = 0xfa16e581
)

type qmdlMetrics struct {
	QSHFrameCount       int           `json:"qsh_frame_count"`
	ActiveHashCount     int           `json:"active_hash_count"`
	MalformedFrameCount int           `json:"malformed_frame_count"`
	QMDLBytes           int64         `json:"qmdl_bytes"`
	Results             []ml1Result   `json:"results"`
	Evidence            []qshEvidence `json:"evidence,omitempty"`
}

func analyzeQMDLFiles(paths []string, target targetSpec) (qmdlMetrics, error) {
	sort.Strings(paths)
	metrics := qmdlMetrics{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return metrics, err
		}
		metrics.QMDLBytes += info.Size()
		file, err := os.Open(path)
		if err != nil {
			return metrics, err
		}
		err = consumeQMDL(file, target, &metrics)
		closeErr := file.Close()
		if err != nil {
			return metrics, fmt.Errorf("parse %s: %w", path, err)
		}
		if closeErr != nil {
			return metrics, closeErr
		}
	}
	return metrics, nil
}

func consumeQMDL(source io.Reader, target targetSpec, metrics *qmdlMetrics) error {
	reader := bufio.NewReaderSize(source, 64*1024)
	frame := make([]byte, 0, 1024)
	escaped := false
	for {
		value, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if value == 0x7e {
			if len(frame) != 0 {
				consumeQMDLFrame(frame, escaped, target, metrics)
			}
			frame = frame[:0]
			escaped = false
			continue
		}
		if escaped {
			frame = append(frame, value^0x20)
			escaped = false
			continue
		}
		if value == 0x7d {
			escaped = true
			continue
		}
		frame = append(frame, value)
	}
}

func consumeQMDLFrame(frame []byte, escaped bool, target targetSpec, metrics *qmdlMetrics) {
	if escaped || len(frame) < 18 {
		metrics.MalformedFrameCount++
		return
	}
	payload := frame[:len(frame)-2]
	if binary.LittleEndian.Uint16(frame[len(frame)-2:]) != diagCRC16(payload) {
		metrics.MalformedFrameCount++
		return
	}
	if payload[0] != 0x9d || len(payload) < 16 {
		return
	}
	metrics.QSHFrameCount++
	wordCount := int(payload[4]) - 0x13
	if wordCount < 0 || wordCount > 236 || len(payload) < 16+wordCount*4 {
		metrics.MalformedFrameCount++
		return
	}
	hash := binary.LittleEndian.Uint32(payload[12:16])
	if hash != activeResultHash && hash != foundCellHash && hash != arfcnCountHash && hash != nrARFCNHash &&
		hash != nr2nrStartHash1 && hash != nr2nrStartHash2 && hash != nr2nrStartHash3 {
		return
	}
	words := make([]uint32, wordCount)
	for index := range words {
		words[index] = binary.LittleEndian.Uint32(payload[16+index*4:])
	}
	if hash == activeResultHash {
		metrics.ActiveHashCount++
		result, ok := decodeActiveResult(words, target)
		if ok {
			metrics.Results = append(metrics.Results, result)
		}
		return
	}
	evidence := qshEvidence{Hash: fmt.Sprintf("0x%08x", hash), Kind: qshEvidenceKind(hash), Words: words}
	switch {
	case hash == foundCellHash && len(words) >= 2:
		arfcn, pci := words[0], words[1]
		evidence.ARFCN, evidence.PCI = &arfcn, &pci
	case hash == nrARFCNHash && len(words) >= 2:
		arfcn := words[1]
		evidence.ARFCN = &arfcn
	case (hash == nr2nrStartHash1 || hash == nr2nrStartHash2 || hash == nr2nrStartHash3) && len(words) > 0:
		arfcn := words[len(words)-1]
		if arfcn > 0 && arfcn <= 3279165 {
			evidence.ARFCN = &arfcn
		}
	}
	metrics.Evidence = append(metrics.Evidence, evidence)
}

func decodeActiveResult(words []uint32, target targetSpec) (ml1Result, bool) {
	if len(words) < 9 {
		return ml1Result{}, false
	}
	pci, arfcn, valid := words[0], words[1], words[2]
	if pci > 1007 || arfcn == 0 || arfcn > 3279165 || valid != 1 {
		return ml1Result{}, false
	}
	rsrpRaw := int32(words[3])
	rsrp := float64(rsrpRaw) / 128
	if rsrp < -160 || rsrp > -20 {
		return ml1Result{}, false
	}
	sinrRaw := int32(words[5])
	sinr := float64(sinrRaw) / 128
	sinrInteger := int32(words[6])
	rsrqRaw := int32(words[7])
	rsrq := float64(rsrqRaw) / 128
	energy := int32(words[8])
	band := nrBandForARFCN(arfcn)
	if arfcn == target.ARFCN {
		band = target.Band
	}
	return ml1Result{
		Hash: "0xd8f582a8", PCI: pci, ARFCN: arfcn, Band: band, Valid: valid,
		RSRP: rsrp, RSRPInteger: int32(words[4]), SINR: &sinr, SINRInteger: &sinrInteger,
		RSRQ: &rsrq, SearchEnergy: &energy, Words: words,
	}, true
}

func matchesTargetResult(result ml1Result, target targetSpec) bool {
	if result.ARFCN != target.ARFCN {
		return false
	}
	return target.PCI == nil || result.PCI == *target.PCI
}

func nrBandForARFCN(arfcn uint32) uint32 {
	switch {
	case arfcn >= 151600 && arfcn <= 160600:
		return 28
	case arfcn >= 499200 && arfcn <= 537999:
		return 41
	default:
		return 0
	}
}

func qshEvidenceKind(hash uint32) string {
	switch hash {
	case foundCellHash:
		return "ml1-found-cell"
	case arfcnCountHash:
		return "cm-arfcn-count"
	case nrARFCNHash:
		return "cm-nr-arfcn"
	case nr2nrStartHash1, nr2nrStartHash2, nr2nrStartHash3:
		return "nr2nr-search-start"
	default:
		return "unknown"
	}
}
