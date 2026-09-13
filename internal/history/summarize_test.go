package history

import "testing"

func sample(minute float64, speed int, lat, lng float64, ignitionOn bool) Sample {
	return Sample{
		Point:      Point{Lat: lat, Lng: lng, Timestamp: int64(minute * 60_000), SpeedKph: speed},
		IgnitionOn: ignitionOn,
	}
}

func TestSummarizeStoppedTime(t *testing.T) {
	samples := []Sample{
		sample(0, 40, 13.800, 100.500, true),
		sample(1, 0, 13.806, 100.500, true),  // จอดตั้งแต่นาทีที่ 1 หลังวิ่งมา ≈ 0.67 กม.
		sample(3, 30, 13.806, 100.500, true), // ออกตัวนาทีที่ 3
	}

	got := Summarize(samples)

	want := Summary{DistanceKm: 0.7, MovingMinutes: 1, StoppedMinutes: 2, MaxSpeedKph: 40, AverageSpeedKph: 40}
	if got != want {
		t.Fatalf("Summarize = %+v, want %+v", got, want)
	}
}

func TestSummarizeIgnoresGPSJitterWhileParked(t *testing.T) {
	samples := []Sample{
		sample(0, 0, 13.80000, 100.50000, false),
		sample(1, 0, 13.80010, 100.50012, false), // พิกัดแกว่งเองตอนจอด
		sample(2, 0, 13.79995, 100.49990, false),
	}

	got := Summarize(samples)

	if got.DistanceKm != 0 || got.StoppedMinutes != 2 || got.MovingMinutes != 0 || got.AverageSpeedKph != 0 {
		t.Fatalf("Summarize = %+v, want 0 km, 2 stopped minutes, no movement", got)
	}
}

func TestSummarizeAverageIsTimeWeighted(t *testing.T) {
	samples := []Sample{
		sample(0, 20, 13.80, 100.50, true), // 20 กม./ชม. นาน 1 นาที
		sample(1, 80, 13.80, 100.50, true), // 80 กม./ชม. นาน 3 นาที
		sample(4, 0, 13.80, 100.50, true),
	}

	// (20·1 + 80·3) / 4 = 65 ไม่ใช่ค่าเฉลี่ยตรง ๆ ของสองจุด (50)
	if got := Summarize(samples).AverageSpeedKph; got != 65 {
		t.Fatalf("AverageSpeedKph = %d, want 65", got)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if got := Summarize(nil); got != (Summary{}) {
		t.Fatalf("Summarize(nil) = %+v, want zero", got)
	}
}
