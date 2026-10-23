package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vote-service/internal/client"
	"vote-service/internal/config"
	"vote-service/internal/handler"
	"vote-service/internal/repository"
	"vote-service/internal/service"
)

func main() {
	log.SetOutput(os.Stdout)
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := repository.NewMongoStore(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}

	voting := service.NewVoting(
		store,
		client.NewElectionClient(cfg.ElectionServiceURL),
		client.NewVoterClient(cfg.VoterServiceURL, cfg.ServiceAPIKey),
	)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.New(voting, cfg.CORSOrigins).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("vote-service escuchando en :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("el servidor falló: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("apagado con error: %v", err)
	}
	if err := store.Close(shutdownCtx); err != nil {
		log.Printf("no se pudo cerrar MongoDB: %v", err)
	}
}
