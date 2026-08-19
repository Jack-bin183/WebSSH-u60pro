package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

const maxQSHWords = 236

type targetKind string

const (
	targetNR       targetKind = "nr"
	targetLTE      targetKind = "lte"
	targetCombined targetKind = "combined"
)

func parseTarget(value string) (targetKind, error) {
	switch targetKind(strings.ToLower(strings.TrimSpace(value))) {
	case targetNR:
		return targetNR, nil
	case targetLTE:
		return targetLTE, nil
	case targetCombined, "nr+lte", "lte+nr":
		return targetCombined, nil
	default:
		return "", fmt.Errorf("invalid target %q (want nr, lte, or combined)", value)
	}
}

type servingIdentity struct {
	RAT   string `json:"rat"`
	PCI   uint32 `json:"pci"`
	ARFCN uint32 `json:"arfcn,omitempty"`
}

type parsedCell struct {
	RAT     string  `json:"rat"`
	PCI     uint32  `json:"pci"`
	ARFCN   uint32  `json:"arfcn"`
	Band    uint32  `json:"band,omitempty"`
	RSRP    float64 `json:"rsrp"`
	Samples int     `json:"samples"`
}

type captureMetrics struct {
	CapturedBytes     int64        `json:"captured_bytes"`
	FrameCount        int          `json:"frame_count"`
	MalformedFrames   int          `json:"malformed_frames"`
	QSHFrameCount     int          `json:"qsh_frame_count"`
	QSHTotalBytes     int64        `json:"qsh_total_bytes"`
	TargetHashCount   int          `json:"target_hash_count"`
	ParseSuccessCount int          `json:"parse_success_count"`
	ParseErrorCount   int          `json:"parse_error_count"`
	NRCellCount       int          `json:"nr_cell_count"`
	LTECellCount      int          `json:"lte_cell_count"`
	NRHashCount       int          `json:"nr_hash_count"`
	LTEHashCount      int          `json:"lte_hash_count"`
	Cells             []parsedCell `json:"cells,omitempty"`
}

func (metrics captureMetrics) matches(target targetKind) bool {
	switch target {
	case targetNR:
		return metrics.NRCellCount > 0
	case targetLTE:
		return metrics.LTECellCount > 0
	case targetCombined:
		return metrics.NRCellCount > 0 && metrics.LTECellCount > 0
	default:
		return false
	}
}

type qshCandidate struct {
	seq   uint32
	file  uint32
	pci   uint32
	arfcn uint32
	band  uint32
}

type qshReport struct {
	seq           uint32
	file          uint32
	rat           uint32 // 0=NR, 1=LTE
	pci           uint32
	arfcn         uint32
	band          uint32
	rsrp          float64
	direct        bool
	requiresARFCN bool
}

type qshParser struct {
	metrics    captureMetrics
	candidates []qshCandidate
	reports    []qshReport
	serving    []servingIdentity
}

func analyzeQMDLFiles(ctx context.Context, paths []string, serving []servingIdentity, target targetKind) (captureMetrics, error) {
	return analyzeQMDLWindow(ctx, paths, nil, serving, target)
}

// analyzeQMDLWindow ignores bytes that existed at the end of the settle
// interval. If an offset lands in the middle of an in-flight HDLC frame, that
// first partial frame is discarded through its delimiter.
func analyzeQMDLWindow(ctx context.Context, paths []string, offsets map[string]int64, serving []servingIdentity, target targetKind) (captureMetrics, error) {
	parser := &qshParser{serving: serving}
	for index, path := range paths {
		if err := ctx.Err(); err != nil {
			return captureMetrics{}, err
		}
		file, err := os.Open(path)
		if err != nil {
			return captureMetrics{}, err
		}
		offset := int64(0)
		discardFirst := false
		if offsets != nil {
			offset = offsets[path]
		}
		if info, statErr := file.Stat(); statErr == nil {
			if offset > info.Size() {
				offset = info.Size()
			}
			parser.metrics.CapturedBytes += info.Size() - offset
		}
		if offset > 0 {
			previous := []byte{0}
			if _, readErr := file.ReadAt(previous, offset-1); readErr == nil && previous[0] != 0x7e {
				discardFirst = true
			}
			if _, err := file.Seek(offset, io.SeekStart); err != nil {
				_ = file.Close()
				return captureMetrics{}, err
			}
		}
		err = parser.parseReader(ctx, file, uint32(index), target, discardFirst)
		closeErr := file.Close()
		if err != nil {
			return captureMetrics{}, fmt.Errorf("parse %s: %w", path, err)
		}
		if closeErr != nil {
			return captureMetrics{}, closeErr
		}
	}
	parser.finish()
	return parser.metrics, nil
}

