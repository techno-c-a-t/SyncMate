package com.syncmate.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record RegisterRequest(
        @NotBlank(message = "Имя пользователя обязательно")
        @Size(min = 3, max = 32, message = "Имя пользователя должно содержать от 3 до 32 символов")
        String username,

        @NotBlank(message = "Пароль обязателен")
        @Size(min = 6, max = 64, message = "Пароль должен содержать от 6 до 64 символов")
        String password
) {}

