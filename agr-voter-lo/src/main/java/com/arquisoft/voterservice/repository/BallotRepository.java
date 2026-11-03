package com.arquisoft.voterservice.repository;

import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

/**
 * Acceso a la tabla ballots_cast (agr-voters-db).
 */
@Repository
public class BallotRepository {

    private final JdbcTemplate jdbc;

    public BallotRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    /**
     * Inserta la marca de participación. Devuelve las filas insertadas: 1 si quedó
     * marcada, 0 si ya existía. La llave primaria lo hace atómico aunque lleguen
     * dos peticiones al mismo tiempo.
     */
    public int insert(long voterId, long electionId) {
        return jdbc.update(
                "INSERT INTO ballots_cast (voter_id, election_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
                voterId, electionId);
    }

    /** Borra la marca. Devuelve las filas borradas (0 si no existía). */
    public int delete(long voterId, long electionId) {
        return jdbc.update(
                "DELETE FROM ballots_cast WHERE voter_id = ? AND election_id = ?",
                voterId, electionId);
    }
}
