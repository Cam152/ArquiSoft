package com.arquisoft.voterservice.repository;

import com.arquisoft.voterservice.model.Voter;
import org.springframework.data.jpa.repository.*;
import org.springframework.data.repository.query.Param;

import java.util.Optional;

public interface VoterRepository extends JpaRepository<Voter, Long> {

    Optional<Voter> findByDocument(String document);

    @Modifying
    @Query("""
                UPDATE Voter v
                SET v.hasVoted = true
                WHERE v.id = :id
                AND v.hasVoted = false
            """)
    int markVoted(@Param("id") Long id);

    @Modifying
    @Query("""
                UPDATE Voter v
                SET v.hasVoted = false
                WHERE v.id = :id
            """)
    int unmarkVoted(@Param("id") Long id);
}