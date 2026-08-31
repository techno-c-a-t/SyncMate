package room

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"syncmate/server/internal/domain"
	pb "syncmate/server/internal/proto/syncmate"
)

var (
	ErrRoomNotFound     = errors.New("room not found")
	ErrUserBanned       = errors.New("user is banned from this room")
	ErrAlreadyInRoom    = errors.New("user is already in room")
	ErrNotAdmin         = errors.New("action requires admin privileges")
	ErrIncompleteProfile = errors.New("user weight and height required to participate in active workout")
)

type RoomManager struct {
	mu           sync.RWMutex
	rooms        map[string]*domain.Room
	codeToRoomID map[string]string
}

func NewRoomManager() *RoomManager {
	rm := &RoomManager{
		rooms:        make(map[string]*domain.Room),
		codeToRoomID: make(map[string]string),
	}

	// Start background cleanup ticker for inactive rooms (15 mins rule)
	go rm.startCleanupLoop()
	return rm
}

// GenerateUniqueInviteCode generates a guaranteed unique invite code
func (rm *RoomManager) GenerateUniqueInviteCode() string {
	for {
		bytes := make([]byte, 3)
		_, _ = rand.Read(bytes)
		code := fmt.Sprintf("RUN-%s", hex.EncodeToString(bytes))
		if _, exists := rm.codeToRoomID[code]; !exists {
			return code
		}
	}
}

// CreateRoom creates a new workout room with creator as initial admin
func (rm *RoomManager) CreateRoom(creator *domain.User, title, description string, goal *pb.WorkoutGoal) (*domain.Room, string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	roomID := fmt.Sprintf("rm_%d_%d", creator.ID, time.Now().UnixNano())
	inviteCode := rm.GenerateUniqueInviteCode()

	if goal == nil {
		goal = &pb.WorkoutGoal{GoalType: pb.GoalType_GOAL_NONE, TargetValue: 0}
	}

	r := &domain.Room{
		ID:                 roomID,
		Title:              title,
		Description:        description,
		InviteCode:         inviteCode,
		Goal:               goal,
		WorkoutActive:      false,
		Members:            make(map[int64]*domain.RoomMember),
		SlotToUserID:       make(map[int32]int64),
		Blacklist:          make(map[int64]bool),
		Admins:             map[int64]bool{creator.ID: true},
		LastActiveTime:     time.Now(),
		NextSlotID:         1,
	}

	// Add creator as member
	member := &domain.RoomMember{
		UserID:        creator.ID,
		Username:      creator.Username,
		SlotID:        r.NextSlotID,
		IsAdmin:       true,
		State:         pb.ParticipantStateEnum_STATE_WAITING,
		TrainCfScaled: int32(creator.FitnessCoef * 1000),
		RecentPoints:  make([]*pb.PathPoint, 0),
	}
	r.Members[creator.ID] = member
	r.SlotToUserID[r.NextSlotID] = creator.ID
	r.NextSlotID++

	rm.rooms[roomID] = r
	rm.codeToRoomID[inviteCode] = roomID

	return r, inviteCode, nil
}

// JoinRoomByCode allows users (even with incomplete profiles) to join and preview room
func (rm *RoomManager) JoinRoomByCode(user *domain.User, code string) (*domain.Room, *domain.RoomMember, error) {
	rm.mu.Lock()
	roomID, exists := rm.codeToRoomID[code]
	if !exists {
		rm.mu.Unlock()
		return nil, nil, ErrRoomNotFound
	}
	r := rm.rooms[roomID]
	rm.mu.Unlock()

	r.Mu.Lock()
	defer r.Mu.Unlock()

	// Check Blacklist for THIS room
	if r.Blacklist[user.ID] {
		return nil, nil, ErrUserBanned
	}

	r.LastActiveTime = time.Now()

	// Return existing member if already present
	if member, ok := r.Members[user.ID]; ok {
		// If user was previously left, reset their slate to zero
		if member.State == pb.ParticipantStateEnum_STATE_LEFT {
			member.State = pb.ParticipantStateEnum_STATE_WAITING
			member.DistanceM = 0
			member.Kalories = 0
			member.SpeedMmS = 0
			member.RecentPoints = make([]*pb.PathPoint, 0)
		}
		return r, member, nil
	}

	// New member join
	slotID := r.NextSlotID
	r.NextSlotID++

	isAdmin := r.Admins[user.ID]
	member := &domain.RoomMember{
		UserID:        user.ID,
		Username:      user.Username,
		SlotID:        slotID,
		IsAdmin:       isAdmin,
		State:         pb.ParticipantStateEnum_STATE_WAITING,
		TrainCfScaled: int32(user.FitnessCoef * 1000),
		RecentPoints:  make([]*pb.PathPoint, 0),
	}

	r.Members[user.ID] = member
	r.SlotToUserID[slotID] = user.ID

	return r, member, nil
}

// GetRoomByID retrieves a room by ID
func (rm *RoomManager) GetRoomByID(roomID string) (*domain.Room, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	r, exists := rm.rooms[roomID]
	return r, exists
}

