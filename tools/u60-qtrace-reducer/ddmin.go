package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

type repeatedResult struct {
	Passed  bool        `json:"passed"`
	Hits    int         `json:"hits"`
	Runs    int         `json:"runs"`
	Records []runRecord `json:"records"`
}

type calibrationResult struct {
	Target          targetKind  `json:"target"`
	Runs            int         `json:"runs"`
	Hits            int         `json:"hits"`
	LatenciesMS     []int64     `json:"latencies_ms"`
	P95MS           int64       `json:"p95_ms"`
	CandidateWindow int64       `json:"candidate_window_ms"`
	Passed          bool        `json:"passed"`
	FailureReason   string      `json:"failure_reason,omitempty"`
	Records         []runRecord `json:"records"`
}

type ddminOptions struct {
	Runner             *deviceOptions
	Target             targetKind
	ControlsReport     string
	CalibrationRuns    int
	PositiveMinimumHit int
	CandidateRepeats   int
	CandidateMinHits   int
	WindowMargin       time.Duration
	MinimumWindow      time.Duration
	MaximumWindow      time.Duration
}

type ddminSummary struct {
	Stage                  string            `json:"stage"`
	Target                 targetKind        `json:"target"`
	Started                time.Time         `json:"started"`
	Completed              time.Time         `json:"completed"`
	OriginalFrameIDs       []int             `json:"original_frame_ids"`
	MinimalFrameIDs        []int             `json:"minimal_frame_ids"`
	MinimalSSIDs           []uint16          `json:"minimal_ssids"`
	PrivateCommandState    string            `json:"private_command_state"`
	ControlsReport         string            `json:"controls_report"`
	Calibration            calibrationResult `json:"calibration"`
	EmptyCandidate         repeatedResult    `json:"empty_candidate"`
	MinimalValidation      repeatedResult    `json:"minimal_validation"`
	EndPositiveControl     repeatedResult    `json:"end_positive_control"`
	Volume                 volumeComparison  `json:"volume"`
	OutputConfig           string            `json:"output_config"`
	OutputConfigFrameCount int               `json:"output_config_frame_count"`
	Notes                  []string          `json:"notes"`
}

type volumeComparison struct {
	ReferenceWindowMS     int64   `json:"reference_window_ms"`
	MinimalWindowMS       int64   `json:"minimal_window_ms"`
	WindowsComparable     bool    `json:"windows_comparable"`
	OriginalReference     string  `json:"original_reference"`
	MinimalReference      string  `json:"minimal_reference"`
	OriginalMessageFrames int     `json:"original_message_frames"`
	MinimalMessageFrames  int     `json:"minimal_message_frames"`
	FrameReductionPercent float64 `json:"message_frame_reduction_percent"`
	OriginalAvgQSHFrames  float64 `json:"original_avg_qsh_frames"`
	MinimalAvgQSHFrames   float64 `json:"minimal_avg_qsh_frames"`
	QSHFrameReduction     float64 `json:"qsh_frame_reduction_percent"`
	OriginalAvgQSHBytes   float64 `json:"original_avg_qsh_bytes"`
	MinimalAvgQSHBytes    float64 `json:"minimal_avg_qsh_bytes"`
	QSHByteReduction      float64 `json:"qsh_byte_reduction_percent"`
}

