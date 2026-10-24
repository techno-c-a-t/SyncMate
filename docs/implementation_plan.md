# Схема классов и структуры хранения данных SyncMate (Client & Server)

Документ содержит интерактивную спецификацию классов и разделения хранимых данных между мобильным клиентом Android и бэкендом на Go.

Полная версия сохранена в файле: **[docs/class_diagrams.md](file:///home/technocat/Документы/Projects/SyncMate/docs/class_diagrams.md)**.

---

## 1. Архитектура классов и данных Android-клиента

### Классы и сущности (Kotlin):
- **`UserSession`**: Токены `accessToken`, `refreshToken`, идентификаторы текущего пользователя.
- **`UserProfile`**: Данные метрик (`weightKg`, `heightCm`, `fitnessCoef`) для локального отсчета калорий.
- **`PathPoint`**: Структура GPS-точки (`deltaTimeMs`, `posX`, `posY`, `speedMmS`, `altitudeMm`, `slopeDeg`).
- **`ParticipantState`**: Позиция, дистанция, калории и набранные точки `recentPoints` конкретного напарника.
- **`RoomState`**: Активное состояние всей комнаты (название, цель `WorkoutGoal`, список `participants`, таймеры).
- **`GpsTrackerService`**: Android `ForegroundService`, считывающий GPS от `FusedLocationProviderClient` и шаг за шагом сжимающий их RDP-алгоритмом в `offlineBuffer`.
- **`WebSocketService`**: Управление бинарной сокет-сессией `wss://...`, декодирование Protobuf-сообщений, логика переподключений.
- **`CalorieEngine`**: Локальный расчёт калорий с учётом уклонов, скорости, массы тела и коэффициента погоды.

### Хранение данных на Android-устройстве:
1. **EncryptedSharedPreferences**: Ключи авторизации JWT, логин, пароль, рост и вес.
2. **In-Memory (LiveData / StateFlow)**: Текущие координаты группы, статус соединения, накопившийся оффлайн-батч для леса.
3. **Room Database (SQLite)**: Кэш прошлых пробежек для локального экрана *"История тренировок"*.

---

## 2. Архитектура классов и данных Go Бэкенда

### Классы и структуры (Go):
- **`User`** (`domain/models.go`): Учетная запись (email, bcrypt хэш, вес/рост, `RevokedTokens`).
- **`RoomMember`** (`domain/models.go`): Запись участника внутри комнаты (`SlotID`, `State`, координаты, `RecentPoints`, время паузы).
- **`Room`** (`domain/models.go`): Модель комнаты с `sync.RWMutex`, списком членов `Members`, изолированным Черным списком `Blacklist` и админами `Admins`.
- **`RoomManager`** (`room/room_manager.go`): Бизнес-логика создания, генерации уникальных кодов `RUN-XXXXXX`, обработки батчей, разбанов с 0 и 15-минутной авто-очистки.
- **`TokenManager`** (`auth/auth.go`): JWT HMAC проверка, ревокация токенов, bcrypt хэширование.
- **`Hub` & `Client`** (`realtime/hub.go`): Сокетный Хаб, OOM-защита 1МБ, привязка `slot_id`, трансляция Protobuf.
- **`WeatherService`** (`weather/weather_service.go`): Погодный кэш на 20 минут.

### Хранение данных на Сервере:
1. **PostgreSQL + PostGIS**: База данных учетных записей, завершенных тренировок и PostGIS-геометрия треков (`ST_LineString`).
2. **Оперативная память (RAM)**: Активные комнаты `RoomManager.rooms`, сокет-слоты `SlotID`, ревоцированные JWT токены.
3. **Redis**: Кэш инвайт-кодов и погодная сетка.
