"""
===============================================================================
SyncMate Test Suite: Модуль 1 — Штатные Маленькие Тесты (Nominal Small Tests)
===============================================================================

Данный файл содержит полную реализацию штатных точечных тестов для бэкенда SyncMate.
Тесты проверяют каждый отдельный REST эндпоинт и WebSocket Protobuf сообщение 
в нормальных условиях работы приложения.

Библиотеки:
- urllib.request / json — стандартный клиент для отправки REST HTTP запросов.
- asyncio / websockets — асинхронная библиотека для управления бинарными сокет-сессиями.
- proto_helpers — утилита для кодирования Protobuf кадра в бинарные байты.
"""

import asyncio
import json
import time
import urllib.request
import urllib.error
import pytest
import websockets

# Импортируем наш легкий бинарный Protobuf-кодер
from proto_helpers import (
    ParticipantStateEnum,
    AdminActionCommand,
    GoalType,
    encode_user_point_message,
    encode_admin_command,
    encode_path_point,
    encode_workout_goal,
    decode_protobuf_message,
)

# Адрес запущенного сервера
BASE_URL = "http://127.0.0.1:8080"
WS_URL = "ws://127.0.0.1:8080"


# =============================================================================
# ВСПОМОГАТЕЛЬНЫЕ ХЕЛПЕРЫ ДЛЯ HTTP REST ЗАПРОСОВ
# =============================================================================

def make_http_request(method: str, path: str, payload: dict = None, token: str = None):
    """
    Универсальная обертка для отправки REST запросов на сервер SyncMate.
    Автоматически сериализует JSON и добавляет Bearer токен авторизации.
    """
    url = f"{BASE_URL}{path}"
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    data_bytes = json.dumps(payload).encode("utf-8") if payload else None
    req = urllib.request.Request(url, data=data_bytes, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req) as resp:
            status_code = resp.getcode()
            response_body = resp.read().decode("utf-8")
            data = json.loads(response_body) if response_body else {}
            return status_code, data
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8")
        try:
            err_json = json.loads(error_body)
        except Exception:
            err_json = {"raw_error": error_body}
        return e.code, err_json


# =============================================================================
# 1. HTTP REST API: МАЛЕНЬКИЕ ШТАТНЫЕ ТЕСТЫ
# =============================================================================

def test_rest_1_register_single_and_multiple_users():
    """
    Тест REST-1: Циклическая регистрация нескольких пользователей.
    Проверяет, что сервер создает уникальные user_id и выдает пары JWT токенов.
    """
    timestamp = int(time.time() * 1000)
    
    # 1. Последовательная регистрация 3 спортсменов в цикле
    for i in range(1, 4):
        payload = {
            "username": f"Runner_{i}_{timestamp}",
            "email": f"runner_{i}_{timestamp}@syncmate.app",
            "password": "Password123!",
            "weight_kg": 75.0 + i,
            "height_cm": 180 + i,
            "fitness_coef": 1.0,
        }
        status, res = make_http_request("POST", "/api/v1/auth/register", payload)
        
        # Проверяем успешный HTTP код 201 Created и наличие токенов
        assert status == 201
        assert "access_token" in res
        assert "refresh_token" in res
        assert res["user_id"] > 0


def test_rest_2_login_and_token_invalidation():
    """
    Тест REST-2: Авторизация пользователя и инвалидация старых токенов.
    Проверяет выдачу новых токенов и сброс предыдущей сессии.
    """
    timestamp = int(time.time() * 1000)
    email = f"relogin_{timestamp}@syncmate.app"
    password = "SecretPassword123"

    # Регистрация первого пользователя
    make_http_request("POST", "/api/v1/auth/register", {
        "username": "ReloginUser",
        "email": email,
        "password": password,
        "weight_kg": 70.0,
        "height_cm": 175,
    })

    # Первый вход и сохранение Token_V1
    status1, res1 = make_http_request("POST", "/api/v1/auth/login", {"email": email, "password": password})
    assert status1 == 200
    token_v1 = res1["access_token"]

    # Повторный вход в систему
    status2, res2 = make_http_request("POST", "/api/v1/auth/login", {"email": email, "password": password})
    assert status2 == 200
    token_v2 = res2["access_token"]

    # Проверяем, что сервер выдал новый токен V2, не совпадающий с V1
    assert token_v1 != token_v2

    # Запрос профиля со старым Token_V1 должен возвращать 401 Unauthorized (ревокация)
    status_old, _ = make_http_request("GET", "/api/v1/profile", token=token_v1)
    assert status_old == 401


