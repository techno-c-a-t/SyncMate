package main

import (
	"log"
	"net/http"
	"os"

	"syncmate/server/internal/auth"
	"syncmate/server/internal/handler"
	"syncmate/server/internal/realtime"
	"syncmate/server/internal/room"
	"syncmate/server/internal/weather"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		secretKey = "fa7116b0727655ba8f57cd865d6e8409"
	}

	tokenManager := auth.NewTokenManager(secretKey)
	roomManager := room.NewRoomManager()
	weatherService := weather.NewWeatherService()

	h := handler.NewHandler(tokenManager, roomManager, weatherService)
	hub := realtime.NewHub(roomManager, tokenManager)

	// REST API Routes
	http.HandleFunc("/api/v1/auth/register", h.Register)
	http.HandleFunc("/api/v1/auth/login", h.Login)
	http.HandleFunc("/api/v1/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			h.UpdateProfile(w, r)
		} else {
			h.GetProfile(w, r)
		}
	})

	http.HandleFunc("/api/v1/rooms", h.CreateRoom)
	http.HandleFunc("/api/v1/rooms/join/", h.JoinRoomByCode)

	http.HandleFunc("/api/v1/weather", h.GetWeather)

	// WebSocket Real-time Endpoint
	http.HandleFunc("/ws", hub.HandleWebSocket)

	log.Printf("SyncMate Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