// ProcessUserPointMessage updates user telemetry & state
func (rm *RoomManager) ProcessUserPointMessage(r *domain.Room, user *domain.User, msg *pb.UserPointMessage) error {
	r.Mu.Lock()
	defer r.Mu.Unlock()

	member, ok := r.Members[user.ID]
	if !ok {
		return errors.New("user not in room")
	}

	// Incomplete profile check: Block transitioning to READY or ACTIVE without weight/height
	targetState := msg.GetState()
	if (targetState == pb.ParticipantStateEnum_STATE_READY || targetState == pb.ParticipantStateEnum_STATE_ACTIVE) && !user.HasCompleteProfile() {
		return ErrIncompleteProfile
	}

	// Handle STATE_LEFT
	if targetState == pb.ParticipantStateEnum_STATE_LEFT {
		member.State = pb.ParticipantStateEnum_STATE_LEFT
		delete(r.Members, user.ID)
		r.LastActiveTime = time.Now()
		return nil
	}

	// Track pause time for accurate final duration on termination
	if targetState == pb.ParticipantStateEnum_STATE_PAUSED && member.State != pb.ParticipantStateEnum_STATE_PAUSED {
		now := time.Now()
		member.PausedAtTime = &now
	} else if targetState == pb.ParticipantStateEnum_STATE_ACTIVE {
		member.PausedAtTime = nil
	}

	member.State = targetState

	// Process out-of-bounds sanitized telemetry
	if msg.GetSpeedMmS() >= 0 && msg.GetSpeedMmS() < 50000 { // Max 50 m/s (~180 km/h)
		member.SpeedMmS = msg.GetSpeedMmS()
	}
	if msg.GetDistanceCommonM() >= 0 {
		member.DistanceM = msg.GetDistanceCommonM()
	}
	member.Kalories = msg.GetKalCommon()
	member.AltitudeMm = msg.GetAltitudeMm()

	// Append batch GPS points (with limit to prevent OOM)
	if len(msg.GetPoints()) > 0 {
		pointsToAdd := msg.GetPoints()
		if len(pointsToAdd) > 500 {
			pointsToAdd = pointsToAdd[:500]
		}
		for _, pt := range pointsToAdd {
			// Bounds check on coordinates
			if pt.PosX >= -180 && pt.PosX <= 180 && pt.PosY >= -90 && pt.PosY <= 90 {
				member.RecentPoints = append(member.RecentPoints, pt)
				member.PosX = pt.PosX
				member.PosY = pt.PosY
			}
		}
	}

	r.LastActiveTime = time.Now()
	return nil
}

// ProcessAdminCommand handles admin control messages
func (rm *RoomManager) ProcessAdminCommand(r *domain.Room, adminID int64, cmd *pb.AdminCommandMessage) (int32, string, error) {
	r.Mu.Lock()
	defer r.Mu.Unlock()

	// Verify Admin privileges
	if !r.Admins[adminID] {
		return 403, "Forbidden: Admin privileges required", ErrNotAdmin
	}

	switch cmd.GetAction() {
	case pb.AdminActionCommand_ACTION_BEGIN:
		r.WorkoutActive = true
		r.WorkoutStartTime = time.Now()
		// Automatically start timer for any user standing in STATE_READY
		for _, m := range r.Members {
			if m.State == pb.ParticipantStateEnum_STATE_READY {
				m.State = pb.ParticipantStateEnum_STATE_ACTIVE
			}
		}
		return 200, "Workout started successfully", nil

	case pb.AdminActionCommand_ACTION_SET_START_COUNTDOWN:
		r.CountdownSeconds = cmd.GetCountdownSeconds()
		r.CountdownStartTime = time.Now()
		return 200, "Countdown initiated", nil

	case pb.AdminActionCommand_ACTION_SET_GOAL:
		if cmd.GetNewGoal() != nil {
			r.Goal = cmd.GetNewGoal()
		}
		return 200, "Workout goal updated", nil

	case pb.AdminActionCommand_ACTION_BAN:
		targetID := int64(cmd.GetTargetUserId())
		r.Blacklist[targetID] = true
		if m, ok := r.Members[targetID]; ok {
			m.State = pb.ParticipantStateEnum_STATE_BANNED
			delete(r.Members, targetID)
		}
		return 200, "User banned from room", nil

	case pb.AdminActionCommand_ACTION_UNBAN:
		targetID := int64(cmd.GetTargetUserId())
		delete(r.Blacklist, targetID)
		return 200, "User unbanned from room", nil

	case pb.AdminActionCommand_ACTION_GIVE_ADMIN:
		targetID := int64(cmd.GetTargetUserId())
		r.Admins[targetID] = true
		if m, ok := r.Members[targetID]; ok {
			m.IsAdmin = true
		}
		return 200, "User granted admin privileges", nil

	case pb.AdminActionCommand_ACTION_TAKE_ADMIN:
		targetID := int64(cmd.GetTargetUserId())
		delete(r.Admins, targetID)
		if m, ok := r.Members[targetID]; ok {
			m.IsAdmin = false
		}
		return 200, "User admin privileges revoked", nil

	case pb.AdminActionCommand_ACTION_TERMINATE:
		r.WorkoutActive = false
		return 200, "Workout terminated", nil

	default:
		return 400, "Unknown admin action", errors.New("unknown action")
	}
}

// startCleanupLoop automatically deletes rooms after 15 mins of inactivity
func (rm *RoomManager) startCleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		rm.mu.Lock()
		now := time.Now()
		for id, r := range rm.rooms {
			r.Mu.RLock()
			inactiveDuration := now.Sub(r.LastActiveTime)
			activeMembersCount := 0
			for _, m := range r.Members {
				if m.State == pb.ParticipantStateEnum_STATE_ACTIVE || m.State == pb.ParticipantStateEnum_STATE_READY || m.State == pb.ParticipantStateEnum_STATE_WAITING {
					activeMembersCount++
				}
			}
			r.Mu.RUnlock()

			// Room auto-deletes if inactive for >15 minutes AND no active admins/members connected
			if inactiveDuration > 15*time.Minute && activeMembersCount == 0 {
				delete(rm.codeToRoomID, r.InviteCode)
				delete(rm.rooms, id)
			}
		}
		rm.mu.Unlock()
	}
}
