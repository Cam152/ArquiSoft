package com.arquisoft.voterservice.dto;

import jakarta.validation.constraints.NotBlank;

public record LoginRequest(
        @NotBlank String document,
        @NotBlank String password) {
}