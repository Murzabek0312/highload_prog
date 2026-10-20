package main

import (
	"context"
	"highload_prog/internal/dashboards"
	"highload_prog/internal/discovery"
	"highload_prog/internal/grafana"
	"log"
	"os"
	"strings"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	AppURL     string `env:"APP_URL,required,notEmpty"`
	GrafanaURL string `env:"GRAFANA_URL,required,notEmpty"`
	TokenFile  string `env:"GRAFANA_TOKEN_FILE,required,notEmpty"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}

	tokenData, err := os.ReadFile(cfg.TokenFile)
	if err != nil {
		log.Fatal(err)
	}
	token := strings.TrimSpace(string(tokenData))
	if token == "" {
		log.Fatal("token file is empty")
	}
	ctx := context.Background()
	client := discovery.New(cfg.AppURL)
	routes, err := client.Routes(ctx)
	if err != nil {
		log.Fatal(err)
	}

	board, err := dashboards.Build(routes)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Built dashboard with %d panels", len(board.Elements))
	log.Printf("Discovered %d routes", len(routes))

	grafanaClient := grafana.NewWithToken(cfg.GrafanaURL, token)
	if err := grafanaClient.SaveDashboard(ctx, "highload-http", board); err != nil {
		log.Fatal(err)
	}
	log.Print("Dashboard saved: highload-http")
}
