package com.arquisoft.authservice.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;

@Entity
@Table(name = "voters")
public class VoterAuth {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(unique = true, nullable = false)
    private String document;

    @Column(nullable = false)
    private String name;

    @Column(nullable = false)
    private String passwordHash;

    public VoterAuth() {}

    public VoterAuth(String document, String name, String passwordHash) {
        this.document = document;
        this.name = name;
        this.passwordHash = passwordHash;
    }

    public Long getId() { 
        return id; 
    }

    public String getDocument() { 
        return document; 
    }

    public void setDocument(String document) { 
        this.document = document; 
    }

    public String getName() { 
        return name; 
    }

    public void setName(String name) { 
        this.name = name; 
    }

    public String getPasswordHash() { 
        return passwordHash; 
    }

    public void setPasswordHash(String passwordHash) { 
        this.passwordHash = passwordHash; 
    }
}