package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port                string        `env:"PORT"                  envDefault:"8080"`
	FetchTimeout        time.Duration `env:"FETCH_TIMEOUT"         envDefault:"10s"`
	LinkCheckTimeout    time.Duration `env:"LINK_CHECK_TIMEOUT"    envDefault:"30s"`
	MaxConcurrentChecks int           `env:"MAX_CONCURRENT_CHECKS" envDefault:"10"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
