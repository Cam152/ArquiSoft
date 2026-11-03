package com.arquisoft.voterservice.service;

import com.arquisoft.voterservice.repository.BallotRepository;

import org.springframework.stereotype.Service;

/**
 * Reglas de participación: un votante vota una sola vez en cada elección.
 */
@Service
public class VoterService {

    private final BallotRepository ballots;

    public VoterService(BallotRepository ballots) {
        this.ballots = ballots;
    }

    /** Marca la participación. Devuelve false si ya había votado en esa elección. */
    public boolean markVoted(long voterId, long electionId) {
        return ballots.insert(voterId, electionId) == 1;
    }

    /** Compensación de la saga. Idempotente: desmarcar algo que no existe no es error. */
    public void unmarkVoted(long voterId, long electionId) {
        ballots.delete(voterId, electionId);
    }
}