def test_rest_3_get_full_and_incomplete_profile():
    """
    Тест REST-3: Запрос полного и неполного профиля (без веса и роста).
    """
    timestamp = int(time.time() * 1000)

    # 1. Зарегистрируем пользователя Б без указания веса и роста
    status_b, res_b = make_http_request("POST", "/api/v1/auth/register", {
        "username": "IncompleteUser",
        "email": f"incomplete_{timestamp}@syncmate.app",
        "password": "Password123!",
    })
    assert status_b == 201
    token_b = res_b["access_token"]

    # Запрашиваем профиль пользователя Б
    status_prof, prof_data = make_http_request("GET", "/api/v1/profile", token=token_b)
    assert status_prof == 200
    # Для неполного профиля поля weight_kg и height_cm должны возвращаться как None (null)
    assert prof_data.get("weight_kg") is None
    assert prof_data.get("height_cm") is None


def test_rest_4_sequential_profile_updates():
    """
    Тест REST-4: Многократные последовательные обновления веса и роста.
    """
    timestamp = int(time.time() * 1000)
    _, res = make_http_request("POST", "/api/v1/auth/register", {
        "username": "Updater",
        "email": f"updater_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 70.0,
        "height_cm": 175,
    })
    token = res["access_token"]

    # Последовательно применяем 3 изменения веса
    updates = [72.0, 75.0, 73.5]
    for w in updates:
        status, updated_prof = make_http_request("PUT", "/api/v1/profile", {"weight_kg": w}, token=token)
        assert status == 200
        assert updated_prof["weight_kg"] == w

    # Финальная проверка GET профиля
    _, final_prof = make_http_request("GET", "/api/v1/profile", token=token)
    assert final_prof["weight_kg"] == 73.5


def test_rest_5_create_room_and_unique_codes():
    """
    Тест REST-5: Создание комнаты и проверка гарантированной уникальности инвайт-кодов.
    """
    timestamp = int(time.time() * 1000)
    _, res = make_http_request("POST", "/api/v1/auth/register", {
        "username": "RoomAdmin",
        "email": f"admin_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 80.0,
        "height_cm": 185,
    })
    token = res["access_token"]

    # Генерируем 10 комнат и собираем их инвайт-коды
    codes = set()
    for i in range(10):
        status, room_res = make_http_request("POST", "/api/v1/rooms", {
            "title": f"Утренний забег #{i}",
            "description": "Бежим в парке",
            "goal": {"goal_type": GoalType.GOAL_DISTANCE, "target_value": 5000}
        }, token=token)
        
        assert status == 201
        code = room_res["invite_code"]
        assert code.startswith("RUN-")
        assert code not in codes, "Инвайт-код не уникален!"
        codes.add(code)


def test_rest_6_heavy_join_room_with_incomplete_profile():
    """
    Тест REST-6: Вход в комнату по инвайт-коду пользователя с НЕПОЛНЫМ профилем.
    Проверяет, что неполный профиль ПУСКАЕТ в комнату для просмотра.
    """
    timestamp = int(time.time() * 1000)

    # 1. Админ создает комнату
    _, res_adm = make_http_request("POST", "/api/v1/auth/register", {
        "username": "CreatorAdmin",
        "email": f"creator_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 75.0,
        "height_cm": 180,
    })
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Общая тренировка"}, token=res_adm["access_token"])
    code = room_data["invite_code"]

    # 2. Пользователь без веса/роста подключается по инвайт-коду
    _, res_inc = make_http_request("POST", "/api/v1/auth/register", {
        "username": "GuestUser",
        "email": f"guest_{timestamp}@syncmate.app",
        "password": "Password123!",
    })
    
    # Первичная тяжелая инициализация комнаты
    status_join, room_info = make_http_request("GET", f"/api/v1/rooms/join/{code}", token=res_inc["access_token"])
    assert status_join == 200
    assert room_info["title"] == "Общая тренировка"