func (options ddminOptions) run(ctx context.Context) (*ddminSummary, error) {
	if options.Runner == nil {
		return nil, errors.New("ddmin runner is nil")
	}
	if err := validateControlsReport(options.ControlsReport, options.Target); err != nil {
		return nil, err
	}
	if options.CalibrationRuns <= 0 {
		options.CalibrationRuns = 10
	}
	if options.PositiveMinimumHit <= 0 {
		options.PositiveMinimumHit = options.CalibrationRuns
	}
	if options.CandidateRepeats <= 0 {
		options.CandidateRepeats = 3
	}
	if options.CandidateMinHits <= 0 {
		options.CandidateMinHits = 2
	}
	allIDs := options.Runner.Config.allMessageFrameIDs()
	summary := &ddminSummary{
		Stage: "frame", Target: options.Target, Started: time.Now().UTC(),
		OriginalFrameIDs: append([]int(nil), allIDs...), PrivateCommandState: string(privateBoth),
		ControlsReport: options.ControlsReport,
		Notes: []string{
			"This is whole-frame ddmin only; SSID and mask-bit minimization have not yet been run.",
			"Both private 0x4b commands are resent for every candidate; their necessity is not inferred here.",
		},
	}
	calibration, err := options.calibrate(ctx, allIDs)
	summary.Calibration = calibration
	if err != nil {
		return summary, err
	}
	window := time.Duration(calibration.CandidateWindow) * time.Millisecond
	emptyCandidate, err := options.runRepeated(ctx, "frame-ddmin", "empty-message-set", nil, privateBoth, options.CandidateRepeats, options.CandidateMinHits, true, window)
	summary.EmptyCandidate = emptyCandidate
	if err != nil {
		return summary, err
	}
	var current []int
	if emptyCandidate.Passed {
		current = []int{}
		summary.Notes = append(summary.Notes, "No 0x7d/0x04 message frame was required under this batch's observed state; interpret together with controls and private-command persistence.")
	} else {
		current, err = ddmin(allIDs, func(candidate []int) (bool, error) {
			result, runErr := options.runRepeated(ctx, "frame-ddmin", "", candidate, privateBoth, options.CandidateRepeats, options.CandidateMinHits, true, window)
			if runErr != nil {
				return false, runErr
			}
			return result.Passed, nil
		})
		if err != nil {
			return summary, err
		}
	}
	summary.MinimalFrameIDs = current
	summary.MinimalSSIDs = options.Runner.Config.allSSIDs(current)
	minimalValidation, err := options.runRepeated(ctx, "frame-ddmin-validation", "minimal", current, privateBoth, options.CandidateRepeats, options.CandidateMinHits, true, window)
	summary.MinimalValidation = minimalValidation
	if err != nil {
		return summary, err
	}
	if !minimalValidation.Passed {
		return summary, errors.New("final minimal candidate failed repeated validation")
	}
	endControl, err := options.runRepeated(ctx, "positive-control", "batch-end", allIDs, privateBoth, options.CandidateRepeats, options.CandidateMinHits, true, window)
	summary.EndPositiveControl = endControl
	if err != nil {
		return summary, err
	}
	if !endControl.Passed {
		return summary, errors.New("batch-end positive control failed; all candidate conclusions in this batch are invalid")
	}
	frames, err := options.Runner.Config.selectFrames(current, false, privateBoth)
	if err != nil {
		return summary, err
	}
	name := fmt.Sprintf("%s-neighbor-frame-min.cfg", options.Target)
	if options.Target == targetCombined {
		name = "cell-neighbor-combined-frame-min.cfg"
	}
	path := options.Runner.Reports.dir + "/" + name
	if err := writeFileAtomic(path, encodeConfig(frames), 0600); err != nil {
		return summary, err
	}
	summary.OutputConfig = path
	summary.OutputConfigFrameCount = len(frames)
	// Compare equal-duration captures. Calibration intentionally uses a longer
	// window to establish P95, so comparing its raw volume with the shorter
	// candidate window would manufacture a false reduction. The batch-end
	// positive control uses the same derived window as minimal validation.
	summary.Volume = compareVolumes(len(allIDs), len(current), endControl.Records, minimalValidation.Records)
	summary.Completed = time.Now().UTC()
	if err := options.Runner.Reports.WriteSummary(fmt.Sprintf("ddmin-%s-summary.json", options.Target), summary); err != nil {
		return summary, err
	}
	return summary, nil
}

func (options ddminOptions) calibrate(ctx context.Context, allIDs []int) (calibrationResult, error) {
	result := calibrationResult{Target: options.Target, Runs: options.CalibrationRuns}
	repeated, err := options.runRepeated(ctx, "positive-control", "batch-start-calibration", allIDs, privateBoth, options.CalibrationRuns, options.PositiveMinimumHit, false, options.MaximumWindow)
	result.Hits = repeated.Hits
	result.Records = repeated.Records
	for _, record := range repeated.Records {
		if record.Result && record.FirstHitLatencyMS != nil {
			result.LatenciesMS = append(result.LatenciesMS, *record.FirstHitLatencyMS)
		}
	}
	if err != nil {
		result.FailureReason = err.Error()
		return result, err
	}
	if repeated.Hits < options.PositiveMinimumHit || len(result.LatenciesMS) == 0 {
		result.FailureReason = fmt.Sprintf("positive control hit %d/%d; need at least %d", repeated.Hits, repeated.Runs, options.PositiveMinimumHit)
		return result, errors.New(result.FailureReason)
	}
	sort.Slice(result.LatenciesMS, func(i, j int) bool { return result.LatenciesMS[i] < result.LatenciesMS[j] })
	index := int(math.Ceil(0.95*float64(len(result.LatenciesMS)))) - 1
	if index < 0 {
		index = 0
	}
	result.P95MS = result.LatenciesMS[index]
	window := time.Duration(result.P95MS)*time.Millisecond + options.WindowMargin
	if window < options.MinimumWindow {
		window = options.MinimumWindow
	}
	if options.MaximumWindow > 0 && window > options.MaximumWindow {
		window = options.MaximumWindow
	}
	result.CandidateWindow = window.Milliseconds()
	result.Passed = true
	return result, nil
}

