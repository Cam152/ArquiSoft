package com.arquisoft.authservice.dto;

public record AuthResponse(String token, Long voterId, String name, String document) {}