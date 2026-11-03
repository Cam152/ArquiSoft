package com.arquisoft.voterservice;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.ResultActions;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@SpringBootTest
@AutoConfigureMockMvc
class VoterServiceApplicationTests {

    private static final String KEY = "test-service-key";

    @Autowired
    private MockMvc mvc;

    @Autowired
    private JdbcTemplate jdbc;

    @BeforeEach
    void clean() {
        jdbc.update("DELETE FROM ballots_cast");
    }

    private ResultActions call(String path, String body, String key) throws Exception {
        var req = post(path).contentType(MediaType.APPLICATION_JSON).content(body);
        if (key != null) {
            req.header("X-Service-Key", key);
        }
        return mvc.perform(req);
    }

    private ResultActions mark(long voter, long election) throws Exception {
        return call("/voters/mark-voted", body(voter, election), KEY);
    }

    private ResultActions unmark(long voter, long election) throws Exception {
        return call("/voters/unmark-voted", body(voter, election), KEY);
    }

    private static String body(long voter, long election) {
        return "{\"voter_id\": " + voter + ", \"election_id\": " + election + "}";
    }

    @Test
    void health() throws Exception {
        mvc.perform(get("/health"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("ok"));
    }

    @Test
    void marcaUnaVezPorEleccion() throws Exception {
        mark(1, 1).andExpect(status().isOk()).andExpect(jsonPath("$.status").value("marked"));
        mark(1, 1).andExpect(status().isConflict()).andExpect(jsonPath("$.detail").exists());
        // Mismo votante, otra elección: puede votar.
        mark(1, 2).andExpect(status().isOk());
        // Otro votante, misma elección: puede votar.
        mark(2, 1).andExpect(status().isOk());
    }

    @Test
    void desmarcarPermiteVolverAMarcar() throws Exception {
        mark(1, 1).andExpect(status().isOk());
        unmark(1, 1).andExpect(status().isOk()).andExpect(jsonPath("$.status").value("unmarked"));
        mark(1, 1).andExpect(status().isOk());
    }

    @Test
    void desmarcarEsIdempotente() throws Exception {
        unmark(7, 7).andExpect(status().isOk()).andExpect(jsonPath("$.status").value("unmarked"));
        unmark(7, 7).andExpect(status().isOk());
    }

    @Test
    void desmarcarSoloBorraEsaEleccion() throws Exception {
        mark(1, 1).andExpect(status().isOk());
        mark(1, 2).andExpect(status().isOk());
        unmark(1, 1).andExpect(status().isOk());
        mark(1, 2).andExpect(status().isConflict());
    }

    @Test
    void sinLlaveOLlaveIncorrectaDa401() throws Exception {
        call("/voters/mark-voted", body(1, 1), null)
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.detail").value("Llave de servicio incorrecta"));
        call("/voters/mark-voted", body(1, 1), "otra").andExpect(status().isUnauthorized());
        call("/voters/unmark-voted", body(1, 1), "otra").andExpect(status().isUnauthorized());
        // La llave se revisa antes que el cuerpo.
        call("/voters/mark-voted", "no es json", null).andExpect(status().isUnauthorized());
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM ballots_cast", Integer.class)).isZero();
    }

    @Test
    void cuerpoInvalidoDa422() throws Exception {
        call("/voters/mark-voted", "{\"voter_id\": 1}", KEY)
                .andExpect(status().isUnprocessableEntity())
                .andExpect(jsonPath("$.detail").exists());
        call("/voters/mark-voted", "{\"voter_id\": 0, \"election_id\": 1}", KEY)
                .andExpect(status().isUnprocessableEntity());
        call("/voters/mark-voted", "{\"voter_id\": \"abc\", \"election_id\": 1}", KEY)
                .andExpect(status().isUnprocessableEntity());
        call("/voters/mark-voted", "no es json", KEY).andExpect(status().isUnprocessableEntity());
    }

    @Test
    void rutaDesconocidaDa404ConDetail() throws Exception {
        mvc.perform(get("/no-existe"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.detail").exists());
    }

    @Test
    void peticionesSimultaneasSoloMarcanUnaVez() throws Exception {
        int threads = 8;
        ExecutorService pool = Executors.newFixedThreadPool(threads);
        try {
            List<Callable<Integer>> tasks = new ArrayList<>();
            for (int i = 0; i < threads; i++) {
                tasks.add(() -> mark(3, 3).andReturn().getResponse().getStatus());
            }
            int ok = 0;
            int conflict = 0;
            for (Future<Integer> f : pool.invokeAll(tasks)) {
                int s = f.get();
                if (s == 200) ok++;
                if (s == 409) conflict++;
            }
            assertThat(ok).isEqualTo(1);
            assertThat(conflict).isEqualTo(threads - 1);
        } finally {
            pool.shutdown();
        }
    }
}
