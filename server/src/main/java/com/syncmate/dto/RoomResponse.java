package com.syncmate.dto;

import com.syncmate.model.Room;
import com.syncmate.model.RoomStatus;
import java.util.List;

public record RoomResponse(
    Long id,
    String title,
    String inviteCode,
    RoomStatus status,
    String ownerUsername,
    List<String> participantUsernames
) {
    // Удобный статический конструктор из Entity в DTO
    public static RoomResponse fromEntity(Room room) {
        return new RoomResponse(
            room.getId(),
            room.getTitle(),
            room.getInviteCode(),
            room.getStatus(),
            room.getOwner().getUsername(),
            room.getParticipants().stream().map(u -> u.getUsername()).toList()
        );
    }
}