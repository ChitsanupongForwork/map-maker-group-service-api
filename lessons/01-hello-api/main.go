package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// message คือรูปแบบข้อมูลที่จะถูกแปลงเป็น JSON ส่งกลับไปหา client
type message struct {
	Text string `json:"message"`
}

type intro struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Go API is running! BBB"))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	response := message{Text: "Hello, Go API! AAA"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// ใส่ paramitor ใส่ endpoint แบบ get
func ParamitorHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	response := message{
		Text: "hello" + " " + name,
	}

	json.NewEncoder(w).Encode(response)
}

// ใส่ body json แบบ post นำมาแสดงใส่ response
func Postbodyjson(w http.ResponseWriter, r *http.Request) {
	var input intro
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(message{
			Text: "รูปแบบ JSON ไม่ถูกต้อง",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	response := message{
		Text: "Body from json is : " + input.Name + strconv.Itoa(input.Age),
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /111", homeHandler)
	mux.HandleFunc("GET /hello", helloHandler)
	mux.HandleFunc("GET /introduction", ParamitorHandler)
	mux.HandleFunc("POST /introductionPost", Postbodyjson)

	address := ":8081"
	log.Printf("server is running at http://localhost%s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
