package ingest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"fleet-monitor-server/internal/platform/httpx"
)

const (
	maxBatch     = 1000
	maxBodyBytes = 1 << 20 // 1 MB พอสำหรับหลายพันจุด
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/positions", h.create)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	positions, err := decodePositions(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(positions) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "no positions in request body")
		return
	}
	if len(positions) > maxBatch {
		httpx.WriteError(w, http.StatusBadRequest, fmt.Sprintf("at most %d positions per request", maxBatch))
		return
	}

	now := time.Now()
	for i, position := range positions {
		if err := position.Validate(now); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, fmt.Sprintf("positions[%d]: %v", i, err))
			return
		}
	}

	result, err := h.store.Write(r.Context(), positions)
	if errors.Is(err, ErrUnknownVehicle) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		log.Printf("POST /api/positions: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "could not store positions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

// decodePositions รับได้ทั้งจุดเดียว {...} และหลายจุด [{...}, ...]
// อุปกรณ์ที่ขาดสัญญาณไปนานจะส่งจุดที่ค้างไว้มาเป็นก้อนเดียว
func decodePositions(body io.Reader) ([]Position, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, errors.New("request body is too large or unreadable")
	}

	decode := func(target any) error {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields() // พิมพ์ชื่อฟิลด์ผิด เช่น "heading" จะรู้ทันที ไม่เงียบกลายเป็น 0
		if err := decoder.Decode(target); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		return nil
	}

	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '[' {
		var positions []Position
		if err := decode(&positions); err != nil {
			return nil, err
		}
		return positions, nil
	}

	var position Position
	if err := decode(&position); err != nil {
		return nil, err
	}
	return []Position{position}, nil
}
