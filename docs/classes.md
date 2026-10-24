# Схема классов и структуры хранения данных SyncMate (Client & Server)

Документ описывает архитектуру классов, моделей данных, локальных хранилищ и **классов Сериализаторов/Десериализаторов Protobuf** для **Android-приложения (Kotlin + XML)** и **Go Бэкенда**.

---

## ❓ Что такое `SlotID` (Слот-Айди)?
> **`SlotID`** — это компактный 1-байтовый порядковый номер бегуна (1, 2, 3...), который сервер присваивает участнику в момент входа в конкретную комнату. 
> Вместо того чтобы каждую секунду передавать по радиоканалу 64-битные `user_id` или длинные UUID-строки, в бинарных Protobuf-сообщениях используется короткий 1-байтовый `SlotID`. Это экономит до 80% сокетного трафика в условиях слабого леса/EDGE.

---

## ⛰️ Что такое Высота (`Altitude`)?
> **Альтитуда (`altitude`)** — это абсолютная высота над уровнем моря (в метрах на клиенте или миллиметрах `altitude_mm` в Protobuf).
> Используется для вычисления уклонов дороги (`slopeDeg`), от которых напрямую зависит расчёт нагрузки и калорий движком `CalorieEngine`.

---

## 1. Схема классов Android-приложения (Client Side)

```mermaid
classDiagram
    class UserSession {
        +Long userId
        +String accessToken
        +String refreshToken
        +Boolean isLoggedIn()
    }

    class UserProfile {
        +Double weightKg
        +Int heightCm
        +Double fitnessCoef
        +Boolean isComplete()
    }

    class PathPoint {
        +Long timestampMs
        +Double posX
        +Double posY
        +Double altitudeM
        +Double speedMS
        +Double slopeDeg
    }

    class ParticipantState {
        +Int slotId
        +String username
        +ParticipantStateEnum state
        +Double posX
        +Double posY
        +Double speedMS
        +Int distanceM
        +Int calories
        +Boolean isAdmin
        +List~PathPoint~ recentPoints
    }

    class RoomState {
        +String roomId
        +String title
        +String description
        +String inviteCode
        +WorkoutGoal goal
        +Boolean workoutActive
        +Long workoutStartTimeMs
        +Int countdownSeconds
        +List~ParticipantState~ participants
    }

    class ProtobufSerializer {
        +serializeUserPointMessage(state, speedMS, distM, points) ByteArray
        +serializeAdminCommand(action, targetUserId, newGoal) ByteArray
        +deserializeRoomStateBroadcast(bytes) RoomState
        +deserializeCommandAck(bytes) CommandAck
    }

    class GpsTrackerService {
        -FusedLocationProviderClient locationClient
        -SensorManager sensorManager
        -List~PathPoint~ rawPointsBuffer
        -List~PathPoint~ historyPoints
        +startTracking()
        +stopTracking()
        +compressRawPointsWithRDP()
        +flushOfflineBatch()
    }

    class CalorieEngine {
        +calculateCalories(weightKg, speedMS, slopeDeg, weatherCoef) Int
    }

    class WebSocketService {
        -OkHttpClient webSocketClient
        -ProtobufSerializer serializer
        -String currentRoomId
        +connect(token, code)
        +sendTelemetry(userPointMessage)
        +sendAdminCommand(adminCommandMessage)
        +observeRoomState() Flow~RoomState~
    }

    class AuthRepository {
        +register(email, pass, name)
        +login(email, pass)
        +getProfile() UserProfile
        +updateProfile(weight, height)
    }

    class RoomRepository {
        +createRoom(title, goal)
        +joinRoomByCode(code) RoomState
    }

    UserProfile <-- CalorieEngine
    PathPoint <-- GpsTrackerService
    ParticipantState *-- PathPoint
    RoomState *-- ParticipantState
    WebSocketService --> ProtobufSerializer
    WebSocketService --> RoomState
    GpsTrackerService --> WebSocketService
```

### 📋 Подробная выжимка по классам Android-клиента:

