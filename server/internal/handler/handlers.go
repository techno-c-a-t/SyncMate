package handler

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"syncmate/server/internal/auth"
	"syncmate/server/internal/domain"
	pb "syncmate/server/internal/proto/syncmate"
	"syncmate/server/internal/room"
	"syncmate/server/internal/weather"
)

type Handler struct {
	mu             sync.RWMutex
	users          map[string]*domain.User // email -> User
	userByID       map[int64]*domain.User  // id -> User
	userTokens     map[int64]string        // id -> current active token (for token revocation test)
	nextUserID     int64
	TokenManager   *auth.TokenManager
	RoomManager    *room.RoomManager
	WeatherService *weather.WeatherService
}

func NewHandler(tm *auth.TokenManager, rm *room.RoomManager, ws *weather.WeatherService) *Handler {
	return &Handler{
		users:          make(map[string]*domain.User),
		userByID:       make(map[int64]*domain.User),
		userTokens:     make(map[int64]string),
		nextUserID:     1,
		TokenManager:   tm,
		RoomManager:    rm,
		WeatherService: ws,
	}
}

// SanitizeInput escapes HTML/script strings to prevent XSS/injection (AGG-AUTH-2)
func SanitizeInput(s string) string {
	return strings.TrimSpace(html.EscapeString(s))
}

type RegisterRequest struct {
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	WeightKg    *float64 `json:"weight_kg"`
	HeightCm    *int32   `json:"height_cm"`
	FitnessCoef float64  `json:"fitness_coef"`
}

type AuthResponse struct {
	UserID       int64   `json:"user_id"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	User         *domain.User `json:"user"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Username = SanitizeInput(req.Username)
	req.Email = strings.ToLower(SanitizeInput(req.Email))

	if req.Email == "" || req.Password == "" || req.Username == "" {
		http.Error(w, "Email, password and username required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	if _, exists := h.users[req.Email]; exists {
		h.mu.Unlock()
		http.Error(w, "User with this email already exists", http.StatusConflict)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.mu.Unlock()
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	userID := h.nextUserID
	h.nextUserID++

	if req.FitnessCoef <= 0 {
		req.FitnessCoef = 1.0
	}

	u := &domain.User{
		ID:            userID,
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  hash,
		WeightKg:      req.WeightKg,
		HeightCm:      req.HeightCm,
		FitnessCoef:   req.FitnessCoef,
		RevokedTokens: make(map[string]time.Time),
		CreatedAt:     time.Now(),
	}

	h.users[req.Email] = u
	h.userByID[userID] = u
	h.mu.Unlock()

	accessToken, refreshToken, _, err := h.TokenManager.GenerateTokens(u.ID, u.Username)
	if err != nil {
		http.Error(w, "Error generating tokens", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		UserID:       u.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         u,
	})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(SanitizeInput(req.Email))

	h.mu.Lock()
	u, exists := h.users[req.Email]
	if !exists {
		h.mu.Unlock()
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Revoke previous active token on new login (REST-2)
	if oldToken, ok := h.userTokens[u.ID]; ok && oldToken != "" {
		h.TokenManager.RevokeToken(oldToken)
	}
	h.mu.Unlock()

	if !auth.CheckPasswordHash(req.Password, u.PasswordHash) {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	accessToken, refreshToken, _, err := h.TokenManager.GenerateTokens(u.ID, u.Username)
	if err != nil {
		http.Error(w, "Error generating tokens", http.StatusInternalServerError)
		return
	}

	h.mu.Lock()
	h.userTokens[u.ID] = accessToken
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AuthResponse{
		UserID:       u.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         u,
	})
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

type UpdateProfileRequest struct {
	WeightKg    *float64 `json:"weight_kg"`
	HeightCm    *int32   `json:"height_cm"`
	FitnessCoef *float64 `json:"fitness_coef"`
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	if req.WeightKg != nil {
		user.WeightKg = req.WeightKg
	}
	if req.HeightCm != nil {
		user.HeightCm = req.HeightCm
	}
	if req.FitnessCoef != nil && *req.FitnessCoef > 0 {
		user.FitnessCoef = *req.FitnessCoef
	}
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

type CreateRoomRequest struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Goal        *pb.WorkoutGoal `json:"goal"`
}

type CreateRoomResponse struct {
	RoomID     string `json:"room_id"`
	InviteCode string `json:"invite_code"`
	InviteURL  string `json:"invite_url"`
}

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Title = SanitizeInput(req.Title)
	req.Description = SanitizeInput(req.Description)

	roomObj, code, err := h.RoomManager.CreateRoom(user, req.Title, req.Description, req.Goal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CreateRoomResponse{
		RoomID:     roomObj.ID,
		InviteCode: code,
		InviteURL:  "https://syncmate.app/join/" + code,
	})
}

func (h *Handler) JoinRoomByCode(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	code := pathParts[len(pathParts)-1]

	roomObj, _, err := h.RoomManager.JoinRoomByCode(user, code)
	if err != nil {
		if err == room.ErrUserBanned {
			http.Error(w, "Forbidden: Banned from room", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(roomObj)
}

func (h *Handler) GetWeather(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lng, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		http.Error(w, "Invalid lat/lng parameters", http.StatusBadRequest)
		return
	}

	weatherInfo := h.WeatherService.GetWeather(lat, lng)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(weatherInfo)
}

func (h *Handler) authenticate(r *http.Request) (*domain.User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, auth.ErrInvalidToken
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.TokenManager.ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}

	h.mu.RLock()
	user, exists := h.userByID[claims.UserID]
	h.mu.RUnlock()

	if !exists {
		return nil, auth.ErrInvalidToken
	}

	return user, nil
}