func (parser *qshParser) parseReader(ctx context.Context, source io.Reader, fileIndex uint32, target targetKind, discardFirst bool) error {
	reader := bufio.NewReaderSize(source, 64*1024)
	frame := make([]byte, 0, 1024)
	escaped := false
	seq := uint32(0)
	for bytesRead := 0; ; bytesRead++ {
		if bytesRead&0x3fff == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		value, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// A growing QMDL may end in a partial frame. Never treat it as a
				// complete record; the next poll will see it after the delimiter.
				return nil
			}
			return err
		}
		if value == 0x7e {
			if discardFirst {
				frame = frame[:0]
				escaped = false
				discardFirst = false
				continue
			}
			if len(frame) != 0 {
				parser.consumeFrame(frame, escaped, seq, fileIndex, target)
				seq++
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

func (parser *qshParser) consumeFrame(frame []byte, escaped bool, seq, fileIndex uint32, target targetKind) {
	parser.metrics.FrameCount++
	if escaped || len(frame) <= 2 {
		parser.metrics.MalformedFrames++
		parser.metrics.ParseErrorCount++
		return
	}
	payload := frame[:len(frame)-2]
	gotCRC := binary.LittleEndian.Uint16(frame[len(frame)-2:])
	if gotCRC != diagCRC16(payload) {
		parser.metrics.MalformedFrames++
		parser.metrics.ParseErrorCount++
		return
	}
	if len(payload) <= 15 || payload[0] != 0x9d {
		return
	}
	parser.metrics.QSHFrameCount++
	parser.metrics.QSHTotalBytes += int64(len(payload))
	wordCount := int(payload[4]) - 0x13
	if wordCount < 0 || wordCount > maxQSHWords || len(payload) < (wordCount+4)*4 {
		parser.metrics.ParseErrorCount++
		return
	}
	hash := binary.LittleEndian.Uint32(payload[12:16])
	words := make([]uint32, wordCount)
	for index := range words {
		words[index] = binary.LittleEndian.Uint32(payload[16+index*4:])
	}
	if candidate, ok := decodeQSHCandidate(hash, words); ok {
		candidate.seq, candidate.file = seq, fileIndex
		parser.candidates = append(parser.candidates, candidate)
	}
	if _, _, isNR := nrReportSpec(hash); isNR {
		parser.metrics.NRHashCount++
		if target == targetNR || target == targetCombined {
			parser.metrics.TargetHashCount++
		}
		strict, minimum, _ := nrReportSpec(hash)
		if len(words) < minimum || !parser.addNRReport(seq, fileIndex, words, strict) {
			parser.metrics.ParseErrorCount++
		} else {
			parser.metrics.ParseSuccessCount++
		}
		return
	}
	if report, recognized, valid := decodeQSHLTEReport(hash, words); recognized {
		parser.metrics.LTEHashCount++
		if target == targetLTE || target == targetCombined {
			parser.metrics.TargetHashCount++
		}
		if !valid {
			parser.metrics.ParseErrorCount++
			return
		}
		report.seq, report.file = seq, fileIndex
		parser.reports = append(parser.reports, report)
		parser.metrics.ParseSuccessCount++
	}
}

func (parser *qshParser) addNRReport(seq, fileIndex uint32, words []uint32, requiresARFCN bool) bool {
	if len(words) < 7 || words[3] > 1007 || words[5] > 1007 {
		return false
	}
	rsrpRaw := int32(words[4])
	if !validQ7Signal(rsrpRaw) || !validQ7Signal(int32(words[6])) {
		return false
	}
	parser.reports = append(parser.reports, qshReport{
		seq: seq, file: fileIndex, pci: words[3], rsrp: roundQSH(float64(rsrpRaw) / 128),
		direct: true, requiresARFCN: requiresARFCN,
	})
	return true
}

func (parser *qshParser) finish() {
	parser.correlateNRReports()
	type aggregateKey struct {
		rat, pci, arfcn uint32
	}
	type aggregate struct {
		band uint32
		rsrp []float64
	}
	aggregates := map[aggregateKey]*aggregate{}
	for _, report := range parser.reports {
		if !report.direct || report.arfcn == 0 || report.rsrp < -140 || report.rsrp > -30 {
			continue
		}
		key := aggregateKey{rat: report.rat, pci: report.pci, arfcn: report.arfcn}
		if parser.isServing(report.rat, report.pci, report.arfcn) {
			continue
		}
		item := aggregates[key]
		if item == nil {
			item = &aggregate{}
			aggregates[key] = item
		}
		if report.band != 0 {
			item.band = report.band
		}
		item.rsrp = append(item.rsrp, report.rsrp)
	}
	keys := make([]aggregateKey, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].rat != keys[j].rat {
			return keys[i].rat < keys[j].rat
		}
		if keys[i].arfcn != keys[j].arfcn {
			return keys[i].arfcn < keys[j].arfcn
		}
		return keys[i].pci < keys[j].pci
	})
	for _, key := range keys {
		item := aggregates[key]
		sort.Float64s(item.rsrp)
		median := item.rsrp[len(item.rsrp)/2]
		if len(item.rsrp)%2 == 0 {
			median = roundQSH((item.rsrp[len(item.rsrp)/2-1] + median) / 2)
		}
		rat := "NR"
		if key.rat == 1 {
			rat = "LTE"
			parser.metrics.LTECellCount++
		} else {
			parser.metrics.NRCellCount++
		}
		parser.metrics.Cells = append(parser.metrics.Cells, parsedCell{
			RAT: rat, PCI: key.pci, ARFCN: key.arfcn, Band: item.band,
			RSRP: median, Samples: len(item.rsrp),
		})
	}
}