func (options ddminOptions) runRepeated(
	ctx context.Context,
	stage, control string,
	frameIDs []int,
	private privateMode,
	runs, minimumHits int,
	positiveOK bool,
	window time.Duration,
) (repeatedResult, error) {
	result := repeatedResult{Runs: runs}
	for attempt := 1; attempt <= runs; attempt++ {
		record, err := options.Runner.testCandidate(ctx, stage, control, options.Target, frameIDs, false, private, string(private), attempt, positiveOK, window)
		result.Records = append(result.Records, record)
		if record.Result {
			result.Hits++
		}
		if err != nil {
			return result, err
		}
		remaining := runs - attempt
		if result.Hits+remaining < minimumHits {
			break
		}
	}
	result.Passed = result.Hits >= minimumHits
	return result, nil
}

// ddmin implements Zeller-style delta debugging. Both subsets and
// complements are tested, so cross-frame dependencies are retained instead of
// being lost by a simple left/right binary search. Candidate order is stable.
func ddmin(items []int, test func([]int) (bool, error)) ([]int, error) {
	current := append([]int(nil), items...)
	if len(current) == 0 {
		return current, nil
	}
	n := 2
	for len(current) >= 2 {
		partitions := partitionStable(current, n)
		reduced := false
		for _, subset := range partitions {
			passed, err := test(subset)
			if err != nil {
				return nil, err
			}
			if passed {
				current = append([]int(nil), subset...)
				if n > 2 {
					n--
				}
				reduced = true
				break
			}
		}
		if reduced {
			continue
		}
		for _, subset := range partitions {
			complement := stableComplement(current, subset)
			if len(complement) == 0 {
				continue
			}
			passed, err := test(complement)
			if err != nil {
				return nil, err
			}
			if passed {
				current = complement
				if n > 2 {
					n--
				}
				reduced = true
				break
			}
		}
		if reduced {
			continue
		}
		if n >= len(current) {
			break
		}
		n *= 2
		if n > len(current) {
			n = len(current)
		}
	}
	return current, nil
}

func partitionStable(items []int, parts int) [][]int {
	if parts > len(items) {
		parts = len(items)
	}
	if parts < 1 {
		parts = 1
	}
	result := make([][]int, 0, parts)
	base, extra := len(items)/parts, len(items)%parts
	offset := 0
	for index := 0; index < parts; index++ {
		size := base
		if index < extra {
			size++
		}
		result = append(result, append([]int(nil), items[offset:offset+size]...))
		offset += size
	}
	return result
}

func stableComplement(items, remove []int) []int {
	counts := make(map[int]int, len(remove))
	for _, value := range remove {
		counts[value]++
	}
	result := make([]int, 0, len(items)-len(remove))
	for _, value := range items {
		if counts[value] > 0 {
			counts[value]--
			continue
		}
		result = append(result, value)
	}
	return result
}

func compareVolumes(originalFrames, minimalFrames int, originalRuns, minimalRuns []runRecord) volumeComparison {
	originalQSHFrames, originalQSHBytes := averageVolume(originalRuns)
	minimalQSHFrames, minimalQSHBytes := averageVolume(minimalRuns)
	comparison := volumeComparison{
		ReferenceWindowMS:     commonCaptureDuration(originalRuns),
		MinimalWindowMS:       commonCaptureDuration(minimalRuns),
		OriginalReference:     "batch-end-positive-control",
		MinimalReference:      "minimal-validation",
		OriginalMessageFrames: originalFrames,
		MinimalMessageFrames:  minimalFrames,
		OriginalAvgQSHFrames:  originalQSHFrames,
		MinimalAvgQSHFrames:   minimalQSHFrames,
		OriginalAvgQSHBytes:   originalQSHBytes,
		MinimalAvgQSHBytes:    minimalQSHBytes,
	}
	comparison.WindowsComparable = comparison.ReferenceWindowMS > 0 && comparison.ReferenceWindowMS == comparison.MinimalWindowMS
	comparison.FrameReductionPercent = reductionPercent(float64(originalFrames), float64(minimalFrames))
	if comparison.WindowsComparable {
		comparison.QSHFrameReduction = reductionPercent(originalQSHFrames, minimalQSHFrames)
		comparison.QSHByteReduction = reductionPercent(originalQSHBytes, minimalQSHBytes)
	}
	return comparison
}

func commonCaptureDuration(records []runRecord) int64 {
	if len(records) == 0 {
		return 0
	}
	duration := records[0].CaptureDurationMS
	for _, record := range records[1:] {
		if record.CaptureDurationMS != duration {
			return 0
		}
	}
	return duration
}

func averageVolume(records []runRecord) (frames, bytes float64) {
	if len(records) == 0 {
		return 0, 0
	}
	for _, record := range records {
		frames += float64(record.QSHFrameCount)
		bytes += float64(record.QSHTotalBytes)
	}
	return frames / float64(len(records)), bytes / float64(len(records))
}

func reductionPercent(original, reduced float64) float64 {
	if original <= 0 {
		return 0
	}
	return math.Round((1-reduced/original)*10000) / 100
}
