package com.syncmate.dto;

import java.time.Instant;

public record PingResponse(
        String status,
        String message,
        Instant timestamp
) {
    public PingResponse(String status, String message) {
        this(status, message, Instant.now());
    }
}

