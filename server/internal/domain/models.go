package domain

import (
	"sync"
	"time"

	pb "syncmate/server/internal/proto/syncmate"
)

// User represents a user account
type User struct {
	ID            int64      `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	WeightKg      *float64   `json:"weight_kg"` // Nullable for incomplete profiles
	HeightCm      *int32     `json:"height_cm"` // Nullable for incomplete profiles
	FitnessCoef   float64    `json:"fitness_coef"`
	RevokedTokens map[string]time.Time `json:"-"` // Tracks revoked JWT IDs/tokens for logout/re-login invalidation
	CreatedAt     time.Time  `json:"created_at"`
}

// HasCompleteProfile checks if weight and height are filled
func (u *User) HasCompleteProfile() bool {
	return u.WeightKg != nil && *u.WeightKg > 0 && u.HeightCm != nil && *u.HeightCm > 0
}

// RoomMember represents a member in a room
type RoomMember struct {
	UserID        int64                   `json:"user_id"`
	Username      string                  `json:"username"`
	SlotID        int32                   `json:"slot_id"`
	IsAdmin       bool                    `json:"is_admin"`
	State         pb.ParticipantStateEnum `json:"state"`
	PosX          float64                 `json:"pos_x"`
	PosY          float64                 `json:"pos_y"`
	TrainCfScaled int32                   `json:"train_cf_scaled"`
	AltitudeMm    int32                   `json:"altitude_mm"`
	SpeedMmS      int32                   `json:"speed_mm_s"`
	DistanceM     int32                   `json:"distance_m"`
	Kalories      int32                   `json:"kalories"`
	SlopeDeg      int32                   `json:"slope_deg"`
	PausedAtTime  *time.Time              `json:"-"` // Tracks exact time when user paused
	RecentPoints  []*pb.PathPoint         `json:"recent_points"`
}

// Room represents a workout room
type Room struct {
	Mu                   sync.RWMutex                  `json:"-"`
	ID                   string                        `json:"id"`
	Title                string                        `json:"title"`
	Description          string                        `json:"description"`
	InviteCode           string                        `json:"invite_code"`
	Goal                 *pb.WorkoutGoal               `json:"goal"`
	WorkoutActive        bool                          `json:"workout_active"`
	WorkoutStartTime     time.Time                     `json:"workout_start_time"`
	CountdownStartTime   time.Time                     `json:"countdown_start_time"`
	CountdownSeconds     int32                         `json:"countdown_seconds"`
	Members              map[int64]*RoomMember         `json:"members"`
	SlotToUserID         map[int32]int64               `json:"slot_to_user_id"`
	Blacklist            map[int64]bool                `json:"blacklist"` // Banned user IDs isolated to THIS room
	Admins               map[int64]bool                `json:"admins"`
	LastActiveTime       time.Time                     `json:"last_active_time"`
	NextSlotID           int32                         `json:"-"`
}

// WeatherInfo represents cached weather data
type WeatherInfo struct {
	TempCelsiusX10 int32     `json:"temp_celsius_x10"`
	WindSpeedMmS   int32     `json:"wind_speed_mm_s"`
	WindDirDeg     int32     `json:"wind_dir_deg"`
	FetchedAt      time.Time `json:"fetched_at"`
}
