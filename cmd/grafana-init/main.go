package main

import (
	"context"
	"errors"
	"highload_prog/internal/grafana"
	"log"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	URL           string `env:"GRAFANA_URL,required,notEmpty"`
	AdminUser     string `env:"GRAFANA_ADMIN_USER" envDefault:"admin"`
	AdminPassword string `env:"GRAFANA_ADMIN_PASSWORD,required,notEmpty"`
	TokenFile     string `env:"GRAFANA_TOKEN_FILE,required,notEmpty"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}
	savedToken, err := os.ReadFile(cfg.TokenFile)
	if err == nil {
		if len(savedToken) == 0 {
			log.Fatalf("token file %s is empty", cfg.TokenFile)
		}
		log.Printf("Grafana token file already exists")
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}
	ctx := context.Background()
	client := grafana.New(cfg.URL, cfg.AdminUser, cfg.AdminPassword)
	account, err := client.FindServiceAccount(ctx, "dashboard-generator")
	if err != nil {
		log.Fatal(err)
	}
	if account == nil {
		account, err = client.CreateServiceAccount(ctx, "dashboard-generator")
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(cfg.TokenFile), 0700); err != nil {
		log.Fatal(err)
	}

	token, err := client.CreateToken(ctx, account.ID, "dashboard-generator")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(cfg.TokenFile, []byte(token), 0600); err != nil {
		log.Fatal(err)
	}
	log.Printf("Grafana token created")

	log.Printf("service account ready: id=%d", account.ID)

}
