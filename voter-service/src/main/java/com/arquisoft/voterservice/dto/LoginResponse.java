package com.arquisoft.voterservice.dto;

public record LoginResponse(
        String access_token,
        String token_type,
        long expires_in) {
}