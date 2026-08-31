# Спецификация API SyncMate

Документ описывает HTTP REST API и WebSocket / Protobuf интерфейсы для взаимодействия мобильного приложения **SyncMate** (Kotlin) и бэкенд-сервера (Go).

---

## 1. HTTP REST API

Базовый URL: `https://api.syncmate.app/api/v1` (или IP вашего сервера).
Все защищенные эндпоинты требуют заголовок: `Authorization: Bearer <JWT_TOKEN>`.

### Auth & Profile

#### `POST /auth/register`
Регистрация нового пользователя.
- **Request Body**:
  ```json
  {
    "username": "runner_alex",
    "email": "alex@example.com",
    "password": "strongpassword123",
    "weight_kg": 75.5,
    "height_cm": 180,
    "fitness_coef": 1.0
  }
  ```
- **Response (201 Created)**:
  ```json
  {
    "user_id": "usr_987654321",
    "access_token": "eyJhbGciOi...",
    "refresh_token": "def456..."
  }
  ```

#### `POST /auth/login`
Авторизация по логину/email.
- **Request Body**:
  ```json
  {
    "email": "alex@example.com",
    "password": "strongpassword123"
  }
  ```
- **Response (200 OK)**:
  ```json
  {
    "user_id": "usr_987654321",
    "access_token": "eyJhbGciOi...",
    "refresh_token": "def456..."
  }
  ```

#### `GET /profile`
Получение профиля текущего пользователя (физические метрики для калорий).
- **Response (200 OK)**:
  ```json
  {
    "user_id": "usr_987654321",
    "username": "runner_alex",
    "weight_kg": 75.5,
    "height_cm": 180,
    "fitness_coef": 1.0
  }
  ```

---

### Rooms & Sessions

#### `POST /rooms`
Создание новой тренировочной комнаты. Создатель становится админом.
- **Request Body**:
  ```json
  {
    "title": "Утренняя пробежка",
    "activity_type": "RUNNING"
  }
  ```
- **Response (201 Created)**:
  ```json
  {
    "room_id": "rm_12345",
    "invite_code": "RUN-9821",
    "invite_url": "https://syncmate.app/join/RUN-9821",
    "admin_id": "usr_987654321"
  }
  ```

#### `GET /rooms/join/:code`
Получение информации о комнате по коду инвайта.
- **Response (200 OK)**:
  ```json
  {
    "room_id": "rm_12345",
    "invite_code": "RUN-9821",
    "title": "Утренняя пробежка",
    "activity_type": "RUNNING",
    "status": "WAITING",
    "admin_id": "usr_987654321",
    "participants_count": 2
  }
  ```

---

### Weather API

#### `GET /weather`
Получение погодных условий для точки старта (для вычисления сопротивления воздуха).
- **Query Params**: `lat=55.7558&lng=37.6173`
- **Response (200 OK)**:
  ```json
  {
    "temperature_celsius": 18.5,
    "wind_speed_m_s": 4.2,
    "wind_direction_deg": 180.0,
    "weather_condition": "CLEAR"
  }
  ```

---

### Workout History

#### `GET /workouts`
Получение списка завершенных тренировок пользователя.

#### `GET /workouts/:id`
Получение полной геометрии (GPX/JSON) и итоговой статистики конкретной тренировки.

---

## 2. Real-Time WebSocket API & Protobuf Schema

Эндпоинт подключения WebSocket:
`wss://api.syncmate.app/ws?room_id=rm_12345&token=<JWT_TOKEN>`

Сообщения бинарные (Google Protocol Buffers).

### Схема сообщений Protobuf (`syncmate.proto`)

```protobuf
syntax = "proto3";

package syncmate;

option go_package = "server/internal/proto/syncmate";

enum ActivityStatus {
  STATUS_UNKNOWN = 0;
  STATUS_PAUSED = 1;
  STATUS_ACTIVE = 2;
  STATUS_RECONNECTING = 3;
}

enum AdminAction {
  ACTION_UNKNOWN = 0;
  ACTION_START_WORKOUT = 1;
  ACTION_FINISH_WORKOUT = 2;
  ACTION_KICK_PARTICIPANT = 3;
}

// Отдельная сжатая точка GPS-трека (дельты относительно предыдущей точки)
message TelemetryPoint {
  int64 delta_time_ms = 1;     // Время с прошлой точки (мс)
  double lat = 2;              // Широта
  double lng = 3;              // Долгота
  float altitude = 4;          // Высота (метры)
  float speed_m_s = 5;         // Скорость (м/с)
  float slope_percentage = 6;  // Уклон дороги (%)
  int32 delta_steps = 7;       // Кол-во шагов за этот отрезок
}

// Пакет телеметрии от клиента к серверу (может содержать RDP-сжатый батч точек)
message ClientTelemetryBatch {
  string user_id = 1;
  uint64 sequence_number = 2;  // Инкрементный номер пакета
  int64 timestamp_ms = 3;
  ActivityStatus status = 4;   // Статус участника (ACTIVE / PAUSED)
  
  float current_calories = 5;  // Текущие калории
  int32 total_steps = 6;       // Всего шагов
  float total_distance_m = 7;  // Всего дистанция (метры)
  
  repeated TelemetryPoint points = 8; // Сжатые ключевые точки траектории
}

// Состояние одного участника внутри рассылки комнаты
message ParticipantState {
  string user_id = 1;
  string username = 2;
  ActivityStatus status = 3;
  
  double last_lat = 4;
  double last_lng = 5;
  float speed_m_s = 6;
  float slope_percentage = 7;
  
  float current_calories = 8;
  int32 total_steps = 9;
  float total_distance_m = 10;
  
  int64 last_seen_ms = 11;
  repeated TelemetryPoint recent_points = 12; // Недавние точки для отрисовки трека
}

// Сообщение рассылки от сервера ко всем участникам комнаты (1 раз в 1-2 сек или по батчу)
message ServerRoomBroadcast {
  string room_id = 1;
  int64 server_time_ms = 2;
  bool workout_started = 3;
  
  repeated ParticipantState participants = 4;
}

// Команда админа комнаты
message AdminRoomControlCommand {
  string admin_id = 1;
  AdminAction action = 2;
  string target_user_id = 3; // Используется при ACTION_KICK_PARTICIPANT
}
```
