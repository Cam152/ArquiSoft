package com.arquisoft.authservice.service;

import com.arquisoft.authservice.dto.AuthResponse;
import com.arquisoft.authservice.dto.LoginRequest;
import com.arquisoft.authservice.dto.ValidateResponse;
import com.arquisoft.authservice.model.VoterAuth;
import com.arquisoft.authservice.repository.VoterAuthRepository;
import io.jsonwebtoken.Claims;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;

@Service
public class AuthService {

    private final VoterAuthRepository voterRepository;
    private final PasswordEncoder passwordEncoder;
    private final JwtService jwtService;

    public AuthService(VoterAuthRepository voterRepository, 
                       PasswordEncoder passwordEncoder, 
                       JwtService jwtService) {
        this.voterRepository = voterRepository;
        this.passwordEncoder = passwordEncoder;
        this.jwtService = jwtService;
    }

    public AuthResponse login(LoginRequest request) {
        VoterAuth voter = voterRepository.findByDocument(request.document())
                .orElseThrow(() -> new RuntimeException("Credenciales inválidas"));

        if (!passwordEncoder.matches(request.password(), voter.getPasswordHash())) {
            throw new RuntimeException("Credenciales inválidas");
        }

        String token = jwtService.generateToken(voter.getId(), voter.getDocument(), voter.getName());
        return new AuthResponse(token, voter.getId(), voter.getName(), voter.getDocument());
    }

    public ValidateResponse validateToken(String authHeader) {
        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            return new ValidateResponse(false, null, null, "Header de autorización inválido");
        }

        String token = authHeader.substring(7);
        try {
            Claims claims = jwtService.validateAndExtractClaims(token);
            Long voterId = claims.get("voter_id", Long.class);
            String document = claims.getSubject();
            return new ValidateResponse(true, voterId, document, null);
        } catch (Exception e) {
            return new ValidateResponse(false, null, null, "Token expirado o alterado: " + e.getMessage());
        }
    }
}