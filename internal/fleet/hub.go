package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"
)

const (
	pollInterval = 3 * time.Second
	// client ที่รับไม่ทันจนค้างเกินนี้จะถูกตัดสาย แล้วเบราว์เซอร์จะต่อใหม่เอง
	clientBuffer = 16
)

// Hub อ่าน Snapshot จาก Store ทุก 3 วินาทีด้วย goroutine ตัวเดียว (ไม่ใช่ตัวละ client)
// แล้วกระจายเฉพาะส่วนที่เปลี่ยนให้ทุกคนที่เปิด /api/fleet/stream อยู่
//
// เพราะอ่านจาก snapshot ทั้งกอง จึงไม่พลาดกรณีไหนเลย — รถออกจากอุโมงค์,
// job ตั้ง offline, ข้อมูลเข้าช้า ทุกอย่างสะท้อนอยู่ใน snapshot รอบถัดไปเอง
type Hub struct {
	store Store

	mu      sync.Mutex
	current *Snapshot // nil = ยังโหลดรอบแรกไม่เสร็จ
	clients map[chan []byte]struct{}
}

func NewHub(store Store) *Hub {
	return &Hub{store: store, clients: make(map[chan []byte]struct{})}
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		h.refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Subscribe คืน snapshot ล่าสุด พร้อมช่องรับข้อความถัดไป ภายใต้ล็อกครั้งเดียว
// ทำให้ไม่มี patch หลุดหรือซ้ำระหว่าง "ส่งก้อนเต็ม" กับ "เริ่มรับ patch"
func (h *Hub) Subscribe() (current *Snapshot, messages <-chan []byte, unsubscribe func()) {
	client := make(chan []byte, clientBuffer)

	h.mu.Lock()
	h.clients[client] = struct{}{}
	current = h.current
	h.mu.Unlock()

	unsubscribe = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.clients[client]; ok {
			delete(h.clients, client)
			close(client)
		}
	}
	return current, client, unsubscribe
}

func (h *Hub) refresh(ctx context.Context) {
	next, err := h.store.Snapshot(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("fleet hub: %v", err)
		}
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	prev := h.current
	h.current = &next
	if prev == nil || len(h.clients) == 0 {
		return
	}

	var message []byte
	patches, sameFleet := Diff(prev.Vehicles, next.Vehicles)
	switch {
	case !sameFleet || !sameOptions(*prev, next):
		// มีรถเพิ่ม/หาย หรือตัวเลือกใน dropdown เปลี่ยน — patch อธิบายไม่ได้ ส่งก้อนเต็ม
		message, err = encodeEvent("snapshot", next)
	case len(patches) > 0:
		message, err = encodeEvent("patch", patchEvent{Patches: patches})
	default:
		return
	}
	if err != nil {
		log.Printf("fleet hub encode: %v", err)
		return
	}

	for client := range h.clients {
		select {
		case client <- message:
		default:
			// คิวเต็ม = client ช้าเกินไป ตัดสายดีกว่าข้าม patch ไปเงียบ ๆ จนข้อมูลบนจอเพี้ยน
			// เบราว์เซอร์จะต่อใหม่เองแล้วได้ก้อนเต็ม (สเปกหัวข้อ 4)
			delete(h.clients, client)
			close(client)
		}
	}
}

type patchEvent struct {
	Patches []Patch `json:"patches"`
}

func sameOptions(a, b Snapshot) bool {
	return slices.Equal(a.Groups, b.Groups) &&
		slices.Equal(a.Areas, b.Areas) &&
		slices.Equal(a.Drivers, b.Drivers)
}

// encodeEvent จัดรูปข้อความ SSE หนึ่งก้อน — json.Marshal ไม่มีขึ้นบรรทัดใหม่ จึงใส่ใน data: บรรทัดเดียวได้
func encodeEvent(name string, data any) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return fmt.Appendf(nil, "event: %s\ndata: %s\n\n", name, payload), nil
}
