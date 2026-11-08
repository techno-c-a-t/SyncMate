#!/usr/bin/env python3
"""
SyncMate Auth & Ping E2E Test Suite
Тестирование эндпоинтов авторизации, JWT и проверки ping/auth_ping.
Не требует сторонних зависимостей (использует urllib).
"""

import json
import sys
import urllib.error
import urllib.request

BASE_URL = "http://localhost:8080/api/v1"

# ANSI цвета для консоли
GREEN = "\033[0;32m"
RED = "\033[0;31m"
BLUE = "\033[0;34m"
YELLOW = "\033[1;33m"
NC = "\033[0m"


def request(method: str, path: str, payload=None, token: str = None):
    url = f"{BASE_URL}{path}"
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req) as resp:
            status = resp.status
            body = resp.read().decode("utf-8")
            parsed = json.loads(body) if body else {}
            return status, parsed
    except urllib.error.HTTPError as e:
        status = e.code
        body = e.read().decode("utf-8")
        try:
            parsed = json.loads(body) if body else {}
        except Exception:
            parsed = {"raw": body}
        return status, parsed
    except Exception as e:
        print(f"{RED}[ERROR] Ошибка соединения с сервером {url}: {e}{NC}")
        print(f"{YELLOW}Убедитесь, что сервер запущен: ./gradlew bootRun{NC}")
        sys.exit(1)


def assert_test(step_num: int, title: str, condition: bool, details: str = ""):
    print(f"\n{BLUE}[Тест {step_num}] {title}{NC}")
    if condition:
        print(f"{GREEN}✓ PASS: {details}{NC}")
    else:
        print(f"{RED}✗ FAIL: {details}{NC}")
        sys.exit(1)


def main():
    print(f"{BLUE}=============================================={NC}")
    print(f"{BLUE}    ТЕСТИРОВАНИЕ АВТОРИЗАЦИИ И PING SYNCMATE  {NC}")
    print(f"{BLUE}=============================================={NC}")

    # 1. Публичный ping без авторизации
    status, res = request("GET", "/ping")
    assert_test(
        1,
        "Публичный GET /api/v1/ping",
        status == 200 and res.get("message") == "pong",
        f"Статус {status}, ответ: {res}",
    )

    # 2. Закрытый auth_ping БЕЗ токена -> Ожидаем 401 или 403
    status, res = request("GET", "/auth_ping")
    assert_test(
        2,
        "Защищенный GET /api/v1/auth_ping без токена",
        status in (401, 403),
        f"Доступ запрещен (HTTP {status}), ответ: {res}",
    )

    # 3. Логин предзаполненного пользователя alex_runner
    status, res = request(
        "POST",
        "/auth/login",
        payload={"username": "alex_runner", "password": "password123"},
    )
    token_alex = res.get("token")
    assert_test(
        3,
        "Логин существующего пользователя (alex_runner)",
        status == 200 and bool(token_alex),
        f"Получен JWT токен (длина: {len(token_alex) if token_alex else 0}), userId={res.get('userId')}",
    )

    # 4. Проверка auth_ping С валидным токеном alex_runner
    status, res = request("GET", "/auth_ping", token=token_alex)
    assert_test(
        4,
        "GET /api/v1/auth_ping с валидным токеном alex_runner",
        status == 200
        and res.get("username") == "alex_runner"
        and res.get("message") == "auth_pong",
        f"Успешная аутентификация: userId={res.get('userId')}, username={res.get('username')}",
    )

    # 5. Регистрация нового бегуна
    import time

    new_username = f"runner_{int(time.time())}"
    status, res = request(
        "POST",
        "/auth/register",
        payload={"username": new_username, "password": "securePass2026!"},
    )
    token_new = res.get("token")
    assert_test(
        5,
        f"Регистрация нового пользователя ({new_username})",
        status == 201 and bool(token_new),
        f"Пользователь создан, получен токен, userId={res.get('userId')}",
    )

    # 6. Проверка auth_ping для нового бегуна
    status, res = request("GET", "/auth_ping", token=token_new)
    assert_test(
        6,
        "GET /api/v1/auth_ping с токеном нового пользователя",
        status == 200 and res.get("username") == new_username,
        f"Токен принадлежит новому пользователю: {res.get('username')}",
    )

    # 7. Защита: Попытка доступа с испорченным / фальшивым токеном
    status, res = request("GET", "/auth_ping", token="fake.invalid.jwt_token")
    assert_test(
        7,
        "GET /api/v1/auth_ping с поддельным токеном",
        status in (401, 403),
        f"Сервер отклонил поддельный токен (HTTP {status})",
    )

    # 8. Защита: Логин с неверным паролем
    status, res = request(
        "POST",
        "/auth/login",
        payload={"username": "alex_runner", "password": "WRONG_PASSWORD_123"},
    )
    assert_test(
        8,
        "Логин с неверным паролем",
        status == 401,
        f"HTTP статус {status}, код ошибки: {res.get('errorCode')}",
    )

    # 9. Защита: Повторная регистрация существующего никнейма
    status, res = request(
        "POST",
        "/auth/register",
        payload={"username": "alex_runner", "password": "password123"},
    )
    assert_test(
        9,
        "Повторная регистрация занятого username",
        status == 400,
        f"HTTP статус {status}, сообщение: {res.get('message')}",
    )

    print(f"\n{BLUE}=============================================={NC}")
    print(f"{GREEN}   ВСЕ 9 ТЕСТОВ АВТОРИЗАЦИИ УСПЕШНО ПРОЙДЕНЫ!  {NC}")
    print(f"{BLUE}=============================================={NC}\n")


if __name__ == "__main__":
    main()

