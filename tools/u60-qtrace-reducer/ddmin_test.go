package main

import "testing"

func TestDDMinPreservesCombinationDependency(t *testing.T) {
	items := make([]int, 23)
	for index := range items {
		items[index] = index
	}
	tests := 0
	result, err := ddmin(items, func(candidate []int) (bool, error) {
		tests++
		return containsInt(candidate, 3) && containsInt(candidate, 18), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !equalInts(result, []int{3, 18}) {
		t.Fatalf("ddmin result=%v, want [3 18]", result)
	}
	if tests == 0 {
		t.Fatal("test function was not called")
	}
}

func TestDDMinRetainsOriginalOrder(t *testing.T) {
	items := []int{8, 2, 17, 4, 21}
	result, err := ddmin(items, func(candidate []int) (bool, error) {
		return containsInt(candidate, 2) && containsInt(candidate, 21), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !equalInts(result, []int{2, 21}) {
		t.Fatalf("ddmin result=%v", result)
	}
}

func TestVolumeComparisonUsesEqualCandidateWindows(t *testing.T) {
	original := []runRecord{
		{CaptureDurationMS: 3000, QSHFrameCount: 300, QSHTotalBytes: 3000},
		{CaptureDurationMS: 3000, QSHFrameCount: 330, QSHTotalBytes: 3300},
	}
	minimal := []runRecord{
		{CaptureDurationMS: 3000, QSHFrameCount: 150, QSHTotalBytes: 1500},
		{CaptureDurationMS: 3000, QSHFrameCount: 180, QSHTotalBytes: 1800},
	}
	comparison := compareVolumes(23, 2, original, minimal)
	if !comparison.WindowsComparable || comparison.ReferenceWindowMS != 3000 || comparison.MinimalWindowMS != 3000 {
		t.Fatalf("unexpected window comparison: %+v", comparison)
	}
	if comparison.QSHFrameReduction <= 0 || comparison.QSHByteReduction <= 0 {
		t.Fatalf("expected positive equal-window reduction: %+v", comparison)
	}

	minimal[1].CaptureDurationMS = 5000
	comparison = compareVolumes(23, 2, original, minimal)
	if comparison.WindowsComparable || comparison.QSHFrameReduction != 0 || comparison.QSHByteReduction != 0 {
		t.Fatalf("mismatched windows must not report a reduction: %+v", comparison)
	}
}

func containsInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
