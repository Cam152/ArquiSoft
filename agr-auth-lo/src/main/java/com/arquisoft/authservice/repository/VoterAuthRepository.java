package com.arquisoft.authservice.repository;

import com.arquisoft.authservice.model.VoterAuth;
import jakarta.persistence.LockModeType;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.Optional;

public interface VoterAuthRepository extends JpaRepository<VoterAuth, Long> {

    Optional<VoterAuth> findByDocument(String document);

    boolean existsByDocument(String document);

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("SELECT v FROM VoterAuth v WHERE v.document = :document")
    Optional<VoterAuth> findByDocumentForUpdate(
            @Param("document") String document
    );
}
