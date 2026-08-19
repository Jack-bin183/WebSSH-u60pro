package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	diagSubsysCommand  = 0x4b
	qshSubsystem       = 0x44
	qshConfigCommand   = 0x9001
	activeQSHID        = 96
	activeQSHMaskBit   = uint16(0x8000)
	expectedBaseSHA256 = "15b669eff11b53cf0fa461fe91b3c9b086912dee7a80a166c47d7d1b8c646c58"
)

type qtracePatchResult struct {
	SourceSHA256 string
	SourceBytes  int
	ActiveSHA256 string
	ActiveBytes  int
	OriginalMask uint16
	ActiveMask   uint16
}

func buildActiveQTrace(sourcePath, outputPath string) (qtracePatchResult, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return qtracePatchResult{}, err
	}
	active, result, err := patchActiveQTrace(data)
	if err != nil {
		return qtracePatchResult{}, fmt.Errorf("patch %s: %w", sourcePath, err)
	}
	if err := writeFileAtomic(outputPath, active, 0600); err != nil {
		return qtracePatchResult{}, err
	}
	return result, nil
}

func patchActiveQTrace(data []byte) ([]byte, qtracePatchResult, error) {
	result := qtracePatchResult{SourceBytes: len(data)}
	sourceHash := sha256.Sum256(data)
	result.SourceSHA256 = hex.EncodeToString(sourceHash[:])
	if result.SourceSHA256 != expectedBaseSHA256 {
		return nil, result, fmt.Errorf("unexpected production qtrace SHA-256 %s (want %s)", result.SourceSHA256, expectedBaseSHA256)
	}
	frames, err := decodeQTraceFrames(data)
	if err != nil {
		return nil, result, err
	}
	if len(frames) != 1 {
		return nil, result, fmt.Errorf("production minimum must contain exactly one frame, got %d", len(frames))
	}
	payload := frames[0][:len(frames[0])-2]
	if len(payload) < 20 || payload[0] != diagSubsysCommand || payload[1] != qshSubsystem || binary.LittleEndian.Uint16(payload[2:4]) != qshConfigCommand {
		return nil, result, errors.New("production frame is not 0x44/0x9001")
	}
	count := int(binary.LittleEndian.Uint32(payload[16:20]))
	if count <= 0 || len(payload) != 20+count*4 {
		return nil, result, fmt.Errorf("invalid 0x9001 ID/mask table: count=%d payload=%d", count, len(payload))
	}
	found := 0
	for index := 0; index < count; index++ {
		offset := 20 + index*4
		id := binary.LittleEndian.Uint16(payload[offset : offset+2])
		if id != activeQSHID {
			continue
		}
		found++
		result.OriginalMask = binary.LittleEndian.Uint16(payload[offset+2 : offset+4])
		result.ActiveMask = result.OriginalMask | activeQSHMaskBit
		binary.LittleEndian.PutUint16(payload[offset+2:offset+4], result.ActiveMask)
	}
	if found != 1 {
		return nil, result, fmt.Errorf("expected exactly one QSH ID %d entry, got %d", activeQSHID, found)
	}
	if result.OriginalMask&activeQSHMaskBit != 0 {
		return nil, result, fmt.Errorf("QSH ID %d bit 15 is already enabled", activeQSHID)
	}
	crc := diagCRC16(payload)
	frames[0][len(frames[0])-2] = byte(crc)
	frames[0][len(frames[0])-1] = byte(crc >> 8)
	active := encodeQTraceFrames(frames)
	verified, err := decodeQTraceFrames(active)
	if err != nil || len(verified) != 1 || !bytes.Equal(verified[0], frames[0]) {
		return nil, result, fmt.Errorf("generated active qtrace failed round-trip validation: %w", err)
	}
	activeHash := sha256.Sum256(active)
	result.ActiveSHA256 = hex.EncodeToString(activeHash[:])
	result.ActiveBytes = len(active)
	return active, result, nil
}

func decodeQTraceFrames(data []byte) ([][]byte, error) {
	var frames [][]byte
	for _, encoded := range bytes.Split(data, []byte{0x7e}) {
		if len(encoded) == 0 {
			continue
		}
		decoded, err := hdlcUnescape(encoded)
		if err != nil {
			return nil, err
		}
		if len(decoded) < 3 {
			return nil, fmt.Errorf("frame %d is too short", len(frames))
		}
		got := binary.LittleEndian.Uint16(decoded[len(decoded)-2:])
		want := diagCRC16(decoded[:len(decoded)-2])
		if got != want {
			return nil, fmt.Errorf("frame %d CRC mismatch: got=%04x want=%04x", len(frames), got, want)
		}
		frames = append(frames, append([]byte(nil), decoded...))
	}
	if len(frames) == 0 {
		return nil, errors.New("qtrace contains no frames")
	}
	return frames, nil
}

func encodeQTraceFrames(frames [][]byte) []byte {
	var output []byte
	for _, frame := range frames {
		for _, value := range frame {
			switch value {
			case 0x7d, 0x7e:
				output = append(output, 0x7d, value^0x20)
			default:
				output = append(output, value)
			}
		}
		output = append(output, 0x7e)
	}
	return output
}

func hdlcUnescape(encoded []byte) ([]byte, error) {
	decoded := make([]byte, 0, len(encoded))
	escaped := false
	for _, value := range encoded {
		if escaped {
			decoded = append(decoded, value^0x20)
			escaped = false
			continue
		}
		if value == 0x7d {
			escaped = true
			continue
		}
		decoded = append(decoded, value)
	}
	if escaped {
		return nil, errors.New("truncated HDLC escape")
	}
	return decoded, nil
}

func diagCRC16(data []byte) uint16 {
	crc := uint16(0xffff)
	for _, value := range data {
		crc ^= uint16(value)
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8408
			} else {
				crc >>= 1
			}
		}
	}
	return crc ^ 0xffff
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".u60-nr-probe-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	ok := false
	defer func() {
		_ = temporary.Close()
		if !ok {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}
