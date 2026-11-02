package com.arquisoft.voterservice.controller;

import com.arquisoft.voterservice.dto.ParticipationRequest;
import com.arquisoft.voterservice.service.VoterService;

import jakarta.validation.Valid;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

/**
 * Participación del votante. Solo lo llama agr-gateway-or, con X-Service-Key
 * (lo verifica ServiceKeyInterceptor antes de llegar aquí).
 */
@RestController
@RequestMapping("/voters")
public class VoterController {

    private final VoterService voterService;

    public VoterController(VoterService voterService) {
        this.voterService = voterService;
    }

    @PostMapping("/mark-voted")
    public ResponseEntity<Map<String, String>> markVoted(@Valid @RequestBody ParticipationRequest request) {
        if (!voterService.markVoted(request.voterId(), request.electionId())) {
            return ResponseEntity
                    .status(HttpStatus.CONFLICT)
                    .body(Map.of("detail", "El votante ya votó en esta elección"));
        }
        return ResponseEntity.ok(Map.of("status", "marked"));
    }

    @PostMapping("/unmark-voted")
    public Map<String, String> unmarkVoted(@Valid @RequestBody ParticipationRequest request) {
        voterService.unmarkVoted(request.voterId(), request.electionId());
        return Map.of("status", "unmarked");
    }
}
