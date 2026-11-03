// agr-vote-lo guarda el voto anónimo en agr-votes-db y publica VoteRegistered en
// agr-broker-as con el patrón outbox.
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

	"vote-service/internal/config"
	"vote-service/internal/handler"
	"vote-service/internal/outbox"
	"vote-service/internal/publisher"
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

	broker := publisher.NewAMQP(cfg.AMQPURL, cfg.Exchange)
	relay := outbox.NewRelay(store, broker, cfg.OutboxInterval, cfg.OutboxBatch)
	votes := service.NewVotes(store, relay)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.New(votes, store, cfg.ServiceAPIKey).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	relayCtx, stopRelay := context.WithCancel(context.Background())
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		// Al arrancar se publican los votos que quedaron pendientes de antes.
		relay.Flush(relayCtx)
		relay.Run(relayCtx)
	}()

	go func() {
		log.Printf("agr-vote-lo escuchando en :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("el servidor falló: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("apagando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("apagado del servidor con error: %v", err)
	}
	stopRelay()
	<-relayDone
	relay.Wait() // publicaciones que salieron de peticiones ya respondidas
	broker.Close()
	if err := store.Close(shutdownCtx); err != nil {
		log.Printf("no se pudo cerrar MongoDB: %v", err)
	}
}
