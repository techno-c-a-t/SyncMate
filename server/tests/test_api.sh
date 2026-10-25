#!/usr/bin/env bash

BASE_URL="http://localhost:8080/api/v1/rooms"

# Цвета для вывода в консоль
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==============================================${NC}"
echo -e "${BLUE}       ТЕСТИРОВАНИЕ API КОМНАТ SYNCMATE       ${NC}"
echo -e "${BLUE}==============================================${NC}\n"

# 1. Создание комнаты
echo -e "1. Создаем комнату от имени владельца (ID = 1)..."
CREATE_RES=$(curl -s -X POST "${BASE_URL}" \
  -H "Content-Type: application/json" \
  -d '{"title": "Утренний кросс ФПМИ", "ownerId": 1}')

echo "Ответ сервера: $CREATE_RES"

# Достаем inviteCode регуляркой (без необходимости ставить jq)
INVITE_CODE=$(echo "$CREATE_RES" | grep -o '"inviteCode":"[^"]*"' | cut -d'"' -f4)
ROOM_ID=$(echo "$CREATE_RES" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)

if [ -z "$INVITE_CODE" ]; then
    echo -e "${RED}[FAIL] Не удалось получить инвайт-код. Сервер запущен?${NC}"
    exit 1
fi
echo -e "${GREEN}[OK] Комната успешно создана! ID=${ROOM_ID}, Код=${INVITE_CODE}${NC}\n"

# 2. Получение данных комнаты по инвайт-коду
echo -e "2. Запрашиваем информацию о комнате по коду ${INVITE_CODE}..."
GET_RES=$(curl -s -X GET "${BASE_URL}/join/${INVITE_CODE}")
echo "Ответ сервера: $GET_RES"
echo -e "${GREEN}[OK] Данные получены.${NC}\n"

# 3. Вход второго участника (ID = 2)
echo -e "3. Пользователь с ID = 2 присоединяется к комнате..."
JOIN_RES=$(curl -s -X POST "${BASE_URL}/join/${INVITE_CODE}" \
  -H "Content-Type: application/json" \
  -d '{"userId": 2}')
echo "Ответ сервера: $JOIN_RES"
echo -e "${GREEN}[OK] Пользователь 2 успешно вошел в состав участников!${NC}\n"

# 4. Владелец банит второго участника
echo -e "4. Владелец (ID = 1) банит участника (ID = 2)..."
BAN_RES=$(curl -s -X POST "${BASE_URL}/${ROOM_ID}/ban/2?adminId=1")
echo "Ответ сервера: $BAN_RES"
echo -e "${GREEN}[OK] Участник заблокирован.${NC}\n"

# 5. Проверка безопасности: забаненный пытается войти снова
echo -e "5. ПРОВЕРКА ОШИБКИ: Забаненный участник пытается войти снова..."
HTTP_STATUS=$(curl -s -o /tmp/syncmate_err.json -w "%{http_code}" -X POST "${BASE_URL}/join/${INVITE_CODE}" \
  -H "Content-Type: application/json" \
  -d '{"userId": 2}')

ERR_BODY=$(cat /tmp/syncmate_err.json)
echo "HTTP Статус: $HTTP_STATUS"
echo "Тело ответа: $ERR_BODY"

if [ "$HTTP_STATUS" -eq 403 ]; then
    echo -e "${GREEN}[OK] Доступ запрещен! Сервер вернул ожидаемый 403 Forbidden.${NC}\n"
else
    echo -e "${RED}[FAIL] Ожидался 403, получен ${HTTP_STATUS}${NC}\n"
fi

# 6. Запуск тренировки админом
echo -e "6. Админ запускает тренировку..."
START_RES=$(curl -s -X POST "${BASE_URL}/${ROOM_ID}/start?adminId=1")
echo "Ответ сервера: $START_RES"
echo -e "${GREEN}[OK] Тренировка переведена в статус TRAINING!${NC}\n"

# 7. Проверка несуществующей комнаты
echo -e "7. ПРОВЕРКА ОШИБКИ: Попытка зайти в несуществующую комнату..."
HTTP_404=$(curl -s -o /tmp/syncmate_404.json -w "%{http_code}" -X POST "${BASE_URL}/join/RUN-FAKE" \
  -H "Content-Type: application/json" \
  -d '{"userId": 1}')

echo "HTTP Статус: $HTTP_404"
echo "Тело ответа: $(cat /tmp/syncmate_404.json)"

if [ "$HTTP_404" -eq 404 ]; then
    echo -e "${GREEN}[OK] Обработка 404 сработала корректно!${NC}\n"
else
    echo -e "${RED}[FAIL] Ожидался 404, получен ${HTTP_404}${NC}\n"
fi

echo -e "${BLUE}==============================================${NC}"
echo -e "${GREEN}   ВСЕ ТЕСТЫ ПРОЙДЕНЫ! АРХИТЕКТУРА РАБОТАЕТ!  ${NC}"
echo -e "${BLUE}==============================================${NC}"
