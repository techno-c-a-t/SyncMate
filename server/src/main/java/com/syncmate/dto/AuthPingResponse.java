package com.syncmate.dto;

import java.time.Instant;

public record AuthPingResponse(
        String status,
        String message,
        Long userId,
        String username,
        Instant timestamp
) {
    public AuthPingResponse(String status, String message, Long userId, String username) {
        this(status, message, userId, username, Instant.now());
    }
}

