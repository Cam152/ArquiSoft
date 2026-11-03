package com.arquisoft.authservice.config;

import com.arquisoft.authservice.model.VoterAuth;
import com.arquisoft.authservice.repository.VoterAuthRepository;
import org.springframework.boot.CommandLineRunner;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Component;

@Component
public class DataSeeder implements CommandLineRunner {

    private final VoterAuthRepository voterRepository;
    private final PasswordEncoder passwordEncoder;

    public DataSeeder(VoterAuthRepository voterRepository, PasswordEncoder passwordEncoder) {
        this.voterRepository = voterRepository;
        this.passwordEncoder = passwordEncoder;
    }

    @Override
    public void run(String... args) {
        if (voterRepository.count() == 0) {
            String passwordHash = passwordEncoder.encode("123456");

            voterRepository.save(new VoterAuth("1001", "Ana Garcia", passwordHash));
            voterRepository.save(new VoterAuth("1002", "Carlos Rodriguez", passwordHash));
            voterRepository.save(new VoterAuth("1003", "Beatriz Lopez", passwordHash));
            voterRepository.save(new VoterAuth("1004", "David Martinez", passwordHash));
            voterRepository.save(new VoterAuth("1005", "Elena Gomez", passwordHash));

            System.out.println(">>> [agr-auth-lo] Seeder completado: 5 votantes creados.");
        }
    }
}