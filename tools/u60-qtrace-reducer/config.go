package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	diagExtMessageConfig byte = 0x7d
	diagSubsysCommand    byte = 0x4b
	extMessageSetMask    byte = 0x04
)

type diagFrame struct {
	ID      int
	Decoded []byte // Includes the two-byte CRC, excludes the 0x7e delimiter.
}

type messageMaskFrame struct {
	Frame    diagFrame
	First    uint16
	Last     uint16
	Reserved uint16
	Masks    []uint32
}

type qtraceConfig struct {
	Frames        []diagFrame
	MessageFrames []messageMaskFrame
	PrivateFrames []diagFrame
	SourceSHA256  string
	SourceBytes   int
}

type configManifest struct {
	Source              string               `json:"source"`
	SourceSHA256        string               `json:"source_sha256"`
	SourceBytes         int                  `json:"source_bytes"`
	FrameCount          int                  `json:"frame_count"`
	MessageFrameCount   int                  `json:"message_frame_count"`
	PrivateFrameCount   int                  `json:"private_frame_count"`
	SSIDCount           int                  `json:"ssid_count"`
	AllMasksFFFFFFFF    bool                 `json:"all_masks_ffffffff"`
	RoundTripExact      bool                 `json:"round_trip_exact"`
	ContainsGlobalClear bool                 `json:"contains_global_0x7d_0x05"`
	MessageFrames       []manifestMaskFrame  `json:"message_frames"`
	PrivateFrames       []manifestDiagFrame  `json:"private_frames"`
	Generated           []generatedConfigRef `json:"generated"`
}

type manifestMaskFrame struct {
	ID        int    `json:"id"`
	FirstSSID uint16 `json:"first_ssid"`
	LastSSID  uint16 `json:"last_ssid"`
	SSIDCount int    `json:"ssid_count"`
	Mask      string `json:"mask"`
}

type manifestDiagFrame struct {
	ID         int     `json:"id"`
	Command    string  `json:"command"`
	Subsystem  *uint8  `json:"subsystem,omitempty"`
	Subcommand *uint16 `json:"subcommand,omitempty"`
	Length     int     `json:"decoded_length"`
}

type generatedConfigRef struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int    `json:"bytes"`
	FrameCount int    `json:"frame_count"`
}

