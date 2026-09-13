package fleet

import (
	"encoding/json"
	"testing"
)

func TestDiffSendsOnlyChangedFields(t *testing.T) {
	prev := []Vehicle{
		{ID: "v-0001", Status: StatusRunning, SpeedKph: 40, Lat: 13.8, LastUpdate: 1_789_765_430_000},
		{ID: "v-0002", Status: StatusParking, SpeedKph: 0, Lat: 13.7, LastUpdate: 1_789_765_430_000},
	}
	next := []Vehicle{
		// รถหยุด: ความเร็วเปลี่ยนเป็น 0 ต้องยังโผล่ใน patch ไม่หายไปเพราะ omitempty
		{ID: "v-0001", Status: StatusEngineOn, SpeedKph: 0, Lat: 13.8, LastUpdate: 1_789_765_433_000},
		// ไม่มีอะไรเปลี่ยน ต้องไม่ถูกส่ง
		{ID: "v-0002", Status: StatusParking, SpeedKph: 0, Lat: 13.7, LastUpdate: 1_789_765_430_000},
	}

	patches, sameFleet := Diff(prev, next)
	if !sameFleet {
		t.Fatal("sameFleet = false, want true")
	}
	if len(patches) != 1 {
		t.Fatalf("len(patches) = %d, want 1", len(patches))
	}

	body, err := json.Marshal(patches[0])
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"v-0001","status":"engine-on","speedKph":0,"lastUpdate":1789765433000}`
	if string(body) != want {
		t.Fatalf("patch JSON\n got  %s\n want %s", body, want)
	}
}

func TestDiffNoChangesGivesEmptySlice(t *testing.T) {
	fleet := []Vehicle{{ID: "v-0001", SpeedKph: 10}}

	patches, sameFleet := Diff(fleet, fleet)
	if !sameFleet || patches == nil || len(patches) != 0 {
		t.Fatalf("Diff(same) = %v, %v; want [], true", patches, sameFleet)
	}
}

func TestDiffDetectsDifferentFleet(t *testing.T) {
	tests := map[string]struct{ prev, next []Vehicle }{
		"vehicle added":   {prev: []Vehicle{{ID: "v-0001"}}, next: []Vehicle{{ID: "v-0001"}, {ID: "v-0002"}}},
		"vehicle removed": {prev: []Vehicle{{ID: "v-0001"}, {ID: "v-0002"}}, next: []Vehicle{{ID: "v-0001"}}},
		"vehicle swapped": {prev: []Vehicle{{ID: "v-0001"}}, next: []Vehicle{{ID: "v-0009"}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, sameFleet := Diff(tt.prev, tt.next); sameFleet {
				t.Fatal("sameFleet = true, want false")
			}
		})
	}
}
