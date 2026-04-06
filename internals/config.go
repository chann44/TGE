package internal

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	ClickHouseHost      string
	ClickHousePort      string
	ClickHouseUser      string
	ClickHousePassword  string
	ClickHouseDatabase  string
	RedisHost           string
	RedisPort           string
	FrontendURL         string
	GithubClientID      string
	GithubClientSecret  string
	GithubRedirectURI   string
	GithubAppID         string
	GithubAppPrivateKey string
	GithubAppSlug       string
	GithubAppInstallURL string
	GHSAAPIToken        string
	NVDAPIKey           string
}

var cfg *Config

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}
}

func GetConfig() *Config {
	if cfg == nil {
		cfg = &Config{
			DBHost:              os.Getenv("DB_HOST"),
			DBPort:              os.Getenv("DB_PORT"),
			DBUser:              os.Getenv("DB_USER"),
			DBPassword:          os.Getenv("DB_PASSWORD"),
			DBName:              os.Getenv("DB_NAME"),
			ClickHouseHost:      os.Getenv("CLICKHOUSE_HOST"),
			ClickHousePort:      os.Getenv("CLICKHOUSE_PORT"),
			ClickHouseUser:      os.Getenv("CLICKHOUSE_USER"),
			ClickHousePassword:  os.Getenv("CLICKHOUSE_PASSWORD"),
			ClickHouseDatabase:  os.Getenv("CLICKHOUSE_DATABASE"),
			RedisHost:           os.Getenv("REDIS_HOST"),
			RedisPort:           os.Getenv("REDIS_PORT"),
			FrontendURL:         os.Getenv("FRONTEND_URL"),
			GithubClientID:      os.Getenv("GITHUB_CLIENT_ID"),
			GithubClientSecret:  os.Getenv("GITHUB_CLIENT_SECRET"),
			GithubRedirectURI:   os.Getenv("GITHUB_REDIRECT_URI"),
			GithubAppID:         os.Getenv("GITHUB_APP_ID"),
			GithubAppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY"),
			GithubAppSlug:       os.Getenv("GITHUB_APP_SLUG"),
			GithubAppInstallURL: os.Getenv("GITHUB_APP_INSTALL_URL"),
			GHSAAPIToken:        os.Getenv("GHSA_API_TOKEN"),
			NVDAPIKey:           os.Getenv("NVD_API_KEY"),
		}

		if cfg.FrontendURL == "" {
			cfg.FrontendURL = "http://localhost:5173"
		}
		if cfg.ClickHouseHost == "" {
			cfg.ClickHouseHost = "localhost"
		}
		if cfg.ClickHousePort == "" {
			cfg.ClickHousePort = "9000"
		}
		if cfg.ClickHouseUser == "" {
			cfg.ClickHouseUser = "default"
		}
		if cfg.ClickHousePassword == "" {
			cfg.ClickHousePassword = "clickhouse"
		}
		if cfg.ClickHouseDatabase == "" {
			cfg.ClickHouseDatabase = "default"
		}
	}
	return cfg
}
