-- Participación: en qué elecciones ya votó cada votante.
-- voter_id es el "sub" del JWT que emite agr-auth-lo; aquí no hay datos personales.
-- No se guarda fecha ni hora: permitiría cruzar la participación con el voto.
CREATE TABLE IF NOT EXISTS ballots_cast (
    voter_id    BIGINT NOT NULL,
    election_id BIGINT NOT NULL,
    PRIMARY KEY (voter_id, election_id)
);
