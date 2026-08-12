package service

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const (
	neighborGoParserEngine  = "go-native"
	neighborGoParserVersion = "1.2.1"
	neighborMaxDiagWords    = 236
)

type qmdlCandidate struct {
	seq   uint32
	file  uint32
	pci   uint32
	arfcn uint32
	band  uint32
}

type qmdlReport struct {
	seq           uint32
	file          uint32
	rat           uint32
	pci           uint32
	arfcn         uint32
	band          uint32
	rsrp          float64
	direct        bool
	requiresARFCN bool
}

type qmdlAggregateKey struct {
	rat   uint32
	pci   uint32
	arfcn uint32
}

type qmdlAggregate struct {
	key        qmdlAggregateKey
	band       uint32
	bandClash  bool
	samples    int
	directHits int
	firstSeq   uint32
	lastSeq    uint32
	hasSeq     bool
	rsrp       []float64
}

type qmdlParser struct {
	frames     int
	malformed  int
	candidates []qmdlCandidate
	reports    []qmdlReport
	serving    []nativeServingCell

	hasCurrentARFCN bool
	currentARFCN    uint32
}

func parseNeighborQMDLFiles(ctx context.Context, paths []string) (*nativeNeighborResult, error) {
	if len(paths) == 0 {
		return nil, os.ErrNotExist
	}

	parser := &qmdlParser{}
	opened := 0
	var lastOpenErr error
	for fileIndex, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		stream, err := os.Open(path)
		if err != nil {
			lastOpenErr = err
			continue
		}
		opened++
		err = parser.parseReader(ctx, stream, uint32(fileIndex))
		closeErr := stream.Close()
		if err != nil {
			return nil, fmt.Errorf("读取 QMDL %s 失败: %w", filepath.Base(path), err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("关闭 QMDL %s 失败: %w", filepath.Base(path), closeErr)
		}
	}
	if opened == 0 {
		return nil, fmt.Errorf("打开 QMDL 失败（文件可能已轮转）: %w", lastOpenErr)
	}
	return parser.result(), nil
}