- **`UserSession`**: Хранит активный сессионный контекст авторизации на смартфоне (токены `accessToken`, `refreshToken` и `userId`). Отвечает за проверку статуса входа в приложение.
- **`UserProfile`**: Модель личных физических параметров бегуна (`weightKg`, `heightCm`, `fitnessCoef`). Служит первоисточником данных для расчёта сжигаемых калорий.
- **`PathPoint`**: Структура отдельной точки гео-маршрута на клиенте (`timestampMs` — точная метка времени Unix, координаты `posX`/`posY`, `altitudeM`, скорость в м/с `speedMS` и уклон `slopeDeg`).
- **`ParticipantState`**: Модель текущего состояния напарника по пробежке на карте (`slotId`, имя, статус `state`, текущая позиция, скорость, набранные километры, калории и массив точек `recentPoints`).
- **`RoomState`**: Главный экранный снимок комнаты. Хранит инвайт-код `inviteCode`, цель `goal`, статус активности `workoutActive`, таймер обратного отсчёта и список всех участников `participants`.
- **`ProtobufSerializer` (Сериализатор/Десериализатор Android)**: Модуль преобразования данных между удобными Kotlin-объектами (`RoomState`, `PathPoint`, `speedMS: Double`) и компактными сетевыми байтами Protobuf (`UserPointMessage`, `RoomStateBroadcast`). Конвертирует м/с в миллиметры в секунду (`speed_mm_s`), абсолютные миллисекунды в `delta_time_ms` и назад.
- **`GpsTrackerService`**: Android `ForegroundService` непрерывного сбора географических координат от датчиков. Разделяет свежий поток в `rawPointsBuffer` и сжатый RDP-алгоритмом итоговый трек в `historyPoints`.
- **`CalorieEngine`**: Модуль автономного расчёта калорий. Вычисляет энергозатраты на основе массы тела, текущей скорости `speedMS`, угла наклона поверхности `slopeDeg` и погодного коэффициента.
- **`WebSocketService`**: Транспортный агент бинарной сокет-связи. Управляет авто-переподключениями, использует `ProtobufSerializer` для кодирования и декодирования Protobuf-сообщений (`UserPointMessage`, `AdminCommandMessage`, `RoomStateBroadcast`).
- **`AuthRepository`**: HTTP REST клиент для авторизации, регистрации и обновления профиля (`/auth/register`, `/auth/login`, `/profile`).
- **`RoomRepository`**: HTTP REST клиент для создания комнат и первичной загрузки тяжелых данных по инвайт-ссылке (`/rooms`, `/rooms/join/:code`).

---

## 2. Схема структур и классов Go Сервера (Server Side)

```mermaid
classDiagram
    class pb_PathPoint {
        +int64 DeltaTimeMs
        +float64 PosX
        +float64 PosY
        +int32 AltitudeMm
        +int32 SpeedMmS
        +int32 SlopeDeg
        +int32 DeltaSteps
    }

    class pb_WorkoutGoal {
        +GoalType GoalType
        +int32 TargetValue
    }

    class pb_UserPointMessage {
        +int32 UserId
        +int32 PacketId
        +ParticipantStateEnum State
        +int32 SpeedMmS
        +int32 KalCommon
        +int32 DistanceCommonM
        +[]*pb_PathPoint Points
    }

    class pb_AdminCommandMessage {
        +int32 AdminId
        +AdminActionCommand Action
        +int32 TargetUserId
        +pb_WorkoutGoal* NewGoal
        +int32 CountdownSeconds
    }

    class pb_RoomStateBroadcast {
        +string RoomId
        +string Title
        +int32 RoomTrainTimeSec
        +bool WorkoutActive
        +pb_WorkoutGoal* Goal
        +[]*ParticipantState Participants
    }

    class ProtoSerializer {
        +UnmarshalUserPointMessage(bytes) (*pb_UserPointMessage, error)
        +UnmarshalAdminCommand(bytes) (*pb_AdminCommandMessage, error)
        +MarshalRoomStateBroadcast(room *domain_Room) ([]byte, error)
        +MarshalCommandAck(action, status, msg) ([]byte, error)
    }

    class User {
        +int64 ID
        +string Username
        +string Email
        +string PasswordHash
        +*float64 WeightKg
        +*int32 HeightCm
        +float64 FitnessCoef
        +map~string,Time~ RevokedTokens
        +HasCompleteProfile() bool
    }

    class RoomMember {
        +int64 UserID
        +string Username
        +int32 SlotID
        +bool IsAdmin
        +ParticipantStateEnum State
        +float64 PosX
        +float64 PosY
        +int32 SpeedMmS
        +int32 DistanceM
        +int32 Kalories
        +Time* PausedAtTime
        +[]*pb_PathPoint RecentPoints
    }

    class Room {
        +RWMutex Mu
        +string ID
        +string Title
        +string InviteCode
        +pb_WorkoutGoal* Goal
        +bool WorkoutActive
        +Time WorkoutStartTime
        +map~int64,RoomMember*~ Members
        +map~int64,bool~ Blacklist
        +map~int64,bool~ Admins
    }

    class RoomManager {
        -map~string,Room*~ rooms
        -map~string,string~ codeToRoomID
        +CreateRoom(creator, title, goal)
        +JoinRoomByCode(user, code)
        +ProcessUserPointMessage(room, user, msg)
        +ProcessAdminCommand(room, adminId, cmd)
    }

    class Hub {
        -map~string,map~int64,Client**~ Clients
        -ProtoSerializer serializer
        +HandleWebSocket(w, r)
        +BroadcastRoomState(room)
    }

    pb_UserPointMessage *-- pb_PathPoint
    pb_AdminCommandMessage *-- pb_WorkoutGoal
    pb_RoomStateBroadcast *-- pb_WorkoutGoal
    ProtoSerializer --> pb_UserPointMessage
    ProtoSerializer --> pb_RoomStateBroadcast
    Room *-- RoomMember
    RoomMember *-- pb_PathPoint
    Room *-- pb_WorkoutGoal
    RoomManager *-- Room
    Hub --> ProtoSerializer
    Hub --> RoomManager
```

