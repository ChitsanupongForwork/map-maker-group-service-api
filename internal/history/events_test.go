package history

import "testing"

type eventWant struct {
	eventType  EventType
	pointIndex int
	title      string
}

func assertEvents(t *testing.T, got []Event, want []eventWant) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].Type != want[i].eventType || got[i].PointIndex != want[i].pointIndex {
			t.Errorf("event %d = %s@%d, want %s@%d", i, got[i].Type, got[i].PointIndex, want[i].eventType, want[i].pointIndex)
		}
		if want[i].title != "" && got[i].Title != want[i].title {
			t.Errorf("event %d title = %q, want %q", i, got[i].Title, want[i].title)
		}
	}
}

func TestBuildEventsUsesIgnitionNotSpeed(t *testing.T) {
	samples := []Sample{
		sample(0, 30, 13.80, 100.50, true),
		sample(1, 40, 13.81, 100.50, true),
		sample(2, 0, 13.82, 100.50, true), // ติดไฟแดง: ความเร็ว 0 แต่เครื่องยังติด → ไม่ใช่ "จอด"
		sample(3, 35, 13.82, 100.50, true),
		sample(4, 0, 13.83, 100.50, false), // ดับเครื่อง → จอด
		sample(10, 0, 13.83, 100.50, false),
		sample(16, 0, 13.83, 100.50, true), // ติดเครื่อง รวมจอด 12 นาที
		sample(17, 20, 13.84, 100.50, true),
		sample(18, 25, 13.85, 100.50, true),
	}

	assertEvents(t, BuildEvents(samples), []eventWant{
		{EventStart, 0, "เริ่มต้นเส้นทาง"},
		{EventStop, 4, "จอดรถ 12 นาที"},
		{EventMoving, 7, "เดินทางต่อ"},
		{EventEnd, 8, "สิ้นสุดเส้นทาง"},
	})
}

func TestBuildEventsDepartureFromParked(t *testing.T) {
	samples := []Sample{
		sample(0, 0, 13.80, 100.50, false),
		sample(1, 0, 13.80, 100.50, true), // ติดเครื่องอุ่นรถ
		sample(2, 15, 13.81, 100.50, true),
		sample(3, 30, 13.82, 100.50, true),
	}

	assertEvents(t, BuildEvents(samples), []eventWant{
		{EventStart, 0, ""},
		{EventMoving, 2, "ออกเดินทาง"},
		{EventEnd, 3, ""},
	})
}

func TestBuildEventsStillParkedAtEnd(t *testing.T) {
	samples := []Sample{
		sample(0, 30, 13.80, 100.50, true),
		sample(5, 0, 13.81, 100.50, false),
		sample(95, 0, 13.81, 100.50, false),
	}

	assertEvents(t, BuildEvents(samples), []eventWant{
		{EventStart, 0, ""},
		{EventStop, 1, "จอดรถ 1 ชม. 30 นาที"},
		{EventEnd, 2, ""},
	})
}

func TestBuildEventsDetailFallsBackToCoordinates(t *testing.T) {
	events := BuildEvents([]Sample{sample(0, 0, 13.801234, 100.554119, false)})

	if len(events) != 1 || events[0].Detail != "13.80123, 100.55412" {
		t.Fatalf("events = %+v, want one start event with coordinates as detail", events)
	}
}

func TestBuildEventsEmpty(t *testing.T) {
	if got := BuildEvents(nil); got == nil || len(got) != 0 {
		t.Fatalf("BuildEvents(nil) = %#v, want non-nil empty slice (nil becomes JSON null)", got)
	}
}
