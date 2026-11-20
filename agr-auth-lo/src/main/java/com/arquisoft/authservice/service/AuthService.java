package com.arquisoft.authservice.service;

import com.arquisoft.authservice.dto.AuthResponse;
import com.arquisoft.authservice.dto.LoginRequest;
import com.arquisoft.authservice.dto.ValidateResponse;
import com.arquisoft.authservice.exception.AccountLockedException;
import com.arquisoft.authservice.exception.InvalidCredentialsException;
import com.arquisoft.authservice.model.VoterAuth;
import com.arquisoft.authservice.repository.VoterAuthRepository;

import io.jsonwebtoken.Claims;

import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;

@Service
public class AuthService {

    private static final int MAX_FAILED_ATTEMPTS = 5;
    private static final int LOCK_DURATION_MINUTES = 15;

    private final VoterAuthRepository voterRepository;
    private final PasswordEncoder passwordEncoder;
    private final JwtService jwtService;

    public AuthService(
            VoterAuthRepository voterRepository,
            PasswordEncoder passwordEncoder,
            JwtService jwtService) {

        this.voterRepository = voterRepository;
        this.passwordEncoder = passwordEncoder;
        this.jwtService = jwtService;
    }

    @Transactional(noRollbackFor = {
            AccountLockedException.class,
            InvalidCredentialsException.class
    })
    public AuthResponse login(LoginRequest request) {

        if (request == null
                || request.document() == null
                || request.password() == null
                || request.document().isBlank()
                || request.password().isBlank()) {

            throw new InvalidCredentialsException(
                    "Documento o clave incorrectos"
            );
        }

        VoterAuth voter = voterRepository
                .findByDocumentForUpdate(request.document())
                .orElseThrow(() -> new InvalidCredentialsException(
                        "Documento o clave incorrectos"
                ));

        LocalDateTime now = LocalDateTime.now();

        // Comprobar si la cuenta esta bloqueada.
        if (voter.getLockedUntil() != null) {

            if (now.isBefore(voter.getLockedUntil())) {

                throw new AccountLockedException(
                        "Cuenta bloqueada temporalmente. Intente nuevamente mas tarde."
                );
            }

            // El periodo de bloqueo termino.
            voter.setLockedUntil(null);
            voter.setFailedAttempts(0);
        }

        // Validar la contraseña.
        if (!passwordEncoder.matches(
                request.password(),
                voter.getPasswordHash())) {

            int attempts = voter.getFailedAttempts() + 1;

            voter.setFailedAttempts(attempts);

            // Bloquear al alcanzar cinco intentos.
            if (attempts >= MAX_FAILED_ATTEMPTS) {

                voter.setLockedUntil(
                        now.plusMinutes(LOCK_DURATION_MINUTES)
                );

                throw new AccountLockedException(
                        "Cuenta bloqueada por cinco intentos fallidos. "
                        + "Intente nuevamente en 15 minutos."
                );
            }

            throw new InvalidCredentialsException(
                    "Documento o clave incorrectos. Intentos restantes: "
                    + (MAX_FAILED_ATTEMPTS - attempts)
            );
        }

        // Inicio de sesion correcto: reiniciar contador.
        voter.setFailedAttempts(0);
        voter.setLockedUntil(null);

        String token = jwtService.generateToken(
                voter.getId(),
                voter.getDocument(),
                voter.getName()
        );

        return new AuthResponse(
                token,
                voter.getId(),
                voter.getName(),
                voter.getDocument()
        );
    }

    public ValidateResponse validateToken(String authHeader) {

        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            return new ValidateResponse(
                    false,
                    null,
                    null,
                    "Header de autorizacion invalido"
            );
        }

        String token = authHeader.substring(7);

        try {
            Claims claims = jwtService.validateAndExtractClaims(token);

            Long voterId = claims.get("voter_id", Long.class);
            String document = claims.getSubject();

            return new ValidateResponse(
                    true,
                    voterId,
                    document,
                    null
            );

        } catch (Exception e) {

            return new ValidateResponse(
                    false,
                    null,
                    null,
                    "Token expirado o alterado"
            );
        }
    }
}
