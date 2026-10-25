package com.syncmate.service;

import com.syncmate.dto.CreateRoomRequest;
import com.syncmate.dto.RoomResponse;
import com.syncmate.exception.InvalidRoomStateException;
import com.syncmate.exception.RoomNotFoundException;
import com.syncmate.exception.UserBannedException;
import com.syncmate.model.Room;
import com.syncmate.model.RoomStatus;
import com.syncmate.model.User;
import com.syncmate.repository.RoomRepository;
import com.syncmate.repository.UserRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.UUID;

@Service
public class RoomService {

    private final RoomRepository roomRepository;
    private final UserRepository userRepository;

    // Dependency Injection через конструктор
    public RoomService(RoomRepository roomRepository, UserRepository userRepository) {
        this.roomRepository = roomRepository;
        this.userRepository = userRepository;
    }

    @Transactional
    public RoomResponse createRoom(CreateRoomRequest request) {
        User owner = userRepository.findById(request.ownerId())
                .orElseThrow(() -> new IllegalArgumentException("Владелец не найден: ID " + request.ownerId()));

        // Генерация компактного инвайт-кода
        String inviteCode = "RUN-" + UUID.randomUUID().toString().substring(0, 4).toUpperCase();

        Room room = new Room(request.title(), inviteCode, owner);
        Room savedRoom = roomRepository.save(room);

        return RoomResponse.fromEntity(savedRoom);
    }

    @Transactional(readOnly = true)
    public RoomResponse getRoomByCode(String inviteCode) {
        Room room = roomRepository.findByInviteCode(inviteCode)
                .orElseThrow(() -> new RoomNotFoundException("Комната с кодом " + inviteCode + " не найдена"));

        return RoomResponse.fromEntity(room);
    }

    @Transactional
    public RoomResponse joinRoom(String inviteCode, Long userId) {
        Room room = roomRepository.findByInviteCode(inviteCode)
                .orElseThrow(() -> new RoomNotFoundException("Комната не найдена: " + inviteCode));

        User user = userRepository.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("Пользователь не найден: ID " + userId));

        // Проверка: не заблокирован ли участник
        if (room.isBanned(user)) {
            throw new UserBannedException("Вы заблокированы в этой комнате");
        }

        // Проверка: можно ли войти в комнату
        if (room.getStatus() != RoomStatus.WAITING) {
            throw new InvalidRoomStateException("Нельзя присоединиться: тренировка уже началась или завершена");
        }

        room.getParticipants().add(user);
        return RoomResponse.fromEntity(room);
    }

    @Transactional
    public RoomResponse banUser(Long roomId, Long adminId, Long targetUserId) {
        Room room = roomRepository.findById(roomId)
                .orElseThrow(() -> new RoomNotFoundException("Комната с ID " + roomId + " не найдена"));

        if (!room.getOwner().getId().equals(adminId)) {
            throw new IllegalArgumentException("Только владелец комнаты может блокировать участников");
        }

        User targetUser = userRepository.findById(targetUserId)
                .orElseThrow(() -> new IllegalArgumentException("Пользователь не найден: ID " + targetUserId));

        room.getParticipants().remove(targetUser);
        room.getBannedUsers().add(targetUser);

        return RoomResponse.fromEntity(room);
    }

    @Transactional
    public RoomResponse startWorkout(Long roomId, Long adminId) {
        Room room = roomRepository.findById(roomId)
                .orElseThrow(() -> new RoomNotFoundException("Комната с ID " + roomId + " не найдена"));

        if (!room.getOwner().getId().equals(adminId)) {
            throw new IllegalArgumentException("Только владелец комнаты может запустить тренировку");
        }

        if (room.getStatus() != RoomStatus.WAITING) {
            throw new InvalidRoomStateException("Тренировка уже запущена или завершена");
        }

        room.setStatus(RoomStatus.TRAINING);
        return RoomResponse.fromEntity(room);
    }
}