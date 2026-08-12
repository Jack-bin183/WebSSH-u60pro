package service

import "testing"

func TestParseNeighborBand(t *testing.T) {
	tests := map[string]int{
		"n78":        78,
		"N41":        41,
		"B3":         3,
		"LTE_BAND_3": 3,
		"Band 7":     7,
	}
	for input, want := range tests {
		got, ok := parseNeighborBand(input)
		if !ok || got != want {
			t.Fatalf("parseNeighborBand(%q) = %d, %v; want %d, true", input, got, ok, want)
		}
	}
	if _, ok := parseNeighborBand("--"); ok {
		t.Fatal("parseNeighborBand(\"--\") unexpectedly succeeded")
	}
}

func TestNeighborResultFromUbusNSA(t *testing.T) {
	snapshot := neighborUbusSnapshot{
		"network_type":        "5G NSA",
		"network_provider":    "测试运营商",
		"lte_pci":             "101",
		"wan_active_band":     "LTE_BAND_3",
		"wan_active_channel":  "1650",
		"lte_rsrp":            "-96.5",
		"lte_rsrq":            "-11",
		"lte_snr":             "14.2",
		"nr5g_pci":            "208",
		"nr5g_action_band":    "n41",
		"nr5g_action_channel": "504990",
		"nr5g_rsrp":           "-100.9",
		"nr5g_rsrq":           "--",
		"nr5g_snr":            "--",
	}
	result := neighborResultFromUbus(snapshot)
	if len(result.Serving) != 2 {
		t.Fatalf("serving count = %d; want 2", len(result.Serving))
	}
	if result.Serving[0].RAT != "LTE" || result.Serving[0].Band == nil || *result.Serving[0].Band != 3 {
		t.Fatalf("unexpected LTE serving cell: %+v", result.Serving[0])
	}
	if result.Serving[1].RAT != "NR" || result.Serving[1].Band == nil || *result.Serving[1].Band != 41 {
		t.Fatalf("unexpected NR serving cell: %+v", result.Serving[1])
	}
	if result.Serving[1].RSRQ != nil || result.Serving[1].SINR != nil {
		t.Fatalf("unknown NR metrics must remain nil: %+v", result.Serving[1])
	}
}

func TestAdaptNativeServingMetrics(t *testing.T) {
	arfcn := 504990
	band := 41
	rsrp := -100.9
	rsrq := -12.5
	sinr := 8.25
	native := &nativeNeighborResult{
		Engine: "u60nbrqt-native", Version: "0.7.1-multi",
		Serving:   []nativeServingCell{{PCI: 208, ARFCN: &arfcn, RSRPDBM: &rsrp, RSRQDB: &rsrq, SINRDB: &sinr}},
		Neighbors: []nativeNeighborCell{{RAT: "NR", PCI: 936, ARFCN: &arfcn, Band: &band, RSRPMedian: &rsrp, Samples: 4}},
	}
	snapshot := neighborUbusSnapshot{
		"network_type":        "5G SA",
		"nr5g_pci":            "208",
		"nr5g_action_band":    "n41",
		"nr5g_action_channel": "504990",
	}
	result := adaptNativeNeighborResult(native, snapshot)
	if result.Source != "parser" || len(result.Serving) != 1 || len(result.Neighbors) != 1 {
		t.Fatalf("unexpected result shape: %+v", result)
	}
	serving := result.Serving[0]
	if serving.RAT != "NR" || serving.Band == nil || *serving.Band != 41 || serving.RSRPMedian == nil || *serving.RSRPMedian != rsrp {
		t.Fatalf("native serving fields were not preserved: %+v", serving)
	}
}

func TestAdaptNativeServingInfersNRWithoutUbus(t *testing.T) {
	arfcn := 504990
	native := &nativeNeighborResult{
		Engine:  "u60nbrqt-native",
		Version: "0.7.1-multi",
		Serving: []nativeServingCell{{PCI: 208, ARFCN: &arfcn}},
	}
	result := adaptNativeNeighborResult(native, nil)
	if len(result.Serving) != 1 || result.Serving[0].RAT != "NR" {
		t.Fatalf("high ARFCN serving cell should be inferred as NR: %+v", result.Serving)
	}
}
