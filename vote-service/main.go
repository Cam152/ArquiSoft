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
)

func main() {
	log.SetOutput(os.Stdout)
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := newMongoStore(ctx, cfg)
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}

	app := &App{cfg: cfg, store: store, clients: newClients(cfg)}
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.routes(),
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
