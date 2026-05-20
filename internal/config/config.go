package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      App
	Database Database
	Logger   Logger
	HTTP     HTTP
	JWT      JWT
	SSO      SSO
}

type App struct {
	Env      string
	Name     string
	Debug    bool
	TimeZone string
}

type Database struct {
	Postgres Postgres
	Redis    Redis
}

type Postgres struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int
}

func (p *Postgres) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode,
	)
}

type Redis struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type Logger struct {
	Level        string
	Encoding     string
	OutputPaths  string
	ErrorOutput  string
	EnableCaller bool
	EnableStack  bool
	MaxSize      int
	MaxBackups   int
	MaxAge       int
	Compress     bool
	LogToFile    bool
	LogDirectory string
}

type HTTP struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type JWT struct {
	Secret           string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	SigningAlgorithm string
}

type SSO struct {
	Google OAuthProvider
	Yandex OAuthProvider
}

type OAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Enabled      bool
}

func Load() (*Config, error) {
	// dev only
	_ = godotenv.Load()

	cfg := &Config{
		App: App{
			Env:      getEnv("APP_ENV", "development"),
			Name:     getEnv("APP_NAME", "corpord-api"),
			Debug:    getBool("APP_DEBUG", true),
			TimeZone: getEnv("APP_TIMEZONE", "UTC"),
		},

		Database: Database{
			Postgres: Postgres{
				Host:     getEnv("DB_POSTGRES_HOST", "localhost"),
				Port:     getEnv("DB_POSTGRES_PORT", "5432"),
				User:     getEnv("DB_POSTGRES_USER", "postgres"),
				Password: getEnv("DB_POSTGRES_PASSWORD", ""),
				DBName:   getEnv("DB_POSTGRES_NAME", "corpord"),
				SSLMode:  getEnv("DB_POSTGRES_SSLMODE", "disable"),
				MaxConns: getInt("DB_POSTGRES_MAX_CONNS", 10),
			},

			Redis: Redis{
				Host:     getEnv("DB_REDIS_HOST", "localhost"),
				Port:     getEnv("DB_REDIS_PORT", "6379"),
				Password: getEnv("DB_REDIS_PASSWORD", ""),
				DB:       getInt("DB_REDIS_DB", 0),
			},
		},

		Logger: Logger{
			Level:        getEnv("LOG_LEVEL", "debug"),
			Encoding:     getEnv("LOG_ENCODING", "console"),
			OutputPaths:  getEnv("LOG_OUTPUT_PATHS", "stdout"),
			ErrorOutput:  getEnv("LOG_ERROR_OUTPUT", "stderr"),
			EnableCaller: getBool("LOG_ENABLE_CALLER", true),
			EnableStack:  getBool("LOG_ENABLE_STACK", true),
			MaxSize:      getInt("LOG_MAX_SIZE", 100),
			MaxBackups:   getInt("LOG_MAX_BACKUPS", 5),
			MaxAge:       getInt("LOG_MAX_AGE", 30),
			Compress:     getBool("LOG_COMPRESS", true),
			LogToFile:    getBool("LOG_TO_FILE", false),
			LogDirectory: getEnv("LOG_DIRECTORY", "./logs"),
		},

		HTTP: HTTP{
			Port:         getInt("HTTP_PORT", 8080),
			ReadTimeout:  getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},

		JWT: JWT{
			Secret:           getEnv("JWT_SECRET", ""),
			AccessTokenTTL:   getDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL:  getDuration("JWT_REFRESH_TTL", 720*time.Hour),
			SigningAlgorithm: getEnv("JWT_SIGNING_ALGORITHM", "HS256"),
		},

		SSO: SSO{
			Google: OAuthProvider{
				ClientID:     getEnv("SSO_GOOGLE_CLIENT_ID", ""),
				ClientSecret: getEnv("SSO_GOOGLE_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("SSO_GOOGLE_REDIRECT_URL", ""),
				Enabled:      getBool("SSO_GOOGLE_ENABLED", false),
			},
			Yandex: OAuthProvider{
				ClientID:     getEnv("SSO_YANDEX_CLIENT_ID", ""),
				ClientSecret: getEnv("SSO_YANDEX_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("SSO_YANDEX_REDIRECT_URL", ""),
				Enabled:      getBool("SSO_YANDEX_ENABLED", false),
			},
		},
	}

	return cfg, nil
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
