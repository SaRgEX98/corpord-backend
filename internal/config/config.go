package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	App      App      `mapstructure:"app"`
	Database Database `mapstructure:"database"`
	Logger   Logger   `mapstructure:"logger"`
	HTTP     HTTP     `mapstructure:"http"`
	JWT      JWT      `mapstructure:"jwt"`
}

type App struct {
	Env      string `mapstructure:"env"`
	Name     string `mapstructure:"name"`
	Debug    bool   `mapstructure:"debug"`
	TimeZone string `mapstructure:"timezone"`
}

type Database struct {
	Postgres Postgres `mapstructure:"postgres"`
	Redis    Redis    `mapstructure:"redis"`
}

type Postgres struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	MaxConns int    `mapstructure:"max_conns"`
}

func (p *Postgres) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode,
	)
}

type Redis struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type Logger struct {
	Level        string `mapstructure:"level"`         // Уровень логирования (debug, info, warn, error, dpanic, panic, fatal)
	Encoding     string `mapstructure:"encoding"`      // Формат вывода (console, json)
	OutputPaths  string `mapstructure:"output_paths"`  // Пути для вывода логов (через запятую)
	ErrorOutput  string `mapstructure:"error_output"`  // Путь для вывода ошибок
	EnableCaller bool   `mapstructure:"enable_caller"` // Включить информацию о вызывающем коде
	EnableStack  bool   `mapstructure:"enable_stack"`  // Включить стектрейс для ошибок
	MaxSize      int    `mapstructure:"max_size"`      // Максимальный размер лог-файла в МБ
	MaxBackups   int    `mapstructure:"max_backups"`   // Максимальное количество старых лог-файлов
	MaxAge       int    `mapstructure:"max_age"`       // Максимальное время хранения логов в днях
	Compress     bool   `mapstructure:"compress"`      // Сжимать старые логи
	LogToFile    bool   `mapstructure:"log_to_file"`   // Писать логи в файл
	LogDirectory string `mapstructure:"log_directory"` // Директория для хранения логов
}

type HTTP struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type JWT struct {
	Secret           string        `mapstructure:"secret"`
	AccessTokenTTL   time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL  time.Duration `mapstructure:"refresh_token_ttl"`
	SigningAlgorithm string        `mapstructure:"signing_algorithm"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	// Load dotenv only in dev
	if _, err := os.Stat(".env"); err == nil {
		godotenv.Load()
	}

	// Read config file
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// ENV support
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	bindEnvs(v)

	setDefaults(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func bindEnvs(v *viper.Viper) {
	// JWT
	v.BindEnv("jwt.secret", "JWT_SECRET")

	// Postgres
	v.BindEnv("database.postgres.host", "DB_HOST")
	v.BindEnv("database.postgres.port", "DB_PORT")
	v.BindEnv("database.postgres.user", "DB_USER")
	v.BindEnv("database.postgres.password", "DB_PASSWORD")
	v.BindEnv("database.postgres.dbname", "DB_NAME")
	v.BindEnv("database.postgres.sslmode", "DB_SSLMODE")

	// Redis
	v.BindEnv("database.redis.host", "REDIS_HOST")
	v.BindEnv("database.redis.port", "REDIS_PORT")
	v.BindEnv("database.redis.password", "REDIS_PASSWORD")
	v.BindEnv("database.redis.db", "REDIS_DB")
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.env", "development")
	v.SetDefault("app.name", "corpord-api")
	v.SetDefault("app.debug", true)
	v.SetDefault("app.timezone", "UTC")

	v.SetDefault("database.postgres.port", "5432")
	v.SetDefault("database.postgres.sslmode", "disable")
	v.SetDefault("database.postgres.max_conns", 10)

	v.SetDefault("database.redis.port", "6379")
	v.SetDefault("database.redis.db", 0)

	v.SetDefault("logger.level", "debug")
	v.SetDefault("logger.encoding", "console")

	v.SetDefault("http.port", 8080)
	v.SetDefault("http.read_timeout", "10s")
	v.SetDefault("http.write_timeout", "10s")
	v.SetDefault("http.idle_timeout", "60s")

	v.SetDefault("jwt.access_token_ttl", "15m")
	v.SetDefault("jwt.refresh_token_ttl", "720h") // 30 дней
	v.SetDefault("jwt.signing_algorithm", "HS256")
}
