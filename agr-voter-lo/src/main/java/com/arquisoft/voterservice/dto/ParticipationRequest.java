package com.arquisoft.voterservice.dto;

import com.fasterxml.jackson.annotation.JsonProperty;

import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Positive;

/** Cuerpo de mark-voted y unmark-voted: {"voter_id": 1, "election_id": 1}. */
public record ParticipationRequest(
        @JsonProperty("voter_id") @NotNull @Positive Long voterId,
        @JsonProperty("election_id") @NotNull @Positive Long electionId) {
}
