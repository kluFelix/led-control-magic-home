package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

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
}

func main() {
	http.HandleFunc("/api/led", corsMiddleware(handleLED))

	log.Println("LED server starting on :3000")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
