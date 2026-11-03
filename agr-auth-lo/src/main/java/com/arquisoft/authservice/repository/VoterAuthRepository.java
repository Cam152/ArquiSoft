package com.arquisoft.authservice.repository;

import com.arquisoft.authservice.model.VoterAuth;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.Optional;

public interface VoterAuthRepository extends JpaRepository<VoterAuth, Long> {
    Optional<VoterAuth> findByDocument(String document);
    boolean existsByDocument(String document);
}