### 📋 Подробная выжимка по структурам Go-сервера:

- **`pb_PathPoint`**: Бинарная Protobuf-структура точки маршрута при передаче по сети (`DeltaTimeMs`, `PosX`, `PosY`, `AltitudeMm`, `SpeedMmS`, `SlopeDeg`, `DeltaSteps`).
- **`pb_WorkoutGoal`**: Бинарная Protobuf-структура цели тренировки (`GoalType` — дистанция/время/калории, `TargetValue` — целевое значение).
- **`pb_UserPointMessage`**: Пакет телеметрии от бегуна к серверу. Несет текущий `State`, набранные километры, калории и оффлайн-батч точек `Points`.
- **`pb_AdminCommandMessage`**: Пакет управляющей команды от администратора (`BEGIN`, `TERMINATE`, `BAN`, `UNBAN`, `SET_GOAL`, `SET_START_COUNTDOWN`).
- **`pb_RoomStateBroadcast`**: Скомпрессованный кадр состояния комнаты, который сервер рассылает всем подключенным сокетам.
- **`ProtoSerializer` (Сериализатор/Десериализатор Go)**: Серверный шлюз распаковки и упаковки бинарного Protobuf-трафика. Переводит входящие байты сокета `[]byte` в структуры `pb.UserPointMessage` / `pb.AdminCommandMessage`, а также собирает доменные структуры `domain.Room` и `domain.RoomMember` в финальный бинарный пакет `pb.RoomStateBroadcast` для рассылки всем клиентам.
- **`User`**: Запись учетной записи в памяти бэкенда (`ID`, `Email`, хэш пароля bcrypt, рост, вес, `RevokedTokens` для инвалидации сессий).
- **`RoomMember`**: Внутреннее состояние участника в оперативной памяти сервера (`UserID`, выданный `SlotID`, статус `State`, `PausedAtTime` для точной фиксации времени паузы и накопленные точки `RecentPoints`).
- **`Room`**: Модель тренировочной комнаты с потокобезопасным мьютексом `sync.RWMutex`, маппингом участников `Members`, изолированным Черным списком `Blacklist` и пулом админов `Admins`.
- **`RoomManager`**: Сердце бизнес-логики сервера. Создает комнаты, генерирует уникальные инвайт-коды `RUN-XXXXXX`, обрабатывает батчи и команды, следит за разбаном с 0 и удаляет неактивные комнаты раз в 15 минут.
- **`Hub`**: Высоконагруженный сокетный хаб. Управляет сокет-соединениями `Client`, использует `ProtoSerializer` для безопасного анмаршалинга, проверяет подлинность `SlotID` (защита от спуфинга), ограничивает размер кадра (защита от OOM до 1 МБ) и рассылает кадры комнат.
