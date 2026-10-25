package com.syncmate.dto;

import java.time.Instant;

public record ErrorResponse(
    String errorCode,
    String message,
    Instant timestamp
) {
    public ErrorResponse(String errorCode, String message) {
        this(errorCode, message, Instant.now());
    }
}