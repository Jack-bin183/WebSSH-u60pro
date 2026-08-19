package main

import "time"

const (
	toolName    = "u60-nr-crossband-probe"
	toolVersion = "0.1.0"
	stateSchema = 1
)

type experimentID string

const (
	experimentA experimentID = "A"
	experimentB experimentID = "B"
	experimentC experimentID = "C"
	experimentD experimentID = "D"
	experimentE experimentID = "E"
	experimentF experimentID = "F"
)

type targetSpec struct {
	Band  uint32  `json:"band"`
	ARFCN uint32  `json:"arfcn"`
	PCI   *uint32 `json:"pci,omitempty"`
}

type networkSnapshot struct {
	CapturedAt      time.Time      `json:"captured_at"`
	Raw             map[string]any `json:"raw"`
	NetSelect       string         `json:"net_select"`
	NetworkType     string         `json:"network_type"`
	NRBandSA        string         `json:"nr_band_sa"`
	NRBandNSA       string         `json:"nr_band_nsa"`
	NRCellLockRaw   string         `json:"nr_cell_lock_raw"`
	NRCellLockPCI   string         `json:"nr_cell_lock_pci"`
	NRCellLockARFCN string         `json:"nr_cell_lock_arfcn"`
	NRCellLockBand  string         `json:"nr_cell_lock_band"`
	ServingBand     string         `json:"serving_band"`
	ServingARFCN    string         `json:"serving_arfcn"`
	ServingPCI      string         `json:"serving_pci"`
}

type recoveryState struct {
	Schema                 int             `json:"schema"`
	ToolVersion            string          `json:"tool_version"`
	RunID                  string          `json:"run_id"`
	Experiment             experimentID    `json:"experiment"`
	Target                 targetSpec      `json:"target"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	Status                 string          `json:"status"`
	Stage                  string          `json:"stage"`
	Snapshot               networkSnapshot `json:"snapshot"`
	CompletedActions       []string        `json:"completed_actions"`
	RestoreAttempts        int             `json:"restore_attempts"`
	TraceRestoreRequired   bool            `json:"trace_restore_required"`
	NetworkRestoreRequired bool            `json:"network_restore_required"`
	ProductionQTraceBase64 string          `json:"production_qtrace_base64,omitempty"`
	DiagMDLog              string          `json:"diag_mdlog,omitempty"`
	WorkDir                string          `json:"work_dir,omitempty"`
	StopGraceMS            int64           `json:"stop_grace_ms,omitempty"`
	LastError              string          `json:"last_error,omitempty"`
}

type qmiEvent struct {
	At          time.Time `json:"at"`
	Service     string    `json:"service,omitempty"`
	Event       string    `json:"event,omitempty"`
	Direction   string    `json:"direction"`
	Kind        string    `json:"kind"`
	MessageID   string    `json:"message_id"`
	Transaction uint16    `json:"transaction,omitempty"`
	Caller      string    `json:"caller,omitempty"`
	Raw         string    `json:"raw,omitempty"`
	Result      *uint16   `json:"result,omitempty"`
	Error       *uint16   `json:"error,omitempty"`
	ScanStatus  *uint32   `json:"scan_status,omitempty"`
	Description string    `json:"description,omitempty"`
}

type actionEvent struct {
	At       time.Time      `json:"at"`
	Action   string         `json:"action"`
	Request  map[string]any `json:"request,omitempty"`
	Response map[string]any `json:"response,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type ml1Result struct {
	ObservedAt   *time.Time `json:"observed_at,omitempty"`
	Hash         string     `json:"hash"`
	PCI          uint32     `json:"pci"`
	ARFCN        uint32     `json:"arfcn"`
	Band         uint32     `json:"band"`
	Valid        uint32     `json:"valid"`
	RSRP         float64    `json:"rsrp"`
	RSRPInteger  int32      `json:"rsrp_integer"`
	SINR         *float64   `json:"sinr,omitempty"`
	SINRInteger  *int32     `json:"sinr_integer,omitempty"`
	RSRQ         *float64   `json:"rsrq,omitempty"`
	SearchEnergy *int32     `json:"search_energy,omitempty"`
	Words        []uint32   `json:"words,omitempty"`
}

