package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

//go:embed web/index.html web/style.css web/script.js web/manifest.json web/sw.js web/offline.html web/icons/*
var staticFiles embed.FS

type State struct {
	Hue        float64 `json:"hue"`
	Saturation float64 `json:"saturation"`
	Value      float64 `json:"value"`
	IPAddress  string  `json:"ipAddress"`
}

var (
	mu           sync.RWMutex
	currentState = State{
		Hue:        0,
		Saturation: 0,
		Value:      0,
		IPAddress:  "192.168.112.189",
	}
)

func getState() State {
	mu.RLock()
	defer mu.RUnlock()
	return currentState
}

func setState(state State) {
	mu.Lock()
	defer mu.Unlock()
	currentState = state
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

type RGB struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

type Request struct {
	Address    string `json:"address"`
	On         *bool  `json:"on"`
	Color      *RGB   `json:"color"`
	Brightness *int   `json:"brightness"`
}

func rgbToHsl(r, g, b int) (h, s, v float64) {
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}

	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}

	v = float64(max) / 255.0

	if max == min {
		h = 0
		s = 0
		return
	}

	d := float64(max - min)
	s = d / float64(max)

	switch max {
	case r:
		h = float64(g-b) / d
		if g < b {
			h += 6
		}
	case g:
		h = float64(b-r)/d + 2
	case b:
		h = float64(r-g)/d + 4
	}

	h /= 6
	return h * 360, s * 100, v
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

const tcpPort = 5577

func sendRaw(ip string, bytes []byte) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, tcpPort), 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to send: %w", err)
	}

	return nil
}

func calculateChecksum(data []byte) byte {
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return byte(sum & 0xFF)
}

func cmdPowerOn(ip string) error {
	data := []byte{0x71, 0x23, 0x0F}
	checksum := calculateChecksum(data)
	data = append(data, checksum)
	return sendRaw(ip, data)
}

func cmdPowerOff(ip string) error {
	data := []byte{0x71, 0x24, 0x0F}
	checksum := calculateChecksum(data)
	data = append(data, checksum)
	return sendRaw(ip, data)
}

func setColor(ip string, r, g, b int, brightness *int) error {
	if brightness != nil && *brightness > 0 {
		scale := float64(*brightness) / 100.0
		r = int(float64(r) * scale)
		g = int(float64(g) * scale)
		b = int(float64(b) * scale)
	}

	data := []byte{0x31, byte(r), byte(g), byte(b), 0x00, 0x0F, 0x0F}
	checksum := calculateChecksum(data)
	data = append(data, checksum)
	return sendRaw(ip, data)
}

func handleLED(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{Status: "error", Message: "Method not allowed"})
		return
	}

	var req Request
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Status: "error", Message: "Invalid JSON"})
		return
	}

	if req.Address == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Status: "error", Message: "Address required"})
		return
	}

	var errors []string

	if req.On != nil && *req.On {
		if err := cmdPowerOn(req.Address); err != nil {
			errors = append(errors, fmt.Sprintf("power on failed: %v", err))
		}
	}

	if req.On != nil && !*req.On {
		if err := cmdPowerOff(req.Address); err != nil {
			errors = append(errors, fmt.Sprintf("power off failed: %v", err))
		}
		state := getState()
		state.Value = 0
		setState(state)
	}

	if req.Color != nil {
		if err := setColor(req.Address, req.Color.R, req.Color.G, req.Color.B, req.Brightness); err != nil {
			errors = append(errors, fmt.Sprintf("color set failed: %v", err))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(errors) > 0 {
		w.WriteHeader(http.StatusInternalServerError)
		message := ""
		for i, e := range errors {
			if i > 0 {
				message += "; "
			}
			message += e
		}
		json.NewEncoder(w).Encode(Response{Status: "error", Message: message})
		return
	}

	json.NewEncoder(w).Encode(Response{Status: "success", Message: "Commands sent"})

	if req.Color != nil {
		hue, sat, val := rgbToHsl(req.Color.R, req.Color.G, req.Color.B)
		setState(State{
			Hue:        hue,
			Saturation: sat,
			Value:      val,
			IPAddress:  req.Address,
		})
	}
}

func handleStateGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{Status: "error", Message: "Method not allowed"})
		return
	}

	state := getState()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func handleStatePOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{Status: "error", Message: "Method not allowed"})
		return
	}

	var state State
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&state); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Status: "error", Message: "Invalid JSON"})
		return
	}

	if state.IPAddress != "" {
		setState(state)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Status: "success", Message: "State updated"})
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	filePath := "web" + path

	file, err := staticFiles.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := "application/octet-stream"
	switch {
	case path == "/index.html":
		contentType = "text/html; charset=utf-8"
	case path == "/style.css":
		contentType = "text/css; charset=utf-8"
	case path == "/script.js":
		contentType = "application/javascript; charset=utf-8"
	case path == "/manifest.json":
		contentType = "application/manifest+json"
	case path == "/sw.js":
		contentType = "application/javascript; charset=utf-8"
	case path == "/offline.html":
		contentType = "text/html; charset=utf-8"
	case path == "/icons/128.png":
		contentType = "image/png"
	case path == "/icons/512.png":
		contentType = "image/png"
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5002"
	}

	bindAddr := os.Getenv("BIND_ADDRESS")
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}

	state := getState()
	if state.IPAddress != "" && state.Value == 0 {
		if err := cmdPowerOff(state.IPAddress); err != nil {
			log.Printf("Warning: Failed to send power off on startup: %v", err)
		} else {
			log.Printf("Sent power off command to %s on startup", state.IPAddress)
		}
	}

	http.HandleFunc("/api/led", corsMiddleware(handleLED))
	http.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleStateGET(w, r)
		} else if r.Method == http.MethodPost {
			handleStatePOST(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{Status: "error", Message: "Method not allowed"})
		}
	})
	http.HandleFunc("/", serveStaticFiles)

	addr := bindAddr + ":" + port
	log.Printf("LED server starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
