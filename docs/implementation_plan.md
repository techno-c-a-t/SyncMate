# Итоговая Спецификация Тестирования SyncMate

Вся документация по тестированию бэкенда SyncMate оформлена в виде 3 структурированных модулей в директории `/server/tests/`:

---

## Картотека тестовых модулей

### 1. [server/tests/nominal_small_tests.md](file:///home/technocat/Документы/Projects/SyncMate/server/tests/nominal_small_tests.md)
**Модуль 1: Маленькие штатные тесты (Nominal Small Tests)**
- **HTTP REST API**:
  - `REST-1`: Регистрация и многопоточность (`POST /auth/register`)
  - `REST-2`: Авторизация и инвалидация токенов (`POST /auth/login`)
  - `REST-3`: Чтение полного и неполного профиля (`GET /profile`)
  - `REST-4`: Циклические апдейты профиля (`PUT /profile`)
  - `REST-5`: Создание комнат и проверка уникальности кодов за 5-15 сек (`POST /rooms`)
  - `REST-6`: Тяжелая инициализация по коду с оффлайн-профилями (`GET /rooms/join/:code`)
  - `REST-7`: Опрос погоды и кэширование (`GET /weather`)
- **WebSocket & Protobuf**:
  - `WS-1`: Сокет Handshake и выдача `slot_id`
  - `WS-2`: Одиночная точка телеметрии (`UserPointMessage`)
  - `WS-3`: Градация батчей (2, 20, 200 точек) и параллельные батчи
  - `WS-4`: Смена состояний `STATE_READY` / `STATE_PAUSED`
  - `WS-5`: Выход из комнаты (`STATE_LEFT`)
  - `WS-6`: Таймер отсчета 3..2..1 (`ACTION_SET_START_COUNTDOWN`)
  - `WS-7`: Глобальный старт и блокировка неполных профилей (`ACTION_BEGIN`)
  - `WS-8`: Изменение цели на ходу (`ACTION_SET_GOAL`)
  - `WS-9A`: Бан активных, паузированных и ливнувших (`ACTION_BAN`)
  - `WS-9B`: Разбан и старт с 0 (`ACTION_UNBAN`)
  - `WS-10`: Финиш и полная тишина для выбывших (`ACTION_TERMINATE`)

---

### 2. [server/tests/nominal_large_tests.md](file:///home/technocat/Документы/Projects/SyncMate/server/tests/nominal_large_tests.md)
**Модуль 1: Большие сценарные штатные тесты (Nominal Large Tests)**
- `Сценарий 1`: Опоздавший бегун и смена цели на ходу
- `Сценарий 2`: Лесной оффлайн, параллельный батчинг, Бан и Разбан с чистого листа
- `Сценарий 3`: Передача прав админа, выход первого владельца и правило 15 минут
- `Сценарий 4`: Полная Изоляция Черного списка (Blacklist) между новыми комнатами

---

### 3. [server/tests/aggressive_tests.md](file:///home/technocat/Документы/Projects/SyncMate/server/tests/aggressive_tests.md)
**Модуль 3: Агрессивные тесты (Aggressive / Security / Fuzzing Tests)**
- `AGG-AUTH-1`: Подделка подписи, просроченные и украденные JWT токены
- `AGG-AUTH-2`: SQL/XSS инъекции в полях имени и профиля
- `AGG-WS-1`: Шквал бинарного мусора в сокет (Garbage Fuzzing)
- `AGG-WS-2`: Выход параметров за математические границы (Out-of-Bounds)
- `AGG-WS-3`: Переполнение памяти гигантским батчем на 50 000 точек (OOM)
- `AGG-SEC-1`: Выполнение админских команд обычным участником (`403 Forbidden`)
- `AGG-SEC-2`: Подмена `slot_id` (Спуфинг чужой позиции)
- `AGG-RACE-1`: Одновременный двухсторонний старт и терминирование в одну миллисекунду
- `AGG-RACE-2`: Сокетный DoS-флуд подключениями