type resourceSample struct {
	At                time.Time `json:"at"`
	MemAvailableBytes uint64    `json:"mem_available_bytes"`
	WorkFreeBytes     uint64    `json:"work_free_bytes"`
	CaptureBytes      int64     `json:"capture_bytes"`
}

type qshEvidence struct {
	Hash  string   `json:"hash"`
	Kind  string   `json:"kind"`
	ARFCN *uint32  `json:"arfcn,omitempty"`
	PCI   *uint32  `json:"pci,omitempty"`
	Words []uint32 `json:"words"`
}

type experimentRecord struct {
	Schema              int              `json:"schema"`
	ToolVersion         string           `json:"tool_version"`
	RunID               string           `json:"run_id"`
	Experiment          experimentID     `json:"experiment"`
	Description         string           `json:"description"`
	StartedAt           time.Time        `json:"started_at"`
	FinishedAt          time.Time        `json:"finished_at"`
	ServingBefore       networkSnapshot  `json:"serving_before"`
	ServingAfterRestore *networkSnapshot `json:"serving_after_restore,omitempty"`
	Target              targetSpec       `json:"target"`
	QMIEvents           []qmiEvent       `json:"qmi_events,omitempty"`
	Actions             []actionEvent    `json:"actions,omitempty"`
	ML1Results          []ml1Result      `json:"ml1_results,omitempty"`
	QSHEvidence         []qshEvidence    `json:"qsh_evidence,omitempty"`
	FirstTargetHitMS    *int64           `json:"first_target_hit_ms,omitempty"`
	QSHFrameCount       int              `json:"qsh_frame_count"`
	ActiveHashCount     int              `json:"active_hash_count"`
	MalformedFrameCount int              `json:"malformed_frame_count"`
	QMDLBytes           int64            `json:"qmdl_bytes"`
	CaptureDurationMS   int64            `json:"capture_duration_ms"`
	NetworkInterrupted  bool             `json:"network_interrupted"`
	RestoreDurationMS   int64            `json:"restore_duration_ms"`
	RestoreSucceeded    bool             `json:"restore_succeeded"`
	Result              string           `json:"result"`
	FailureReason       string           `json:"failure_reason,omitempty"`
	QCRILPathVerified   bool             `json:"qcril_path_verified"`
	QCRILPathEvidence   string           `json:"qcril_path_evidence,omitempty"`
	QMITracePath        string           `json:"qmi_trace_path,omitempty"`
	QMITraceComplete    bool             `json:"qmi_trace_complete"`
	QMITraceError       string           `json:"qmi_trace_error,omitempty"`
	ResourceSamples     []resourceSample `json:"resource_samples,omitempty"`
	CaptureDirectory    string           `json:"capture_directory,omitempty"`
}

type dryRunManifest struct {
	ToolVersion                 string           `json:"tool_version"`
	SourceConfig                string           `json:"source_config"`
	SourceSHA256                string           `json:"source_sha256"`
	SourceBytes                 int              `json:"source_bytes"`
	ActiveConfig                string           `json:"active_config"`
	ActiveSHA256                string           `json:"active_sha256"`
	ActiveBytes                 int              `json:"active_bytes"`
	QSHID                       uint16           `json:"qsh_id"`
	OriginalMask                string           `json:"original_mask"`
	ActiveMask                  string           `json:"active_mask"`
	RoundTripValid              bool             `json:"round_trip_valid"`
	Target                      targetSpec       `json:"target"`
	Manual0085Raw               string           `json:"manual_0x0085_raw"`
	QCRILOneShotExpectedRaw     string           `json:"qcril_one_shot_expected_raw"`
	QCRILOneShotCarriesChannel  bool             `json:"qcril_one_shot_carries_channel"`
	QCRILAdvancedCarriesChannel bool             `json:"qcril_advanced_carries_channel"`
	SetChannelsQMIMessage       string           `json:"set_channels_qmi_message"`
	SetChannelsCarriesChannel   bool             `json:"set_channels_carries_channel"`
	Experiments                 []experimentPlan `json:"experiments"`
}

type experimentPlan struct {
	ID                  experimentID `json:"id"`
	ControlPath         string       `json:"control_path"`
	NetworkDisruption   bool         `json:"network_disruption"`
	RequiresQCRILDriver bool         `json:"requires_qcril_driver"`
	SuccessCondition    string       `json:"success_condition"`
	Limitation          string       `json:"limitation,omitempty"`
}