func (parser *qshParser) isServing(rat, pci, arfcn uint32) bool {
	ratName := "NR"
	if rat == 1 {
		ratName = "LTE"
	}
	for _, cell := range parser.serving {
		if strings.EqualFold(cell.RAT, ratName) && cell.PCI == pci && (cell.ARFCN == 0 || arfcn == 0 || cell.ARFCN == arfcn) {
			return true
		}
	}
	return false
}

func (parser *qshParser) correlateNRReports() {
	for reportIndex := range parser.reports {
		report := &parser.reports[reportIndex]
		if report.rat != 0 {
			continue
		}
		best := -1
		bestDistance := uint32(math.MaxUint32)
		for candidateIndex := range parser.candidates {
			candidate := &parser.candidates[candidateIndex]
			if candidate.file != report.file || candidate.pci != report.pci {
				continue
			}
			distance := unsignedDistance(candidate.seq, report.seq)
			if best < 0 || distance < bestDistance {
				best, bestDistance = candidateIndex, distance
			}
		}
		if best >= 0 {
			report.arfcn = parser.candidates[best].arfcn
			report.band = parser.candidates[best].band
		}
	}
}

// The hash tables and field layouts below intentionally mirror
// gossh/app/service/neighbor_qmdl.go. They are kept in this standalone tool so
// the production WebSSH package and its runtime lifecycle remain untouched.
func decodeQSHLTEReport(hash uint32, words []uint32) (qshReport, bool, bool) {
	var arfcn, pci uint32
	rsrpIndex := -1
	rsrpFractionIndex := -1
	rsrqIndex := -1
	requireValid := false
	switch hash {
	case 0xd936846c:
		if len(words) < 2 {
			return qshReport{}, true, false
		}
		arfcn, pci = words[0], words[1]
	case 0xd8fe54a0:
		if len(words) < 4 {
			return qshReport{}, true, false
		}
		arfcn, pci, rsrpIndex, rsrqIndex = words[0], words[1], 2, 3
	case 0xd8fea29c:
		if len(words) < 6 {
			return qshReport{}, true, false
		}
		arfcn, pci, rsrpIndex, rsrqIndex = words[1], words[2], 3, 4
	case 0xd8f02d94, 0xd8fea210:
		if len(words) < 6 {
			return qshReport{}, true, false
		}
		arfcn, pci, rsrpIndex, rsrqIndex, requireValid = words[2], words[3], 4, 5, true
	case 0xd939607c:
		if len(words) < 2 {
			return qshReport{}, true, false
		}
		arfcn, pci = words[0], words[1]
	case 0xd93960a0, 0xd8fea258:
		if len(words) < 5 {
			return qshReport{}, true, false
		}
		arfcn, pci, rsrpIndex, rsrqIndex = words[0], words[1], 2, 3
	case 0xd94f5690:
		if len(words) < 6 {
			return qshReport{}, true, false
		}
		arfcn, pci, rsrpIndex, rsrpFractionIndex = words[0], words[1], 2, 3
	default:
		return qshReport{}, false, false
	}
	if requireValid && words[1] == 0 {
		return qshReport{}, true, false
	}
	if arfcn == 0 || arfcn > 262143 || pci > 503 {
		return qshReport{}, true, false
	}
	report := qshReport{rat: 1, pci: pci, arfcn: arfcn}
	if rsrpIndex < 0 {
		// Identity-only hashes are useful context but do not constitute a
		// complete reducer hit because they carry no signal measurement.
		return report, true, true
	}
	if rsrpFractionIndex >= 0 {
		report.rsrp, report.direct = decodeLTEFractionalRSRP(int32(words[rsrpIndex]), words[rsrpFractionIndex])
	} else {
		report.rsrp, report.direct = decodeLTERSRP(int32(words[rsrpIndex]))
	}
	if rsrqIndex >= 0 && !validLTERSRQRaw(int32(words[rsrqIndex])) {
		return report, true, false
	}
	return report, true, report.direct
}

