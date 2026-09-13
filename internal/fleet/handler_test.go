package fleet

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// isEpochMs = 13 หลัก ถ้าได้ 10 หลักคือส่งเป็นวินาทีมา ซึ่งผิดสเปก
func isEpochMs(value int64) bool {
	return value >= 1_000_000_000_000 && value < 10_000_000_000_000
}

// TestSnapshotContract ไล่เช็กลิสต์หัวข้อ 11 ของ API-REQUIREMENTS กับ GET /api/fleet
func TestSnapshotContract(t *testing.T) {
	store := NewMemStore(107)
	mux := http.NewServeMux()
	NewHandler(store, NewHub(store)).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/fleet", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "null") {
		t.Fatalf("response contains null: front-end renders values directly")
	}

	var snap Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if !isEpochMs(snap.GeneratedAt) {
		t.Errorf("generatedAt = %d, want 13-digit epoch ms", snap.GeneratedAt)
	}
	if len(snap.Vehicles) != 107 {
		t.Fatalf("len(vehicles) = %d, want 107", len(snap.Vehicles))
	}

	known := func(options []Option) map[string]bool {
		set := map[string]bool{}
		for _, option := range options {
			set[option.Value] = true
		}
		return set
	}
	groups, areas, drivers := known(snap.Groups), known(snap.Areas), known(snap.Drivers)

	seenStatus := map[Status]int{}
	seenID := map[string]bool{}
	for _, v := range snap.Vehicles {
		seenStatus[v.Status]++
		if seenID[v.ID] {
			t.Errorf("duplicate id %s", v.ID)
		}
		seenID[v.ID] = true

		switch v.Status {
		case StatusRunning, StatusEngineOn, StatusParking, StatusOffline:
		default:
			t.Errorf("%s: status %q is not one of the 4 allowed values", v.ID, v.Status)
		}
		if !isEpochMs(v.LastUpdate) {
			t.Errorf("%s: lastUpdate = %d, want 13-digit epoch ms", v.ID, v.LastUpdate)
		}
		if v.HeadingDeg < 0 || v.HeadingDeg > 359 {
			t.Errorf("%s: headingDeg = %d, want 0–359", v.ID, v.HeadingDeg)
		}
		if !groups[v.GroupID] || !areas[v.AreaID] || !drivers[v.DriverID] {
			t.Errorf("%s: groupId/areaId/driverId (%s/%s/%s) missing from options", v.ID, v.GroupID, v.AreaID, v.DriverID)
		}
		if v.Label != v.Make+" "+v.Model {
			t.Errorf("%s: label = %q, want make + model", v.ID, v.Label)
		}
	}
	if len(seenStatus) != 4 {
		t.Errorf("statuses seen = %v, want all 4 so every marker style is visible", seenStatus)
	}
}

func TestStreamStartsWithFullSnapshot(t *testing.T) {
	store := NewMemStore(5)
	mux := http.NewServeMux()
	NewHandler(store, NewHub(store)).Register(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/fleet/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	reader := bufio.NewReader(res.Body)
	lines := make([]string, 0, 4)
	for len(lines) < 4 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v (got %q)", err, lines)
		}
		lines = append(lines, strings.TrimRight(line, "\n"))
	}

	if lines[0] != "retry: 3000" || lines[2] != "event: snapshot" || !strings.HasPrefix(lines[3], `data: {"generatedAt":`) {
		t.Fatalf("first lines = %q, want retry, blank, event: snapshot, data: {...}", lines)
	}
}

type sequenceStore struct {
	snapshots []Snapshot
	calls     int
}

func (s *sequenceStore) Snapshot(context.Context) (Snapshot, error) {
	snap := s.snapshots[min(s.calls, len(s.snapshots)-1)]
	s.calls++
	return snap, nil
}

func TestHubBroadcastsPatchThenSnapshot(t *testing.T) {
	options := []Option{{Value: "g-1", Label: "กลุ่ม"}}
	snapshotOf := func(vehicles ...Vehicle) Snapshot {
		return Snapshot{GeneratedAt: 1_789_765_430_000, Groups: options, Areas: options, Drivers: options, Vehicles: vehicles}
	}
	store := &sequenceStore{snapshots: []Snapshot{
		snapshotOf(Vehicle{ID: "v-0001", SpeedKph: 30}),
		snapshotOf(Vehicle{ID: "v-0001", SpeedKph: 45}),
		snapshotOf(Vehicle{ID: "v-0001", SpeedKph: 45}),
		snapshotOf(Vehicle{ID: "v-0001", SpeedKph: 45}, Vehicle{ID: "v-0002"}),
	}}
	hub := NewHub(store)
	ctx := context.Background()

	hub.refresh(ctx) // รอบแรก: เก็บไว้เทียบ ยังไม่มีอะไรจะส่ง
	current, messages, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	if current == nil || len(current.Vehicles) != 1 {
		t.Fatalf("Subscribe current = %+v, want first snapshot", current)
	}

	hub.refresh(ctx) // ความเร็วเปลี่ยน → patch
	want := "event: patch\ndata: {\"patches\":[{\"id\":\"v-0001\",\"speedKph\":45}]}\n\n"
	if got := string(<-messages); got != want {
		t.Fatalf("message 1\n got  %q\n want %q", got, want)
	}

	hub.refresh(ctx) // ไม่มีอะไรเปลี่ยน → ต้องเงียบ
	hub.refresh(ctx) // มีรถเพิ่ม → patch อธิบายไม่ได้ ต้องส่งก้อนเต็ม
	if got := string(<-messages); !strings.HasPrefix(got, "event: snapshot\n") {
		t.Fatalf("message 2 = %q, want a full snapshot event (unchanged round must send nothing)", got)
	}
}
