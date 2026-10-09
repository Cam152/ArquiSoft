package com.arquisoft.voterservice.service;

import com.arquisoft.voterservice.model.Voter;
import com.arquisoft.voterservice.repository.VoterRepository;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class VoterService {

    private final VoterRepository voterRepository;

    public VoterService(VoterRepository voterRepository) {
        this.voterRepository = voterRepository;
    }

    public Voter findByDocument(String document) {
        return voterRepository
                .findByDocument(document)
                .orElse(null);
    }

    @Transactional
    public boolean markVoted(Long voterId) {

        int updated = voterRepository.markVoted(voterId);

        return updated == 1;
    }

    @Transactional
    public void unmarkVoted(Long voterId) {

        voterRepository.unmarkVoted(voterId);
    }
}