func decodeQSHCandidate(hash uint32, words []uint32) (qshCandidate, bool) {
	var candidate qshCandidate
	minimum := 0
	switch hash {
	case 0xd8fc0f04, 0xd8facf74, 0xd8f773e8, 0xd8fc5cc0, 0xd8fb8ad0, 0xd8f82d98, 0xd8f84514:
		minimum = 4
		if len(words) >= minimum {
			candidate.band, candidate.arfcn, candidate.pci = words[0], words[1], words[2]
		}
	case 0xda014184:
		minimum = 6
		if len(words) >= minimum {
			candidate.pci, candidate.arfcn = words[2], words[4]
		}
	case 0xd9fca428, 0xf93fc16a:
		minimum = 7
		if len(words) >= minimum {
			candidate.pci, candidate.arfcn = words[2], words[4]
		}
	case 0xd9fc7a44, 0xf93fc624:
		minimum = 6
		if len(words) >= minimum {
			candidate.pci, candidate.arfcn = words[1], words[3]
		}
	case 0xf9ad1341, 0xbcbafaec:
		minimum = 3
		if hash == 0xf9ad1341 {
			minimum = 5
		}
		if len(words) >= minimum {
			candidate.arfcn, candidate.pci = words[0], words[1]
		}
	case 0xd8f72994, 0xd9877e9c:
		minimum = 5
		if len(words) >= minimum {
			candidate.pci, candidate.arfcn = words[0], words[1]
		}
	case 0xd8f7e394:
		minimum = 4
		if len(words) >= minimum {
			candidate.pci, candidate.arfcn = words[0], words[1]
		}
	default:
		return qshCandidate{}, false
	}
	if len(words) < minimum || candidate.pci > 1007 || candidate.arfcn == 0 || candidate.arfcn > 3279165 || candidate.band > 1024 {
		return qshCandidate{}, false
	}
	return candidate, true
}

func nrReportSpec(hash uint32) (strict bool, minimum int, ok bool) {
	switch hash {
	case 0xda01a364, 0xda016098, 0xf9486c3c:
		return false, 12, true
	case 0xda0539fc:
		return true, 12, true
	case 0xda054a0c:
		return true, 11, true
	case 0xda01af24:
		return true, 11, true
	case 0xda019f14:
		return false, 12, true
	case 0xda06ba1c:
		return true, 11, true
	case 0xda06aa0c:
		return false, 12, true
	default:
		return false, 0, false
	}
}

func validQ7Signal(raw int32) bool {
	value := float64(raw) / 128
	return value >= -160 && value <= -20
}

func decodeLTERSRP(raw int32) (float64, bool) {
	if raw >= -140 && raw <= -30 {
		return float64(raw), true
	}
	if raw >= -1400 && raw <= -300 {
		return roundQSH(float64(raw) / 10), true
	}
	return 0, false
}

func decodeLTEFractionalRSRP(integer int32, fraction uint32) (float64, bool) {
	if integer < -140 || integer > -30 || fraction > 9999 {
		return 0, false
	}
	return roundQSH(float64(integer) - float64(fraction)/10000), true
}

// The LTE layouts that explicitly name an RSRQ word use tenths of a dB on
// this firmware. Keep the check deliberately broad, but reject zero/default
// and sentinel values. Layouts without a verified RSRQ index are not guessed.
func validLTERSRQRaw(raw int32) bool {
	return raw >= -400 && raw < 0
}

func roundQSH(value float64) float64 {
	return math.Floor(value*100+0.5) / 100
}

func unsignedDistance(left, right uint32) uint32 {
	if left >= right {
		return left - right
	}
	return right - left
}