func (parser *qmdlParser) parseReader(ctx context.Context, source io.Reader, fileIndex uint32) error {
	reader := bufio.NewReaderSize(source, 64*1024)
	frame := make([]byte, 0, 1024)
	escaped := false
	seq := uint32(0)
	parser.hasCurrentARFCN = false
	parser.currentARFCN = 0

	finishFrame := func() {
		if len(frame) == 0 {
			escaped = false
			return
		}
		if escaped || len(frame) <= 2 {
			parser.malformed++
		} else {
			parser.parseFrame(frame[:len(frame)-2], seq, fileIndex)
			parser.frames++
			seq++
		}
		frame = frame[:0]
		escaped = false
	}

	for bytesRead := 0; ; bytesRead++ {
		if bytesRead&0xfff == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		value, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				finishFrame()
				return nil
			}
			return err
		}
		if value == 0x7e {
			finishFrame()
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

func (parser *qmdlParser) parseFrame(payload []byte, seq, fileIndex uint32) {
	if len(payload) <= 15 || payload[0] != 0x9d {
		return
	}
	wordCount := int(payload[4]) - 0x13
	if wordCount < 0 || wordCount > neighborMaxDiagWords || len(payload) < (wordCount+4)*4 {
		return
	}
	logID := binary.LittleEndian.Uint32(payload[12:16])
	words := make([]uint32, wordCount)
	for index := range words {
		words[index] = binary.LittleEndian.Uint32(payload[16+index*4:])
	}

	if candidate, ok := decodeQMDLCandidate(logID, words); ok {
		candidate.seq = seq
		candidate.file = fileIndex
		parser.candidates = append(parser.candidates, candidate)
	}

	switch logID {
	case 0xdcdda754:
		if len(words) >= 5 {
			parser.currentARFCN = words[4]
			parser.hasCurrentARFCN = true
		}
	case 0xd9b9c660:
		if len(words) >= 2 && words[0] > 100000 {
			parser.currentARFCN = words[0]
			parser.hasCurrentARFCN = true
		}
	case 0xdcdda85c:
		if len(words) >= 5 {
			parser.addServing(seq, words)
		}
	}

	strict, minimum, isNR := qmdlNRReportSpec(logID)
	if isNR {
		if len(words) >= minimum {
			parser.addNRReport(seq, fileIndex, words, strict)
		}
		return
	}

	if report, ok := decodeQMDLLTEReport(logID, words); ok {
		report.seq = seq
		report.file = fileIndex
		parser.reports = append(parser.reports, report)
	}
}

// decodeQMDLLTEReport supports both the hashes used by u60nbrqt-native 0.7.1
// and the hashes present in the U60Pro SDX75 QShrink 4 database. QShrink hashes
// change when Qualcomm rebuilds the modem image even when the format string and
// argument layout stay the same.
func decodeQMDLLTEReport(logID uint32, words []uint32) (qmdlReport, bool) {
	var arfcn, pci uint32
	rsrpIndex := -1
	rsrpFractionIndex := -1
	requireValid := false

	switch logID {
	case 0xd936846c: // Legacy: EARFCN, PCI.
		if len(words) < 2 {
			return qmdlReport{}, false
		}
		arfcn, pci = words[0], words[1]
	case 0xd8fe54a0: // Legacy: EARFCN, PCI, RSRP, ...
		if len(words) < 4 {
			return qmdlReport{}, false
		}
		arfcn, pci, rsrpIndex = words[0], words[1], 2
	case 0xd8fea29c: // Cell, EARFCN, PCI, RSRP, RSRQ, elapsed_ms.
		if len(words) < 6 {
			return qmdlReport{}, false
		}
		arfcn, pci, rsrpIndex = words[1], words[2], 3
	case 0xd8f02d94, 0xd8fea210: // LTE index, valid, EARFCN, PCI, RSRP, RSRQ.
		if len(words) < 6 {
			return qmdlReport{}, false
		}
		arfcn, pci, rsrpIndex, requireValid = words[2], words[3], 4, true
	case 0xd939607c: // NBR CELLS MEAS: EARFCN, PCI.
		if len(words) < 2 {
			return qmdlReport{}, false
		}
		arfcn, pci = words[0], words[1]
	case 0xd93960a0, 0xd8fea258: // EARFCN, PCI, RSRP, RSRQ, elapsed_ms.
		if len(words) < 5 {
			return qmdlReport{}, false
		}
		arfcn, pci, rsrpIndex = words[0], words[1], 2
	case 0xd94f5690: // SDX75: EARFCN, PCI, RSRP integer, RSRP fraction, ...
		if len(words) < 6 {
			return qmdlReport{}, false
		}
		arfcn, pci, rsrpIndex, rsrpFractionIndex = words[0], words[1], 2, 3
	default:
		return qmdlReport{}, false
	}

	if requireValid && words[1] == 0 {
		return qmdlReport{}, false
	}
	if arfcn == 0 || arfcn > 262143 || pci > 503 {
		return qmdlReport{}, false
	}
	report := qmdlReport{rat: 1, pci: pci, arfcn: arfcn}
	if rsrpIndex >= 0 {
		if rsrpFractionIndex >= 0 {
			report.rsrp, report.direct = decodeLTEFractionalRSRP(int32(words[rsrpIndex]), words[rsrpFractionIndex])
		} else {
			report.rsrp, report.direct = decodeLTERSRP(int32(words[rsrpIndex]))
		}
	}
	return report, true
}

func decodeQMDLCandidate(logID uint32, words []uint32) (qmdlCandidate, bool) {
	var candidate qmdlCandidate
	var minimum int
	switch logID {
	case 0xd8fc0f04, 0xd8facf74, 0xd8f773e8, 0xd8fc5cc0,
		0xd8fb8ad0, 0xd8f82d98, 0xd8f84514:
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
		if logID == 0xf9ad1341 {
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
		return qmdlCandidate{}, false
	}
	if len(words) < minimum || candidate.pci > 1007 || candidate.arfcn == 0 || candidate.arfcn > 3279165 || candidate.band > 1024 {
		return qmdlCandidate{}, false
	}
	return candidate, true
}

func qmdlNRReportSpec(logID uint32) (strict bool, minimum int, ok bool) {
	switch logID {
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

func (parser *qmdlParser) addNRReport(seq, fileIndex uint32, words []uint32, requiresARFCN bool) {
	if words[3] > 1007 || words[5] > 1007 {
		return
	}
	rsrpRaw := int32(words[4])
	if !validQ7Signal(rsrpRaw) || !validQ7Signal(int32(words[6])) {
		return
	}
	parser.reports = append(parser.reports, qmdlReport{
		seq: seq, file: fileIndex, pci: words[3], rsrp: roundQMDL(float64(rsrpRaw) / 128),
		direct: true, requiresARFCN: requiresARFCN,
	})
}

func (parser *qmdlParser) addServing(seq uint32, words []uint32) {
	cell := nativeServingCell{
		Seq:     int(seq),
		GCI:     int64(int32(words[0])),
		PCI:     int(words[1]),
		RSRPDBM: floatPointer(float64(int32(words[2])) / 10),
		RSRQDB:  floatPointer(float64(int32(words[3])) / 10),
		SINRDB:  floatPointer(float64(int32(words[4])) / 10),
	}
	if parser.hasCurrentARFCN {
		cell.ARFCN = intPointer(parser.currentARFCN)
	}
	parser.serving = append(parser.serving, cell)
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
		return roundQMDL(float64(raw) / 10), true
	}
	return 0, false
}

func decodeLTEFractionalRSRP(integer int32, fraction uint32) (float64, bool) {
	if integer < -140 || integer > -30 || fraction > 9999 {
		return 0, false
	}
	return roundQMDL(float64(integer) - float64(fraction)/10000), true
}

func roundQMDL(value float64) float64 {
	return math.Floor(value*100+0.5) / 100
}

func (parser *qmdlParser) result() *nativeNeighborResult {
	parser.correlateNRReports()
	aggregates := make([]qmdlAggregate, 0)
	positions := make(map[qmdlAggregateKey]int)
	getAggregate := func(key qmdlAggregateKey) *qmdlAggregate {
		if position, ok := positions[key]; ok {
			return &aggregates[position]
		}
		positions[key] = len(aggregates)
		aggregates = append(aggregates, qmdlAggregate{key: key})
		return &aggregates[len(aggregates)-1]
	}

	for _, report := range parser.reports {
		if report.rat == 0 && report.requiresARFCN && report.arfcn == 0 {
			continue
		}
		aggregate := getAggregate(qmdlAggregateKey{rat: report.rat, pci: report.pci, arfcn: report.arfcn})
		aggregate.samples++
		aggregate.addSeq(report.seq)
		if report.direct {
			aggregate.rsrp = append(aggregate.rsrp, report.rsrp)
		}
	}
	for _, candidate := range parser.candidates {
		aggregate := getAggregate(qmdlAggregateKey{pci: candidate.pci, arfcn: candidate.arfcn})
		aggregate.directHits++
		aggregate.addSeq(candidate.seq)
		if candidate.band == 0 {
			continue
		}
		if aggregate.band == 0 {
			aggregate.band = candidate.band
		} else if aggregate.band != candidate.band {
			aggregate.bandClash = true
		}
	}

	neighbors := make([]nativeNeighborCell, 0, len(aggregates))
	for _, aggregate := range aggregates {
		samples := aggregate.samples
		if samples == 0 {
			// The native tool reports candidate hits as samples when no measurement
			// report exists for this identity.
			samples = aggregate.directHits
		}
		cell := nativeNeighborCell{
			RAT: qmdlRATName(aggregate.key.rat), PCI: int(aggregate.key.pci),
			Samples: samples, DirectHits: aggregate.directHits,
			FirstSeq: int(aggregate.firstSeq), LastSeq: int(aggregate.lastSeq),
		}
		if aggregate.key.arfcn != 0 {
			cell.ARFCN = intPointer(aggregate.key.arfcn)
		}
		if aggregate.band != 0 && !aggregate.bandClash {
			cell.Band = intPointer(aggregate.band)
		}
		validRSRP := make([]float64, 0, len(aggregate.rsrp))
		for _, value := range aggregate.rsrp {
			if !math.IsNaN(value) && value >= -140 && value <= -30 {
				validRSRP = append(validRSRP, value)
			}
		}
		cell.PlausibleSamples = len(validRSRP)
		if len(validRSRP) != 0 {
			sort.Float64s(validRSRP)
			middle := len(validRSRP) / 2
			median := validRSRP[middle]
			if len(validRSRP)%2 == 0 {
				median = roundQMDL((validRSRP[middle-1] + median) / 2)
			}
			cell.RSRPMedian = floatPointer(median)
		}
		neighbors = append(neighbors, cell)
	}

	return &nativeNeighborResult{
		Engine: neighborGoParserEngine, Version: neighborGoParserVersion,
		Frames: parser.frames, Malformed: parser.malformed,
		Serving: parser.serving, Neighbors: neighbors,
	}
}

func (parser *qmdlParser) correlateNRReports() {
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
				best = candidateIndex
				bestDistance = distance
			}
		}
		if best >= 0 {
			report.arfcn = parser.candidates[best].arfcn
			report.band = parser.candidates[best].band
		}
	}
}

func (aggregate *qmdlAggregate) addSeq(seq uint32) {
	if !aggregate.hasSeq {
		aggregate.firstSeq = seq
		aggregate.lastSeq = seq
		aggregate.hasSeq = true
		return
	}
	if seq < aggregate.firstSeq {
		aggregate.firstSeq = seq
	}
	if seq > aggregate.lastSeq {
		aggregate.lastSeq = seq
	}
}

func unsignedDistance(left, right uint32) uint32 {
	if left >= right {
		return left - right
	}
	return right - left
}

func qmdlRATName(rat uint32) string {
	if rat == 0 {
		return "NR"
	}
	return "LTE"
}

func intPointer(value uint32) *int {
	converted := int(value)
	return &converted
}

func floatPointer(value float64) *float64 {
	return &value
}
