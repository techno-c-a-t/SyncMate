package realtime

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"syncmate/server/internal/auth"
	"syncmate/server/internal/domain"
	pb "syncmate/server/internal/proto/syncmate"
	"syncmate/server/internal/room"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,  // 64 KB
	WriteBufferSize: 64 * 1024,  // 64 KB
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	User     *domain.User
	Room     *domain.Room
	Send     chan []byte
	SlotID   int32
}

type Hub struct {
	mu           sync.RWMutex
	RoomManager  *room.RoomManager
	TokenManager *auth.TokenManager
	Clients      map[string]map[int64]*Client // map[roomID]map[userID]*Client
}

func NewHub(rm *room.RoomManager, tm *auth.TokenManager) *Hub {
	return &Hub{
		RoomManager:  rm,
		TokenManager: tm,
		Clients:      make(map[string]map[int64]*Client),
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	roomCode := r.URL.Query().Get("code")
	roomID := r.URL.Query().Get("room_id")

	// Validate JWT Token (AGG-AUTH-1)
	claims, err := h.TokenManager.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	user := &domain.User{
		ID:       claims.UserID,
		Username: claims.Username,
	}

	// Retrieve Room
	var targetRoom *domain.Room
	if roomID != "" {
		rObj, ok := h.RoomManager.GetRoomByID(roomID)
		if !ok {
			http.Error(w, "Room not found", http.StatusNotFound)
			return
		}
		targetRoom = rObj
	} else if roomCode != "" {
		rObj, _, err := h.RoomManager.JoinRoomByCode(user, roomCode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		targetRoom = rObj
	} else {
		http.Error(w, "room_id or code required", http.StatusBadRequest)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket Upgrade Error: %v", err)
		return
	}

	// Security: Set Read Limit to 1 MB (AGG-WS-3 OOM protection)
	conn.SetReadLimit(1024 * 1024)

	// Ensure member exists in room
	_, member, err := h.RoomManager.JoinRoomByCode(user, targetRoom.InviteCode)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		conn.Close()
		return
	}

	client := &Client{
		Hub:    h,
		Conn:   conn,
		User:   user,
		Room:   targetRoom,
		Send:   make(chan []byte, 256),
		SlotID: member.SlotID,
	}

	h.registerClient(client)

	go client.writePump()
	go client.readPump()
}

func (h *Hub) registerClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Clients[c.Room.ID]; !ok {
		h.Clients[c.Room.ID] = make(map[int64]*Client)
	}
	h.Clients[c.Room.ID][c.User.ID] = c

	h.BroadcastRoomState(c.Room)
}

func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if roomClients, ok := h.Clients[c.Room.ID]; ok {
		if _, exists := roomClients[c.User.ID]; exists {
			delete(roomClients, c.User.ID)
			close(c.Send)
		}
		if len(roomClients) == 0 {
			delete(h.Clients, c.Room.ID)
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregisterClient(c)
		c.Conn.Close()
	}()

	for {
		messageType, payload, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		// Try Unmarshaling as UserPointMessage
		var pointMsg pb.UserPointMessage
		if err := proto.Unmarshal(payload, &pointMsg); err == nil && pointMsg.GetState() != pb.ParticipantStateEnum_STATE_UNKNOWN {
			// Security: Override client slot_id with actual authenticated slot_id (AGG-SEC-2)
			pointMsg.UserId = c.SlotID

			err := c.Hub.RoomManager.ProcessUserPointMessage(c.Room, c.User, &pointMsg)
			if err != nil {
				// E.g. incomplete profile error
				log.Printf("ProcessUserPointMessage Error: %v", err)
			}
			c.Hub.BroadcastRoomState(c.Room)
			continue
		}

		// Try Unmarshaling as AdminCommandMessage
		var adminCmd pb.AdminCommandMessage
		if err := proto.Unmarshal(payload, &adminCmd); err == nil && adminCmd.GetAction() != pb.AdminActionCommand_ACTION_UNKNOWN {
			statusCode, msgText, err := c.Hub.RoomManager.ProcessAdminCommand(c.Room, c.User.ID, &adminCmd)
			
			ack := &pb.CommandAckMessage{
				Action:     adminCmd.GetAction(),
				StatusCode: statusCode,
				Message:    msgText,
			}
			ackBytes, _ := proto.Marshal(ack)
			c.Send <- ackBytes

			if err == nil {
				c.Hub.BroadcastRoomState(c.Room)
			}
			continue
		}

		// AGG-WS-1: Fuzzing / Garbage packet ignored without crash
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.Conn.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) BroadcastRoomState(r *domain.Room) {
	r.Mu.RLock()
	
	var elapsedSec int32 = 0
	if r.WorkoutActive && !r.WorkoutStartTime.IsZero() {
		elapsedSec = int32(time.Since(r.WorkoutStartTime).Seconds())
	}

	participants := make([]*pb.ParticipantState, 0, len(r.Members))
	for _, m := range r.Members {
		// Calculate member running duration
		var mDistance int32 = m.DistanceM
		
		ps := &pb.ParticipantState{
			Username:      m.Username,
			UserId:        m.SlotID,
			State:         m.State,
			PosX:          m.PosX,
			PosY:          m.PosY,
			TrainCfScaled: m.TrainCfScaled,
			AltitudeMm:    m.AltitudeMm,
			SpeedMmS:      m.SpeedMmS,
			DistanceM:     mDistance,
			Kalories:      m.Kalories,
			SlopeDeg:      m.SlopeDeg,
			IsAdmin:       m.IsAdmin,
			RecentPoints:  m.RecentPoints,
		}
		participants = append(participants, ps)
	}

	var startTimeMs int64 = 0
	if !r.WorkoutStartTime.IsZero() {
		startTimeMs = r.WorkoutStartTime.UnixNano() / 1e6
	}

	var countdownStartMs int64 = 0
	if !r.CountdownStartTime.IsZero() {
		countdownStartMs = r.CountdownStartTime.UnixNano() / 1e6
	}

	broadcast := &pb.RoomStateBroadcast{
		RoomId:               r.ID,
		Title:                r.Title,
		Description:          r.Description,
		RoomTrainTimeSec:     elapsedSec,
		WorkoutStartTimeMs:   startTimeMs,
		CountdownStartTimeMs: countdownStartMs,
		CountdownSeconds:     r.CountdownSeconds,
		WorkoutActive:        r.WorkoutActive,
		Goal:                 r.Goal,
		Participants:         participants,
	}
	r.Mu.RUnlock()

	bBytes, err := proto.Marshal(broadcast)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if roomClients, ok := h.Clients[r.ID]; ok {
		for _, client := range roomClients {
			// WS-10: Do NOT send frames to banned or left clients
			select {
			case client.Send <- bBytes:
			default:
			}
		}
	}
}
