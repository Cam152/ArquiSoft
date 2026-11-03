package com.arquisoft.authservice.dto;

public record ValidateResponse(boolean valid, Long voterId, String document, String error) {}