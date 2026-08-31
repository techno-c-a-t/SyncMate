package room

import (
	"testing"
	"time"

	"syncmate/server/internal/domain"
)

func TestRoomLifecycleAndAdmins(t *testing.T) {
	rm := NewRoomManager()

	creator := &domain.User{
		ID:          "usr_creator",
		Username:    "CreatorAlex",
		FitnessCoef: 1.0,
	}

	// 1. Создание комнаты
	roomState := rm.CreateRoom(creator, "Утренний Забег")
	if roomState == nil {
		t.Fatalf("Expected room to be created")
	}

	if !roomState.AdminIDs[creator.ID] {
		t.Errorf("Expected creator to be admin")
	}

	// 2. Получение по коду инвайта
	fetched, err := rm.GetRoomByCode(roomState.InviteCode)
	if err != nil || fetched.ID != roomState.ID {
		t.Fatalf("Failed to fetch room by invite code")
	}

	// 3. Подключение второго участника
	user2 := &domain.User{
		ID:          "usr_runner2",
		Username:    "BobRunner",
		FitnessCoef: 0.9,
	}

	slotID2, isAdmin2 := roomState.JoinUser(user2)
	if slotID2 == 0 {
		t.Errorf("Expected non-zero slot ID for user2")
	}
	if isAdmin2 {
		t.Errorf("Expected user2 to not be admin initially")
	}

	// 4. Назначение второго участника админом
	err = roomState.GrantAdmin(creator.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to grant admin rights: %v", err)
	}

	if !roomState.AdminIDs[user2.ID] {
		t.Errorf("Expected user2 to now be admin")
	}

	// 5. Вторым админом запуск тренировки
	err = roomState.StartWorkout(user2.ID)
	if err != nil {
		t.Fatalf("Failed to start workout by second admin: %v", err)
	}

	if !roomState.WorkoutStarted {
		t.Errorf("Expected workout to be started")
	}

	// 6. Проверка сборки DTO информации для предпросмотра
	info := roomState.BuildInfoResponse()
	if len(info.Admins) != 2 {
		t.Errorf("Expected 2 admins in room info DTO, got %d", len(info.Admins))
	}
}

func TestRoomAutoExpirationRule(t *testing.T) {
	rm := NewRoomManager()

	creator := &domain.User{
		ID:       "usr_admin",
		Username: "AdminUser",
	}

	r := rm.CreateRoom(creator, "Тест автоудаления")
	code := r.InviteCode

	// Эмулируем неактивность админа > 15 минут
	r.Lock()
	r.LastAdminActiveAt = time.Now().Add(-16 * time.Minute)
	r.WorkoutStarted = false
	r.Unlock()

	// Ручной запуск логики очистки
	rm.cleanupRoutineOnce()

	_, err := rm.GetRoomByCode(code)
	if err != ErrRoomNotFound {
		t.Errorf("Expected room to be auto-deleted after 15m of admin inactivity, got err: %v", err)
	}
}

// Вспомогательный метод для тестового прогона очистки
func (rm *RoomManager) cleanupRoutineOnce() {
	rm.Lock()
	defer rm.Unlock()
	now := time.Now()
	for id, r := range rm.rooms {
		r.RLock()
		started := r.WorkoutStarted
		lastAdminActive := r.LastAdminActiveAt
		code := r.InviteCode
		r.RUnlock()

		if !started && now.Sub(lastAdminActive) > 15*time.Minute {
			delete(rm.rooms, id)
			delete(rm.codeToRoom, code)
		}
	}
}
