package com.arquisoft.voterservice.config;

import com.arquisoft.voterservice.model.Voter;
import com.arquisoft.voterservice.repository.VoterRepository;

import org.springframework.boot.CommandLineRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

@Configuration
public class DataSeeder {

    @Bean
    CommandLineRunner seedVoters(
            VoterRepository repository) {

        return args -> {

            if (repository.count() > 0) {
                return;
            }

            BCryptPasswordEncoder encoder = new BCryptPasswordEncoder();

            String passwordHash = encoder.encode("voter123");

            repository.save(
                    new Voter(
                            "1000000001",
                            "Votante 1",
                            passwordHash));

            repository.save(
                    new Voter(
                            "1000000002",
                            "Votante 2",
                            passwordHash));

            repository.save(
                    new Voter(
                            "1000000003",
                            "Votante 3",
                            passwordHash));

            repository.save(
                    new Voter(
                            "1000000004",
                            "Votante 4",
                            passwordHash));

            repository.save(
                    new Voter(
                            "1000000005",
                            "Votante 5",
                            passwordHash));

            System.out.println(
                    "5 votantes de prueba creados.");
        };
    }
}