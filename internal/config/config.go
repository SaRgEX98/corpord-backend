package config

import (
	"corpord-api/internal/geo"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App         App
	Database    Database
	Logger      Logger
	HTTP        HTTP
	JWT         JWT
	SSO         SSO
	FrontendURL string
	Geo         Geo
}

type Geo struct {
	PhotonBaseURL string
	Timeout       time.Duration
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
	u := url.URL{Scheme: "postgres", User: url.UserPassword(p.User, p.Password), Host: net.JoinHostPort(p.Host, p.Port), Path: "/" + p.DBName}
	q := url.Values{"sslmode": {p.SSLMode}}
	u.RawQuery = q.Encode()
	return u.String()
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
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
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
	if err := validateEnvironment(); err != nil {
		return nil, err
	}

	cfg := &Config{
		Geo:         Geo{PhotonBaseURL: getEnv("PHOTON_BASE_URL", ""), Timeout: getDuration("PHOTON_TIMEOUT", 3*time.Second)},
		FrontendURL: strings.TrimRight(getEnv("FRONTEND_URL", "http://localhost:3000"), "/"),
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
			Port:              getInt("HTTP_PORT", 8080),
			ReadHeaderTimeout: getDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:       getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
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

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate fails before serving requests when credentials or runtime limits are invalid.
func (c *Config) Validate() error {
	if c.Geo.PhotonBaseURL != "" {
		if _, err := geo.NewClient(c.Geo.PhotonBaseURL, c.Geo.Timeout); err != nil {
			return fmt.Errorf("invalid PHOTON_BASE_URL or PHOTON_TIMEOUT")
		}
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	switch c.JWT.SigningAlgorithm {
	case "HS256", "HS384", "HS512":
	default:
		return fmt.Errorf("JWT_SIGNING_ALGORITHM must be HS256, HS384 or HS512")
	}
	if c.JWT.AccessTokenTTL <= 0 || c.JWT.RefreshTokenTTL <= c.JWT.AccessTokenTTL {
		return fmt.Errorf("JWT TTLs must be positive and refresh TTL must exceed access TTL")
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 || c.HTTP.ReadHeaderTimeout <= 0 || c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 || c.HTTP.IdleTimeout <= 0 {
		return fmt.Errorf("HTTP port and timeouts must be positive and valid")
	}
	pg := c.Database.Postgres
	if pg.Host == "" || pg.User == "" || pg.DBName == "" || pg.MaxConns < 1 || int64(pg.MaxConns) > 2147483647 {
		return fmt.Errorf("PostgreSQL host, user, database and max connections must be valid")
	}
	for key, value := range map[string]string{"DB_POSTGRES_PORT": pg.Port, "DB_REDIS_PORT": c.Database.Redis.Port} {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%s must be a valid port", key)
		}
	}
	if c.Database.Redis.Host == "" || c.Database.Redis.DB < 0 {
		return fmt.Errorf("Redis host and database must be valid")
	}
	if !validHTTPURL(c.FrontendURL) {
		return fmt.Errorf("FRONTEND_URL must be an absolute HTTP(S) URL without credentials, query or fragment")
	}
	for name, provider := range map[string]OAuthProvider{"GOOGLE": c.SSO.Google, "YANDEX": c.SSO.Yandex} {
		if provider.Enabled && c.HTTP.WriteTimeout <= 10*time.Second {
			return fmt.Errorf("HTTP_WRITE_TIMEOUT must exceed the 10s SSO login timeout when a provider is enabled")
		}
		if provider.Enabled && (strings.TrimSpace(provider.ClientID) == "" || strings.TrimSpace(provider.ClientSecret) == "" || !validHTTPURL(provider.RedirectURL)) {
			return fmt.Errorf("SSO_%s_ENABLED requires client ID, secret and an absolute HTTP(S) redirect URL", name)
		}
	}
	return nil
}

func validHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

func validateEnvironment() error {
	for _, key := range []string{"APP_DEBUG", "LOG_ENABLE_CALLER", "LOG_ENABLE_STACK", "LOG_COMPRESS", "LOG_TO_FILE", "SSO_GOOGLE_ENABLED", "SSO_YANDEX_ENABLED", "COOKIE_SECURE"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			if _, err := strconv.ParseBool(value); err != nil {
				return fmt.Errorf("%s must be a boolean", key)
			}
		}
	}
	for _, key := range []string{"DB_POSTGRES_MAX_CONNS", "DB_REDIS_DB", "HTTP_PORT", "LOG_MAX_SIZE", "LOG_MAX_BACKUPS", "LOG_MAX_AGE"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			if _, err := strconv.Atoi(value); err != nil {
				return fmt.Errorf("%s must be an integer", key)
			}
		}
	}
	for _, key := range []string{"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "JWT_ACCESS_TTL", "JWT_REFRESH_TTL", "PHOTON_TIMEOUT"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			if _, err := time.ParseDuration(value); err != nil {
				return fmt.Errorf("%s must be a duration", key)
			}
		}
	}
	domain := os.Getenv("COOKIE_DOMAIN")
	if strings.ContainsAny(domain, "/:; \t\r\n") {
		return fmt.Errorf("COOKIE_DOMAIN must be a hostname or empty")
	}
	secure := os.Getenv("APP_ENV") == "production"
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		secure, _ = strconv.ParseBool(value)
	}
	sameSite := strings.ToLower(getEnv("COOKIE_SAME_SITE", "lax"))
	if sameSite != "lax" && sameSite != "strict" && sameSite != "none" {
		return fmt.Errorf("COOKIE_SAME_SITE must be lax, strict or none")
	}
	if sameSite == "none" && !secure {
		return fmt.Errorf("COOKIE_SAME_SITE=none requires COOKIE_SECURE=true")
	}
	for _, origin := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if !validHTTPURL(origin) {
			return fmt.Errorf("ALLOWED_ORIGINS must contain absolute HTTP(S) origins")
		}
		u, _ := url.Parse(origin)
		if u.Path != "" && u.Path != "/" {
			return fmt.Errorf("ALLOWED_ORIGINS cannot contain paths")
		}
	}
	return nil
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
