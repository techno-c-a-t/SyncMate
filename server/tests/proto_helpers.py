"""
Вспомогательный модуль для бинарного кодирования и декодирования пакетов Google Protobuf на чистом Python.
Используется в тестах SyncMate без необходимости внешней зависимости от утилиты protoc.
"""

import struct

# --- Энумы состояний и команд ---
class ParticipantStateEnum:
    STATE_UNKNOWN = 0
    STATE_WAITING = 1
    STATE_READY = 2
    STATE_ACTIVE = 3
    STATE_PAUSED = 4
    STATE_LEFT = 5
    STATE_BANNED = 6

class AdminActionCommand:
    ACTION_UNKNOWN = 0
    ACTION_BEGIN = 1
    ACTION_TERMINATE = 2
    ACTION_BAN = 3
    ACTION_GIVE_ADMIN = 4
    ACTION_TAKE_ADMIN = 5
    ACTION_SET_GOAL = 6
    ACTION_SET_START_COUNTDOWN = 7
    ACTION_UNBAN = 8

class GoalType:
    GOAL_NONE = 0
    GOAL_DISTANCE = 1
    GOAL_TIME = 2
    GOAL_CALORIES = 3

# --- Простейшие Varint и Protobuf сериализаторы ---
def encode_varint(value: int) -> bytes:
    """Упаковывает целое число в формат Protobuf Varint (7 бит на байт)."""
    if value < 0:
        value = (1 << 64) + value
    buf = bytearray()
    while True:
        towrite = value & 0x7f
        value >>= 7
        if value:
            buf.append(towrite | 0x80)
        else:
            buf.append(towrite)
            break
    return bytes(buf)

def decode_varint(data: bytes, offset: int = 0):
    """Декодирует Protobuf Varint из байтового потока."""
    res = 0
    shift = 0
    while True:
        if offset >= len(data):
            raise ValueError("Truncated varint")
        b = data[offset]
        offset += 1
        res |= (b & 0x7f) << shift
        if not (b & 0x80):
            break
        shift += 7
    return res, offset

def encode_key(field_number: int, wire_type: int) -> bytes:
    """Формирует ключ поля Protobuf (Tag = (field_number << 3) | wire_type)."""
    return encode_varint((field_number << 3) | wire_type)

def encode_string(field_number: int, s: str) -> bytes:
    """Упаковывает строковое поле."""
    val_bytes = s.encode('utf-8')
    return encode_key(field_number, 2) + encode_varint(len(val_bytes)) + val_bytes

def encode_bytes(field_number: int, b: bytes) -> bytes:
    """Упаковывает бинарное поле."""
    return encode_key(field_number, 2) + encode_varint(len(b)) + b

def encode_int(field_number: int, val: int) -> bytes:
    """Упаковывает целочисленное поле Varint."""
    if val == 0:
        return b""
    return encode_key(field_number, 0) + encode_varint(val)

def encode_double(field_number: int, val: float) -> bytes:
    """Упаковывает число с плавающей точкой 64-бит (Fixed64)."""
    return encode_key(field_number, 1) + struct.pack("<d", val)

# --- Сериализация Protobuf сообщений SyncMate ---

def encode_path_point(delta_time_ms=0, pos_x=0.0, pos_y=0.0, altitude_mm=0, speed_mm_s=0, slope_deg=0, delta_steps=0) -> bytes:
    """Кодирует точку GPS-маршрута PathPoint."""
    res = bytearray()
    if delta_time_ms: res.extend(encode_int(1, delta_time_ms))
    if pos_x != 0.0: res.extend(encode_double(2, pos_x))
    if pos_y != 0.0: res.extend(encode_double(3, pos_y))
    if altitude_mm: res.extend(encode_int(4, altitude_mm))
    if speed_mm_s: res.extend(encode_int(5, speed_mm_s))
    if slope_deg: res.extend(encode_int(6, slope_deg))
    if delta_steps: res.extend(encode_int(7, delta_steps))
    return bytes(res)

def encode_user_point_message(user_id=0, packet_id=0, state=ParticipantStateEnum.STATE_ACTIVE, 
                              speed_mm_s=0, kal_common=0, distance_common_m=0, 
                              train_time_sec=0, steps=0, altitude_mm=0, points=None) -> bytes:
    """Кодирует пакет телеметрии спортсмена UserPointMessage."""
    res = bytearray()
    if user_id: res.extend(encode_int(1, user_id))
    if packet_id: res.extend(encode_int(2, packet_id))
    if state: res.extend(encode_int(3, state))
    if speed_mm_s: res.extend(encode_int(4, speed_mm_s))
    if kal_common: res.extend(encode_int(5, kal_common))
    if distance_common_m: res.extend(encode_int(6, distance_common_m))
    if train_time_sec: res.extend(encode_int(7, train_time_sec))
    if steps: res.extend(encode_int(8, steps))
    if altitude_mm: res.extend(encode_int(9, altitude_mm))
    if points:
        for pt in points:
            pt_bytes = pt if isinstance(pt, bytes) else encode_path_point(**pt)
            res.extend(encode_bytes(10, pt_bytes))
    return bytes(res)

def encode_workout_goal(goal_type=GoalType.GOAL_NONE, target_value=0) -> bytes:
    """Кодирует структуру цели WorkoutGoal."""
    res = bytearray()
    if goal_type: res.extend(encode_int(1, goal_type))
    if target_value: res.extend(encode_int(2, target_value))
    return bytes(res)

def encode_admin_command(admin_id=0, action=AdminActionCommand.ACTION_UNKNOWN, target_user_id=0, new_goal=None, countdown_seconds=0) -> bytes:
    """Кодирует пакет админской команды AdminCommandMessage."""
    res = bytearray()
    if admin_id: res.extend(encode_int(1, admin_id))
    if action: res.extend(encode_int(2, action))
    if target_user_id: res.extend(encode_int(3, target_user_id))
    if new_goal:
        goal_bytes = new_goal if isinstance(new_goal, bytes) else encode_workout_goal(**new_goal)
        res.extend(encode_bytes(4, goal_bytes))
    if countdown_seconds: res.extend(encode_int(5, countdown_seconds))
    return bytes(res)

# --- Распаковка входящих Protobuf пакетов от сервера ---

def decode_protobuf_message(data: bytes) -> dict:
    """Универсальный парсер бинарных полей Protobuf для тестов."""
    fields = {}
    offset = 0
    length = len(data)
    while offset < length:
        tag, offset = decode_varint(data, offset)
        field_num = tag >> 3
        wire_type = tag & 0x07

        if wire_type == 0: # Varint
            val, offset = decode_varint(data, offset)
        elif wire_type == 1: # Fixed64 / Double
            val = struct.unpack("<d", data[offset:offset+8])[0]
            offset += 8
        elif wire_type == 2: # Length-delimited (String / Bytes / Submessage)
            size, offset = decode_varint(data, offset)
            val = data[offset:offset+size]
            offset += size
        else:
            break

        if field_num not in fields:
            fields[field_num] = []
        fields[field_num].append(val)
    return fields
