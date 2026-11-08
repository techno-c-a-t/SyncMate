package com.syncmate.dto;

public record AuthResponse(
        String token,
        Long userId,
        String username
) {}