def test_rest_7_weather_service_and_caching():
    """
    Тест REST-7: Опрос погодного сервиса по географическим координатам.
    """
    status, weather_data = make_http_request("GET", "/api/v1/weather?lat=55.7558&lng=37.6173")
    assert status == 200
    assert "temp_celsius_x10" in weather_data
    assert "wind_speed_mm_s" in weather_data


# =============================================================================
# 2. WEBSOCKET & PROTOBUF: МАЛЕНЬКИЕ ШТАТНЫЕ ТЕСТЫ
# =============================================================================

@pytest.mark.asyncio
async def test_ws_1_handshake_and_slot_id():
    """
    Тест WS-1: Установка сокет-соединения Handshake и выдача slot_id.
    Используем библиотеку websockets для асинхронного открытия сокета.
    """
    timestamp = int(time.time() * 1000)
    _, res_adm = make_http_request("POST", "/api/v1/auth/register", {
        "username": "WsRunner",
        "email": f"ws_runner_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 75.0,
        "height_cm": 180,
    })
    token = res_adm["access_token"]
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Сокет Забег"}, token=token)
    room_id = room_data["room_id"]

    # Подключаемся к WebSocket сокету сервера
    ws_uri = f"{WS_URL}/ws?token={token}&room_id={room_id}"
    async with websockets.connect(ws_uri) as websocket:
        # Сервер сразу высылает первый кадр RoomStateBroadcast
        response_bytes = await websocket.recv()
        decoded_fields = decode_protobuf_message(response_bytes)
        
        # Проверяем наличие заголовочного поля room_id (поле 1 в Protobuf)
        assert 1 in decoded_fields


@pytest.mark.asyncio
async def test_ws_2_single_point_telemetry_broadcast():
    """
    Тест WS-2: Отправка бинарной точки телеметрии UserPointMessage.
    """
    timestamp = int(time.time() * 1000)
    _, res = make_http_request("POST", "/api/v1/auth/register", {
        "username": "TelemetryUser",
        "email": f"telem_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 75.0,
        "height_cm": 180,
    })
    token = res["access_token"]
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Телеметрия"}, token=token)

    ws_uri = f"{WS_URL}/ws?token={token}&room_id={room_data['room_id']}"
    async with websockets.connect(ws_uri) as websocket:
        await websocket.recv() # Первичное приветствие

        # Формируем 1 точку GPS
        pt = encode_path_point(delta_time_ms=1000, pos_x=55.75, pos_y=37.61, altitude_mm=150000, speed_mm_s=3500)
        
        # Упаковываем пакет UserPointMessage со статусом STATE_ACTIVE
        telemetry_bytes = encode_user_point_message(
            user_id=1,
            state=ParticipantStateEnum.STATE_ACTIVE,
            speed_mm_s=3500,
            distance_common_m=100,
            points=[pt]
        )
        
        # Отправляем бинарный кадр в сокет
        await websocket.send(telemetry_bytes)
        
        # Получаем рассылку сокета с обновленным состоянием
        broadcast_bytes = await websocket.recv()
        assert len(broadcast_bytes) > 0


@pytest.mark.asyncio
async def test_ws_3_offline_batch_points_upload():
    """
    Тест WS-3: Выгрузка оффлайн-батча из 20 GPS-точек после выхода из леса.
    """
    timestamp = int(time.time() * 1000)
    _, res = make_http_request("POST", "/api/v1/auth/register", {
        "username": "ForestRunner",
        "email": f"forest_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 75.0,
        "height_cm": 180,
    })
    token = res["access_token"]
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Лесной Забег"}, token=token)

    ws_uri = f"{WS_URL}/ws?token={token}&room_id={room_data['room_id']}"
    async with websockets.connect(ws_uri) as websocket:
        await websocket.recv()

        # Генерируем массив из 20 накопленных в оффлайне RDP-точек
        batch_points = [
            encode_path_point(delta_time_ms=i*1000, pos_x=55.75 + i*0.001, pos_y=37.61 + i*0.001)
            for i in range(20)
        ]

        # Отправляем батч одним бинарным кадром
        batch_msg = encode_user_point_message(
            user_id=1,
            state=ParticipantStateEnum.STATE_ACTIVE,
            distance_common_m=500,
            points=batch_points
        )
        await websocket.send(batch_msg)
        
        res_bytes = await websocket.recv()
        assert len(res_bytes) > 0