func parseQTraceConfig(path string) (*qtraceConfig, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	config, err := parseQTraceConfigBytes(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return config, data, nil
}

func parseQTraceConfigBytes(data []byte) (*qtraceConfig, error) {
	sum := sha256.Sum256(data)
	config := &qtraceConfig{
		SourceSHA256: hex.EncodeToString(sum[:]),
		SourceBytes:  len(data),
	}
	segments := bytes.Split(data, []byte{0x7e})
	for _, segment := range segments {
		if len(segment) == 0 {
			continue
		}
		decoded, err := hdlcUnescape(segment)
		if err != nil {
			return nil, err
		}
		if len(decoded) < 3 {
			return nil, fmt.Errorf("frame %d is too short: %d bytes", len(config.Frames), len(decoded))
		}
		gotCRC := binary.LittleEndian.Uint16(decoded[len(decoded)-2:])
		wantCRC := diagCRC16(decoded[:len(decoded)-2])
		if gotCRC != wantCRC {
			return nil, fmt.Errorf("frame %d CRC mismatch: got=%04x want=%04x", len(config.Frames), gotCRC, wantCRC)
		}
		frame := diagFrame{ID: len(config.Frames), Decoded: append([]byte(nil), decoded...)}
		config.Frames = append(config.Frames, frame)
		switch decoded[0] {
		case diagExtMessageConfig:
			message, err := parseMessageMaskFrame(frame)
			if err != nil {
				return nil, err
			}
			config.MessageFrames = append(config.MessageFrames, message)
		case diagSubsysCommand:
			if len(decoded) < 6 {
				return nil, fmt.Errorf("subsystem frame %d is too short", frame.ID)
			}
			config.PrivateFrames = append(config.PrivateFrames, frame)
		default:
			return nil, fmt.Errorf("unsupported command 0x%02x in frame %d", decoded[0], frame.ID)
		}
	}
	if len(config.Frames) == 0 {
		return nil, errors.New("configuration contains no HDLC frames")
	}
	return config, nil
}

func parseMessageMaskFrame(frame diagFrame) (messageMaskFrame, error) {
	payload := frame.Decoded[:len(frame.Decoded)-2]
	if len(payload) < 8 {
		return messageMaskFrame{}, fmt.Errorf("message frame %d is too short", frame.ID)
	}
	if payload[1] != extMessageSetMask {
		return messageMaskFrame{}, fmt.Errorf("message frame %d uses 0x7d subcommand 0x%02x, expected 0x04", frame.ID, payload[1])
	}
	first := binary.LittleEndian.Uint16(payload[2:4])
	last := binary.LittleEndian.Uint16(payload[4:6])
	if last < first {
		return messageMaskFrame{}, fmt.Errorf("message frame %d has reversed SSID range %d..%d", frame.ID, first, last)
	}
	count := int(last-first) + 1
	wantLength := 8 + count*4
	if len(payload) != wantLength {
		return messageMaskFrame{}, fmt.Errorf("message frame %d length=%d, expected=%d for SSID range %d..%d", frame.ID, len(payload), wantLength, first, last)
	}
	masks := make([]uint32, count)
	for index := range masks {
		masks[index] = binary.LittleEndian.Uint32(payload[8+index*4:])
	}
	return messageMaskFrame{
		Frame: frame, First: first, Last: last,
		Reserved: binary.LittleEndian.Uint16(payload[6:8]), Masks: masks,
	}, nil
}

func (config *qtraceConfig) allMessageFrameIDs() []int {
	ids := make([]int, len(config.MessageFrames))
	for index, frame := range config.MessageFrames {
		ids[index] = frame.Frame.ID
	}
	return ids
}

func (config *qtraceConfig) allSSIDs(frameIDs []int) []uint16 {
	selected := make(map[int]bool, len(frameIDs))
	for _, id := range frameIDs {
		selected[id] = true
	}
	var ssids []uint16
	for _, frame := range config.MessageFrames {
		if !selected[frame.Frame.ID] {
			continue
		}
		for ssid := uint32(frame.First); ssid <= uint32(frame.Last); ssid++ {
			ssids = append(ssids, uint16(ssid))
		}
	}
	return ssids
}

func (config *qtraceConfig) zeroMessageFrames() ([]diagFrame, error) {
	frames := make([]diagFrame, 0, len(config.MessageFrames))
	for _, original := range config.MessageFrames {
		masks := make([]uint32, len(original.Masks))
		frame, err := buildMessageMaskFrame(original.Frame.ID, original.First, original.Last, original.Reserved, masks)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

func buildMessageMaskFrame(id int, first, last, reserved uint16, masks []uint32) (diagFrame, error) {
	if last < first || len(masks) != int(last-first)+1 {
		return diagFrame{}, fmt.Errorf("invalid SSID range/mask count for frame %d", id)
	}
	payload := make([]byte, 8+len(masks)*4)
	payload[0] = diagExtMessageConfig
	payload[1] = extMessageSetMask
	binary.LittleEndian.PutUint16(payload[2:4], first)
	binary.LittleEndian.PutUint16(payload[4:6], last)
	binary.LittleEndian.PutUint16(payload[6:8], reserved)
	for index, mask := range masks {
		binary.LittleEndian.PutUint32(payload[8+index*4:], mask)
	}
	crc := diagCRC16(payload)
	decoded := append(payload, byte(crc), byte(crc>>8))
	return diagFrame{ID: id, Decoded: decoded}, nil
}

type privateMode string

const (
	privateNone privateMode = "none"
	privateBoth privateMode = "both"
	private9001 privateMode = "0x44/0x9001-only"
	private0004 privateMode = "0x55/0x0004-only"
)

func (config *qtraceConfig) selectFrames(messageFrameIDs []int, zeroMasks bool, private privateMode) ([]diagFrame, error) {
	selected := make(map[int]bool, len(messageFrameIDs))
	for _, id := range messageFrameIDs {
		selected[id] = true
	}
	zeroByID := map[int]diagFrame{}
	if zeroMasks {
		zeroFrames, err := config.zeroMessageFrames()
		if err != nil {
			return nil, err
		}
		for _, frame := range zeroFrames {
			zeroByID[frame.ID] = frame
		}
	}
	frames := make([]diagFrame, 0, len(messageFrameIDs)+len(config.PrivateFrames))
	for _, original := range config.Frames {
		if selected[original.ID] {
			if zeroMasks {
				frames = append(frames, zeroByID[original.ID])
			} else {
				frames = append(frames, original)
			}
			continue
		}
		if original.Decoded[0] == diagSubsysCommand && privateFrameSelected(original, private) {
			frames = append(frames, original)
		}
	}
	return frames, nil
}

func privateFrameSelected(frame diagFrame, mode privateMode) bool {
	if mode == privateNone || len(frame.Decoded) < 6 || frame.Decoded[0] != diagSubsysCommand {
		return false
	}
	if mode == privateBoth {
		return true
	}
	subsystem := frame.Decoded[1]
	subcommand := binary.LittleEndian.Uint16(frame.Decoded[2:4])
	return mode == private9001 && subsystem == 0x44 && subcommand == 0x9001 ||
		mode == private0004 && subsystem == 0x55 && subcommand == 0x0004
}

func encodeConfig(frames []diagFrame) []byte {
	var output []byte
	for _, frame := range frames {
		output = append(output, hdlcEscape(frame.Decoded)...)
		output = append(output, 0x7e)
	}
	return output
}

func hdlcEscape(decoded []byte) []byte {
	encoded := make([]byte, 0, len(decoded)+8)
	for _, value := range decoded {
		switch value {
		case 0x7d, 0x7e:
			encoded = append(encoded, 0x7d, value^0x20)
		default:
			encoded = append(encoded, value)
		}
	}
	return encoded
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

// diagCRC16 is the reflected CRC-16/X-25 used by Qualcomm DIAG HDLC frames.
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

func writeDryRun(configPath, outputDir string) (*configManifest, error) {
	config, source, err := parseQTraceConfig(configPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return nil, err
	}
	allIDs := config.allMessageFrameIDs()
	zeroFrames, err := config.zeroMessageFrames()
	if err != nil {
		return nil, err
	}
	positive, _ := config.selectFrames(allIDs, false, privateBoth)
	zeroPrivate, _ := config.selectFrames(allIDs, true, privateBoth)
	messageOnly, _ := config.selectFrames(allIDs, false, privateNone)
	allOff, _ := config.selectFrames(allIDs, true, privateNone)
	privateBothOnly, _ := config.selectFrames(nil, false, privateBoth)
	private9001Only, _ := config.selectFrames(nil, false, private9001)
	private0004Only, _ := config.selectFrames(nil, false, private0004)
	outputs := []struct {
		name   string
		frames []diagFrame
	}{
		{"original-roundtrip.cfg", positive},
		{"cleanup-zero.cfg", zeroFrames},
		{filepath.Join("controls", "positive.cfg"), positive},
		{filepath.Join("controls", "zero-mask-private.cfg"), zeroPrivate},
		{filepath.Join("controls", "message-only.cfg"), messageOnly},
		{filepath.Join("controls", "all-off.cfg"), allOff},
		{filepath.Join("controls", "private-both-only.cfg"), privateBothOnly},
		{filepath.Join("controls", "private-9001-only.cfg"), private9001Only},
		{filepath.Join("controls", "private-0004-only.cfg"), private0004Only},
	}
	manifest := &configManifest{
		Source: configPath, SourceSHA256: config.SourceSHA256, SourceBytes: config.SourceBytes,
		FrameCount: len(config.Frames), MessageFrameCount: len(config.MessageFrames),
		PrivateFrameCount: len(config.PrivateFrames), RoundTripExact: bytes.Equal(source, encodeConfig(positive)),
		AllMasksFFFFFFFF: true,
	}
	for _, frame := range config.MessageFrames {
		manifest.SSIDCount += len(frame.Masks)
		maskName := "mixed"
		allFull := true
		for _, mask := range frame.Masks {
			if mask != 0xffffffff {
				allFull = false
				manifest.AllMasksFFFFFFFF = false
			}
		}
		if allFull {
			maskName = "0xffffffff"
		}
		manifest.MessageFrames = append(manifest.MessageFrames, manifestMaskFrame{
			ID: frame.Frame.ID, FirstSSID: frame.First, LastSSID: frame.Last,
			SSIDCount: len(frame.Masks), Mask: maskName,
		})
	}
	for _, frame := range config.PrivateFrames {
		payload := frame.Decoded[:len(frame.Decoded)-2]
		subsystem := payload[1]
		subcommand := binary.LittleEndian.Uint16(payload[2:4])
		manifest.PrivateFrames = append(manifest.PrivateFrames, manifestDiagFrame{
			ID: frame.ID, Command: fmt.Sprintf("0x%02x", payload[0]),
			Subsystem: &subsystem, Subcommand: &subcommand, Length: len(frame.Decoded),
		})
	}
	for _, frame := range config.Frames {
		payload := frame.Decoded[:len(frame.Decoded)-2]
		if len(payload) >= 2 && payload[0] == 0x7d && payload[1] == 0x05 {
			manifest.ContainsGlobalClear = true
		}
	}
	for _, output := range outputs {
		path := filepath.Join(outputDir, output.name)
		data := encodeConfig(output.frames)
		if err := writeFileAtomic(path, data, 0600); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		manifest.Generated = append(manifest.Generated, generatedConfigRef{
			Name: output.name, Path: path, SHA256: hex.EncodeToString(sum[:]),
			Bytes: len(data), FrameCount: len(output.frames),
		})
		parsed, err := parseQTraceConfigBytes(data)
		if err != nil || len(parsed.Frames) != len(output.frames) {
			return nil, fmt.Errorf("generated config %s failed round-trip validation: %w", output.name, err)
		}
	}
	sort.Slice(manifest.Generated, func(i, j int) bool { return manifest.Generated[i].Name < manifest.Generated[j].Name })
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestData = append(manifestData, '\n')
	if err := writeFileAtomic(filepath.Join(outputDir, "manifest.json"), manifestData, 0600); err != nil {
		return nil, err
	}
	return manifest, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".u60-qtrace-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}
