package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config содержит все конфигурации приложения
type Config struct {
	App        `mapstructure:"app"`
	Database   `mapstructure:"database"`
	Logger     `mapstructure:"logger"`
	HTTPConfig `mapstructure:"http"`
	JWTConfig  `mapstructure:"jwt"`
}

// Database объединяет настройки всех баз данных
type Database struct {
	Postgres `mapstructure:"postgres"`
	Redis    `mapstructure:"redis"`
}

// Logger содержит настройки логгера
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

// App содержит общие настройки приложения
type App struct {
	Env      string `mapstructure:"env"`
	Name     string `mapstructure:"name"`
	Debug    bool   `mapstructure:"debug"`
	TimeZone string `mapstructure:"timezone"`
}

// Postgres содержит настройки подключения к PostgreSQL
type Postgres struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	MaxConns int    `mapstructure:"max_conns"`
}

// Redis содержит настройки подключения к Redis
type Redis struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// HTTPConfig содержит настройки HTTP-сервера
type HTTPConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// JWTConfig содержит настройки JWT аутентификации
type JWTConfig struct {
	Secret           string        `mapstructure:"secret"`
	AccessTokenTTL   time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL  time.Duration `mapstructure:"refresh_token_ttl"`
	SigningAlgorithm string        `mapstructure:"signing_algorithm"`
}

// Load загружает конфигурацию из файла и переменных окружения
func Load(configPath string) (*Config, error) {
	// Загружаем .env файл, если он существует
	if _, err := os.Stat("./.env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	// Инициализируем Viper
	v := viper.New()

	// Устанавливаем конфигурацию для чтения из основного конфига
	if configPath != "" {
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")

		// Читаем конфиг из файла
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Включаем поддержку переменных окружения
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Устанавливаем значения по умолчанию
	setDefaults(v)

	// Загружаем конфигурацию в структуру
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults устанавливает значения по умолчанию
func setDefaults(v *viper.Viper) {
	// Устанавливаем значения по умолчанию для приложения
	v.SetDefault("app.env", "development")
	v.SetDefault("app.name", "corpord-api")
	v.SetDefault("app.debug", true)
	v.SetDefault("app.timezone", "UTC")

	// Устанавливаем значения по умолчанию для PostgreSQL
	v.SetDefault("database.postgres.host", "localhost")
	v.SetDefault("database.postgres.port", "5432")
	v.SetDefault("database.postgres.sslmode", "disable")
	v.SetDefault("database.postgres.max_conns", 10)

	// Устанавливаем значения по умолчанию для Redis
	v.SetDefault("database.redis.host", "localhost")
	v.SetDefault("database.redis.port", "6379")
	v.SetDefault("database.redis.db", 0)

	// Logger defaults
	v.SetDefault("logger", map[string]interface{}{
		"level":         "info",
		"encoding":      "console",
		"output_paths":  "stdout",
		"error_output":  "stderr",
		"enable_caller": true,
		"enable_stack":  true,
		"max_size":      100, // MB
		"max_backups":   14,  // files
		"max_age":       30,  // days
		"compress":      true,
		"log_to_file":   false,
		"log_directory": "logs",
	})

	// HTTP server defaults
	httpDefaults := map[string]interface{}{
		"port":          8080,
		"read_timeout":  "30s",
		"write_timeout": "30s",
		"idle_timeout":  "120s",
	}
	v.SetDefault("http", httpDefaults)

	// JWT defaults
	v.SetDefault("jwt", map[string]interface{}{
		"secret":            "your-secret-key",
		"access_token_ttl":  "15m",
		"refresh_token_ttl": "720h", // 30 days
		"signing_algorithm": "HS256",
	})
}

// DSN возвращает строку подключения к PostgreSQL
func (p *Postgres) DSN() string {
	// Получаем значения из переменных окружения, если они установлены
	host := getEnv("DB_HOST", p.Host)
	port := getEnv("DB_PORT", p.Port)
	user := getEnv("DB_USER", p.User)
	password := getEnv("DB_PASSWORD", p.Password)
	dbname := getEnv("DB_NAME", p.DBName)
	sslmode := getEnv("DB_SSLMODE", p.SSLMode)

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Addr возвращает адрес Redis в формате host:port
func (r *Redis) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

// IsProduction проверяет, запущено ли приложение в production среде
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
