package com.syncmate.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

public record CreateRoomRequest(
    @NotBlank(message = "Название комнаты не может быть пустым")
    String title,

    @NotNull(message = "ID владельца обязателен")
    Long ownerId
) {}