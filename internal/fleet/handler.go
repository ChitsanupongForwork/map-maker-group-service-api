package fleet

import (
	"log"
	"net/http"
	"time"

	"fleet-monitor-server/internal/platform/httpx"
)

const heartbeatInterval = 20 * time.Second

type Handler struct {
	store Store
	hub   *Hub
}

func NewHandler(store Store, hub *Hub) *Handler {
	return &Handler{store: store, hub: hub}
}

// Register ≈ การที่ src/app ของ front-end ดึงฟีเจอร์มาวางในหน้า
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fleet", h.snapshot)
	mux.HandleFunc("GET /api/fleet/stream", h.stream)
}

// snapshot ส่งรถครบทุกคันในก้อนเดียว ไม่มี query param สำหรับกรอง (สเปกหัวข้อ 8)
// ตัวเลขบนชิปอย่าง "Running 48" ต้องนับจากกองเต็มเสมอ
func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := h.store.Snapshot(r.Context())
	if err != nil {
		log.Printf("GET /api/fleet: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "could not load fleet")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, snap)
}

// stream คือ SSE: ต่อเข้ามาได้ event "snapshot" ก้อนเต็ม 1 ครั้ง แล้วตามด้วย event "patch"
// เบราว์เซอร์ต่อใหม่เองเมื่อสายหลุด และทุกครั้งที่ต่อใหม่จะได้ก้อนเต็มก่อนเสมอ
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	current, messages, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	if current == nil {
		// server เพิ่งเริ่ม Hub ยังโหลดรอบแรกไม่เสร็จ — อ่านจาก store ตรง ๆ แทน
		snap, err := h.store.Snapshot(r.Context())
		if err != nil {
			log.Printf("GET /api/fleet/stream: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "could not load fleet")
			return
		}
		current = &snap
	}
	first, err := encodeEvent("snapshot", current)
	if err != nil {
		log.Printf("GET /api/fleet/stream encode: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "could not encode fleet")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	// retry: บอกเบราว์เซอร์ให้ต่อใหม่ภายใน 3 วินาทีเมื่อสายหลุด
	if !send(w, rc, append([]byte("retry: 3000\n\n"), first...)) {
		return
	}

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done(): // ผู้ใช้ปิดแท็บ หรือ server กำลังปิด
			return
		case message, open := <-messages:
			if !open { // Hub ตัดสายเพราะรับไม่ทัน
				return
			}
			if !send(w, rc, message) {
				return
			}
			heartbeat.Reset(heartbeatInterval)
		case <-heartbeat.C:
			// comment ของ SSE เบราว์เซอร์ไม่สนใจ แต่กันตัวกลาง (proxy / load balancer) ตัดสายตอนเงียบ
			if !send(w, rc, []byte(": heartbeat\n\n")) {
				return
			}
		}
	}
}

func send(w http.ResponseWriter, rc *http.ResponseController, message []byte) bool {
	if _, err := w.Write(message); err != nil {
		return false
	}
	return rc.Flush() == nil
}
