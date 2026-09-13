package history

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDownsampleKeepsLimitEndsAndRequiredPoints(t *testing.T) {
	keep := []int{3, 12_345, 99_999}

	got := Downsample(100_000, 5_000, keep)

	if len(got) > 5_000 {
		t.Fatalf("len = %d, want ≤ 5000", len(got))
	}
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Fatal("indices must be sorted and unique")
	}
	for _, index := range append([]int{0}, keep...) {
		if _, found := slices.BinarySearch(got, index); !found {
			t.Errorf("index %d missing", index)
		}
	}
}

func TestDownsampleSmallInputUnchanged(t *testing.T) {
	if got := Downsample(4, 5_000, nil); !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Fatalf("Downsample(4) = %v, want all indices", got)
	}
}

// หลังลดจุด pointIndex ของ event ต้องชี้ไปที่จุดเดิมใน points ชุดใหม่
func TestBuildRemapsEventPointIndex(t *testing.T) {
	samples := make([]Sample, 20_000)
	for i := range samples {
		ignitionOn := i < 10_001 || i > 10_100 // จอดดับเครื่องช่วงกลาง
		speed := 30
		if !ignitionOn {
			speed = 0
		}
		samples[i] = sample(float64(i)/2, speed, 13.8+float64(i)*0.00001, 100.5, ignitionOn)
	}

	result := Build(samples, 1_000)

	if len(result.Points) > 1_000 {
		t.Fatalf("len(points) = %d, want ≤ 1000", len(result.Points))
	}
	if len(result.Events) != 4 {
		t.Fatalf("events = %+v, want start, stop, moving, end", result.Events)
	}
	for _, event := range result.Events {
		if result.Points[event.PointIndex].Timestamp != event.Timestamp {
			t.Errorf("%s event points at %d whose timestamp differs from the event", event.Type, event.PointIndex)
		}
	}
	// ค่าสรุปคิดจากจุดดิบทั้งหมด ไม่ใช่จุดที่เหลือหลังลด
	if want := Summarize(samples); result.Summary != want {
		t.Errorf("summary = %+v, want %+v (from raw samples)", result.Summary, want)
	}
}

func TestBuildEmptyHasNoNull(t *testing.T) {
	body, err := json.Marshal(Build(nil, MaxPoints))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "null") {
		t.Fatalf("JSON = %s, must not contain null", body)
	}
}
