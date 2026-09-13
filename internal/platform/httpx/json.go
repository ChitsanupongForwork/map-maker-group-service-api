// Package httpx รวมของกลางฝั่ง HTTP ที่ไม่รู้จักฟีเจอร์ไหนเลย
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}

type errorBody struct {
	Error string `json:"error"`
}

// WriteError ตอบ {"error": "..."} — ทุก endpoint ใช้รูปร่างเดียวกัน
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, errorBody{Error: message})
}
