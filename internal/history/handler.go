package history

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"fleet-monitor-server/internal/platform/httpx"
)

// maxRange คือช่วงเวลากว้างสุดที่ยอมให้ขอ — หน้าเว็บดูย้อนหลังได้ไม่เกิน 1 เดือน (สเปกหัวข้อ 5)
const maxRange = 31 * 24 * time.Hour

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/vehicles/{id}/history", h.history)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.PathValue("id")
	query := r.URL.Query()

	from, err := parseEpochMs(query.Get("from"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "from "+err.Error())
		return
	}
	to, err := parseEpochMs(query.Get("to"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "to "+err.Error())
		return
	}
	if !from.Before(to) {
		httpx.WriteError(w, http.StatusBadRequest, "from must be earlier than to")
		return
	}
	if to.Sub(from) > maxRange {
		httpx.WriteError(w, http.StatusBadRequest, "range must not exceed 31 days")
		return
	}

	samples, err := h.store.Samples(r.Context(), vehicleID, from, to)
	if errors.Is(err, ErrVehicleNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "vehicle not found")
		return
	}
	if err != nil {
		log.Printf("GET /api/vehicles/%s/history: %v", vehicleID, err)
		httpx.WriteError(w, http.StatusInternalServerError, "could not load history")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, Build(samples, MaxPoints))
}

// parseEpochMs รับ epoch milliseconds 13 หลักเท่านั้น
// ตัวเลข 10 หลักเกือบแน่นอนว่าคนส่งใส่เป็นวินาทีมา — ตอบ 400 บอกไปตรง ๆ ดีกว่าคืนช่วงเวลาปี 1970
func parseEpochMs(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("is required (epoch milliseconds)")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, errors.New("must be an integer (epoch milliseconds)")
	}
	if value < 100_000_000_000 || value >= 10_000_000_000_000 {
		return time.Time{}, errors.New("must be epoch milliseconds (13 digits), not seconds")
	}
	return time.UnixMilli(value), nil
}
