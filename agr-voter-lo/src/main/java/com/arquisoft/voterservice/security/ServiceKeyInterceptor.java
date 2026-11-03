package com.arquisoft.voterservice.security;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.springframework.web.servlet.HandlerInterceptor;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;

/**
 * Exige X-Service-Key igual a SERVICE_API_KEY en /voters/**. Solo el gateway la
 * tiene. Corre antes de leer el cuerpo, así una llave incorrecta siempre da 401.
 */
@Component
public class ServiceKeyInterceptor implements HandlerInterceptor {

    public static final String HEADER = "X-Service-Key";

    private final byte[] expected;

    public ServiceKeyInterceptor(@Value("${service.api-key}") String serviceApiKey) {
        this.expected = serviceApiKey == null ? new byte[0] : serviceApiKey.getBytes(StandardCharsets.UTF_8);
    }

    @Override
    public boolean preHandle(HttpServletRequest request, HttpServletResponse response, Object handler) {
        String received = request.getHeader(HEADER);
        // Sin llave configurada no se acepta ninguna. Comparación de tiempo constante.
        if (expected.length == 0 || received == null
                || !MessageDigest.isEqual(expected, received.getBytes(StandardCharsets.UTF_8))) {
            throw new InvalidServiceKeyException();
        }
        return true;
    }

    public static class InvalidServiceKeyException extends RuntimeException {
        public InvalidServiceKeyException() {
            super("Llave de servicio incorrecta");
        }
    }
}
