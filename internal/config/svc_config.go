package config

import (
	"fmt"
	"log/slog"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"

	"github.com/knadh/koanf/v2"
)

var Global = koanf.New(".")

// LoadRequiredEnv loads the environment variables required to run the services.
// An error is returned if any of the required variables are missing in .env or
// env.
func LoadRequiredEnv() error {
	// Load default values
	Global.Load(confmap.Provider(map[string]interface{}{
		API_PORT:      "8080",
		API_BIND_ADDR: "127.0.0.1",
	}, "."), nil)

	// .env file is optional, but we still try to load it if it exists.
	err := Global.Load(
		file.Provider(".env"), dotenv.Parser(),
	)
	if err != nil {
		slog.Warn("failed to load .env file", slog.Any("error", err))
	}

	if err := Global.Load(env.Provider("", "", nil), nil); err != nil {
		slog.Warn("failed to load environment variables", slog.Any("error", err))
	}

	required := []string{
		RPC_URL_ETHEREUM,
		RPC_URL_SOLANA,
		RPC_URL_BITCOIN,
		API_BIND_ADDR,
		API_PORT,
	}

	for _, r := range required {
		if !Global.Exists(r) {
			return fmt.Errorf("required environment variable %s is missing", r)
		}
	}

	return nil
}
