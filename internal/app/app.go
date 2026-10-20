package app

import (
	"context"
	"encoding/json"
	"fmt"
	"highload_prog/internal/config"
	"highload_prog/internal/controller"
	"highload_prog/internal/monitoring"
	"highload_prog/internal/repository"
	"highload_prog/internal/usecases"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(cfg *config.Config) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("could not connect to postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("could not ping postgres: %w", err)
	}

	repo := repository.New(pool)
	service := usecases.New(repo)
	ctrl := controller.New(service)
	mux := http.NewServeMux()
	monitored := monitoring.New(mux, cfg.Monitoring.Pod, cfg.Monitoring.Datacenter, cfg.Monitoring.Deployment)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	monitored.HandleFunc("GET /products/{id}", ctrl.Get)
	monitored.HandleFunc("POST /products", ctrl.Post)
	monitored.HandleFunc("DELETE /products/{id}", ctrl.Delete)
	monitored.HandleFunc("PUT /products/{id}", ctrl.Update)

	mux.HandleFunc("GET /internal/routes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(monitored.Routes()); err != nil {
			log.Printf("write routes response: %v", err)
		}
	})

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return server.ListenAndServe()

}