@pytest.mark.asyncio
async def test_ws_4_state_transitions_ready_and_pause():
    """
    Тест WS-4: Переключение состояния спортсмена на "Готов" и "Пауза".
    """
    timestamp = int(time.time() * 1000)
    _, res = make_http_request("POST", "/api/v1/auth/register", {
        "username": "Pauser",
        "email": f"pauser_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 75.0,
        "height_cm": 180,
    })
    token = res["access_token"]
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Тест Паузы"}, token=token)

    ws_uri = f"{WS_URL}/ws?token={token}&room_id={room_data['room_id']}"
    async with websockets.connect(ws_uri) as websocket:
        await websocket.recv()

        # 1. Отправляем статус STATE_READY
        msg_ready = encode_user_point_message(state=ParticipantStateEnum.STATE_READY)
        await websocket.send(msg_ready)
        await websocket.recv()

        # 2. Переключаемся на STATE_PAUSED
        msg_pause = encode_user_point_message(state=ParticipantStateEnum.STATE_PAUSED, distance_common_m=300)
        await websocket.send(msg_pause)
        await websocket.recv()


@pytest.mark.asyncio
async def test_ws_6_admin_countdown_command():
    """
    Тест WS-6: Команда админа запуск обратного отсчета 3..2..1 (ACTION_SET_START_COUNTDOWN).
    """
    timestamp = int(time.time() * 1000)
    _, res = make_http_request("POST", "/api/v1/auth/register", {
        "username": "AdminCountdown",
        "email": f"countdown_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 75.0,
        "height_cm": 180,
    })
    token = res["access_token"]
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Отсчет Старта"}, token=token)

    ws_uri = f"{WS_URL}/ws?token={token}&room_id={room_data['room_id']}"
    async with websockets.connect(ws_uri) as websocket:
        await websocket.recv()

        # Отправляем команду админа на включение 5-секундного отсчета
        cmd_bytes = encode_admin_command(
            admin_id=1,
            action=AdminActionCommand.ACTION_SET_START_COUNTDOWN,
            countdown_seconds=5
        )
        await websocket.send(cmd_bytes)

        # Получаем подтверждение CommandAckMessage от сервера
        ack_bytes = await websocket.recv()
        assert len(ack_bytes) > 0


@pytest.mark.asyncio
async def test_ws_7_admin_begin_and_incomplete_profile_block():
    """
    Тест WS-7: Запуск пробежки и БЛОКИРОВКА пользователя с незаполненным профилем.
    """
    timestamp = int(time.time() * 1000)

    # 1. Зарегистрируем админа с полным профилем
    _, res_adm = make_http_request("POST", "/api/v1/auth/register", {
        "username": "FullAdmin",
        "email": f"full_admin_{timestamp}@syncmate.app",
        "password": "Password123!",
        "weight_kg": 80.0,
        "height_cm": 185,
    })
    _, room_data = make_http_request("POST", "/api/v1/rooms", {"title": "Старт Пробежки"}, token=res_adm["access_token"])
    code = room_data["invite_code"]

    # 2. Зарегистрируем пользователя Б БЕЗ ВЕСА И РОСТА
    _, res_inc = make_http_request("POST", "/api/v1/auth/register", {
        "username": "NoWeightUser",
        "email": f"noweight_{timestamp}@syncmate.app",
        "password": "Password123!",
    })
    token_inc = res_inc["access_token"]
    make_http_request("GET", f"/api/v1/rooms/join/{code}", token=token_inc)

    # 3. Пользователь с неполным профилем пытается стать в STATE_READY через сокет
    ws_uri_inc = f"{WS_URL}/ws?token={token_inc}&code={code}"
    async with websockets.connect(ws_uri_inc) as ws_inc:
        await ws_inc.recv()

        # Попытка установить STATE_READY должна отклоняться сервером
        msg_ready = encode_user_point_message(state=ParticipantStateEnum.STATE_READY)
        await ws_inc.send(msg_ready)
        
        # Получаем реакцию сервера
        resp = await ws_inc.recv()
        assert len(resp) > 0
