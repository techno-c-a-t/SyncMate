package com.syncmate.controller;

import com.syncmate.dto.CreateRoomRequest;
import com.syncmate.dto.JoinRoomRequest;
import com.syncmate.dto.RoomResponse;
import com.syncmate.service.RoomService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1/rooms")
public class RoomController {

    private final RoomService roomService;

    // Внедрение сервиса через конструктор
    public RoomController(RoomService roomService) {
        this.roomService = roomService;
    }

    @PostMapping
    public ResponseEntity<RoomResponse> createRoom(@Valid @RequestBody CreateRoomRequest request) {
        RoomResponse response = roomService.createRoom(request);
        return ResponseEntity.status(HttpStatus.CREATED).body(response);
    }

    @GetMapping("/join/{inviteCode}")
    public ResponseEntity<RoomResponse> getRoomByInviteCode(@PathVariable String inviteCode) {
        RoomResponse response = roomService.getRoomByCode(inviteCode);
        return ResponseEntity.ok(response);
    }

    @PostMapping("/join/{inviteCode}")
    public ResponseEntity<RoomResponse> joinRoom(
            @PathVariable String inviteCode,
            @Valid @RequestBody JoinRoomRequest request) {
        RoomResponse response = roomService.joinRoom(inviteCode, request.userId());
        return ResponseEntity.ok(response);
    }

    @PostMapping("/{roomId}/ban/{targetUserId}")
    public ResponseEntity<RoomResponse> banUser(
            @PathVariable Long roomId,
            @RequestParam Long adminId,
            @PathVariable Long targetUserId) {
        RoomResponse response = roomService.banUser(roomId, adminId, targetUserId);
        return ResponseEntity.ok(response);
    }

    @PostMapping("/{roomId}/start")
    public ResponseEntity<RoomResponse> startWorkout(
            @PathVariable Long roomId,
            @RequestParam Long adminId) {
        RoomResponse response = roomService.startWorkout(roomId, adminId);
        return ResponseEntity.ok(response);
    }
}