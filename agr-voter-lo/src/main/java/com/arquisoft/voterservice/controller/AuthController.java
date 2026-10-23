package com.arquisoft.voterservice.controller;

import com.arquisoft.voterservice.dto.LoginRequest;
import com.arquisoft.voterservice.dto.LoginResponse;
import com.arquisoft.voterservice.model.Voter;
import com.arquisoft.voterservice.security.JwtService;
import com.arquisoft.voterservice.service.VoterService;

import jakarta.validation.Valid;

import org.springframework.http.*;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/auth")
public class AuthController {

    private final VoterService voterService;
    private final JwtService jwtService;

    private final BCryptPasswordEncoder passwordEncoder = new BCryptPasswordEncoder();

    public AuthController(
            VoterService voterService,
            JwtService jwtService) {
        this.voterService = voterService;
        this.jwtService = jwtService;
    }

    @PostMapping("/login")
    public ResponseEntity<?> login(
            @Valid @RequestBody LoginRequest request) {

        Voter voter = voterService.findByDocument(
                request.document());

        if (voter == null ||
                !passwordEncoder.matches(
                        request.password(),
                        voter.getPasswordHash())) {

            return ResponseEntity
                    .status(HttpStatus.UNAUTHORIZED)
                    .body(new ErrorResponse(
                            "Documento o clave incorrectos"));
        }

        String token = jwtService.generateToken(voter.getId());

        return ResponseEntity.ok(
                new LoginResponse(
                        token,
                        "Bearer",
                        jwtService.getExpirationSeconds()));
    }

    public record ErrorResponse(String detail) {
    }
}
