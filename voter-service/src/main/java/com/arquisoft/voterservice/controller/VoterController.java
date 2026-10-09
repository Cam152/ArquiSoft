package com.arquisoft.voterservice.controller;

import com.arquisoft.voterservice.service.VoterService;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.*;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/voters")
public class VoterController {

    private final VoterService voterService;

    @Value("${service.api-key}")
    private String serviceApiKey;

    public VoterController(VoterService voterService) {
        this.voterService = voterService;
    }

    @PostMapping("/mark-voted")
    public ResponseEntity<?> markVoted(
            Authentication authentication,
            @RequestHeader(value = "X-Service-Key", required = false) String serviceKey) {

        validateServiceKey(serviceKey);

        Long voterId = Long.valueOf(authentication.getName());

        boolean marked = voterService.markVoted(voterId);

        if (!marked) {

            return ResponseEntity
                    .status(HttpStatus.CONFLICT)
                    .body(new ErrorResponse(
                            "El votante ya votó"));
        }

        return ResponseEntity.ok(
                new StatusResponse("marked"));
    }

    @PostMapping("/unmark-voted")
    public ResponseEntity<?> unmarkVoted(
            Authentication authentication,
            @RequestHeader(value = "X-Service-Key", required = false) String serviceKey) {

        validateServiceKey(serviceKey);

        Long voterId = Long.valueOf(authentication.getName());

        voterService.unmarkVoted(voterId);

        return ResponseEntity.ok(
                new StatusResponse("unmarked"));
    }

    private void validateServiceKey(String serviceKey) {

        if (serviceKey == null ||
                !serviceApiKey.equals(serviceKey)) {

            throw new ResponseStatusException(
                    HttpStatus.UNAUTHORIZED,
                    "Llave de servicio incorrecta");
        }
    }

    public record StatusResponse(String status) {
    }

    public record ErrorResponse(String detail) {
    }
}