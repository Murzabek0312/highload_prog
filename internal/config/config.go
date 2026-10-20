package config

import "github.com/caarlos0/env/v10"

type Config struct {
	HTTP struct {
		Port string `env:"PORT" envDefault:"8080"`
	}
	Postgres struct {
		URL string `env:"DATABASE_URL,required,notEmpty" default:"postgres://app:password@localhost:5432/highload?sslmode=disable"`
	}

	Monitoring struct {
		Pod        string `env:"POD_ID,required,notEmpty"`
		Datacenter string `env:"DATACENTER,required,notEmpty"`
		Deployment string `env:"DEPLOYMENT,required,notEmpty"`
	}
}

func New() (*Config, error) {
	var cfg Config
	err := env.Parse(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
