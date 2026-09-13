package history

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serveHistory(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(NewMemStore(107)).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHistoryRejectsBadRange(t *testing.T) {
	now := time.Now().UnixMilli()
	tests := map[string]string{
		"missing from":     fmt.Sprintf("/api/vehicles/v-0001/history?to=%d", now),
		"seconds not ms":   fmt.Sprintf("/api/vehicles/v-0001/history?from=%d&to=%d", now/1000-3600, now/1000),
		"not a number":     "/api/vehicles/v-0001/history?from=yesterday&to=today",
		"from after to":    fmt.Sprintf("/api/vehicles/v-0001/history?from=%d&to=%d", now, now-60_000),
		"longer than 31 d": fmt.Sprintf("/api/vehicles/v-0001/history?from=%d&to=%d", now-32*24*3_600_000, now),
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			if rec := serveHistory(t, target); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestHistoryUnknownVehicle(t *testing.T) {
	now := time.Now().UnixMilli()
	rec := serveHistory(t, fmt.Sprintf("/api/vehicles/v-9999/history?from=%d&to=%d", now-3_600_000, now))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHistoryFullMonthStaysUnderPointLimit(t *testing.T) {
	now := time.Now().UnixMilli()
	rec := serveHistory(t, fmt.Sprintf("/api/vehicles/v-0001/history?from=%d&to=%d", now-31*24*3_600_000, now))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	var result Result
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Points) == 0 || len(result.Points) > MaxPoints {
		t.Fatalf("len(points) = %d, want 1–%d", len(result.Points), MaxPoints)
	}
	for i := 1; i < len(result.Points); i++ {
		if result.Points[i].Timestamp <= result.Points[i-1].Timestamp {
			t.Fatalf("points not in time order at %d", i)
		}
	}
	for _, event := range result.Events {
		if event.PointIndex < 0 || event.PointIndex >= len(result.Points) {
			t.Fatalf("event %s pointIndex %d out of range", event.ID, event.PointIndex)
		}
	}
	if result.MovingMinutes == 0 || result.StoppedMinutes == 0 || result.MaxSpeedKph == 0 || result.DistanceKm == 0 {
		t.Errorf("summary = %+v, want a month of demo data to include both driving and parking", result.Summary)
	}
}
