package com.syncmate.dto;

import jakarta.validation.constraints.NotNull;

public record JoinRoomRequest(
    @NotNull(message = "ID пользователя обязателен")
    Long userId
) {}