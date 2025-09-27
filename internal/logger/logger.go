package logger

import (
	"corpord-api/internal/config"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// ErrInvalidConfig is returned when the logger configuration is invalid
	ErrInvalidConfig = errors.New("invalid logger configuration")
	// ErrNoOutputs is returned when no output destinations are configured
	ErrNoOutputs = errors.New("no log output destinations configured")
)

// Logger wraps zap.SugaredLogger with additional functionality
type Logger struct {
	*zap.SugaredLogger
	closers []func() error
}

// New creates a new logger instance with the provided configuration
func New(cfg *config.Logger) (*Logger, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	encoder := newEncoder(cfg.Encoding)
	cores, closers, err := createCores(cfg, level, encoder)
	if err != nil {
		return nil, err
	}

	logger, err := buildLogger(cores, cfg)
	if err != nil {
		closeAll(closers)
		return nil, err
	}

	return &Logger{
		SugaredLogger: logger,
		closers:       closers,
	}, nil
}

// Sync flushes any buffered log entries and closes all underlying resources
func (l *Logger) Sync() error {
	err := l.SugaredLogger.Sync()
	for _, closer := range l.closers {
		if syncErr := closer(); syncErr != nil && err == nil {
			err = syncErr
		}
	}
	return err
}

func validateConfig(cfg *config.Logger) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	return nil
}

// setDefaults is not needed as defaults are set in config package

func parseLevel(level string) (zapcore.Level, error) {
	var lvl zapcore.Level
	switch level {
	case "debug":
		lvl = zapcore.DebugLevel
	case "info":
		lvl = zapcore.InfoLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	case "dpanic":
		lvl = zapcore.DPanicLevel
	case "panic":
		lvl = zapcore.PanicLevel
	case "fatal":
		lvl = zapcore.FatalLevel
	default:
		return lvl, fmt.Errorf("unknown log level: %s", level)
	}

	return lvl, nil
}

func newEncoder(encoding string) zapcore.Encoder {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if encoding == "json" {
		return zapcore.NewJSONEncoder(encoderCfg)
	}
	return zapcore.NewConsoleEncoder(encoderCfg)
}

func createCores(cfg *config.Logger, level zapcore.Level, encoder zapcore.Encoder) ([]zapcore.Core, []func() error, error) {
	var (
		cores   []zapcore.Core
		closers []func() error
	)

	// Add console core if configured
	if cfg.OutputPaths == "stdout" {
		cores = append(cores, newConsoleCore(encoder, level))
	}

	// Add file core if enabled
	if cfg.LogToFile {
		fileCore, closer, err := newFileCore(cfg, encoder, level)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create file core: %w", err)
		}
		cores = append(cores, fileCore)
		closers = append(closers, closer)
	}

	if len(cores) == 0 {
		return nil, nil, ErrNoOutputs
	}

	return cores, closers, nil
}

func newConsoleCore(encoder zapcore.Encoder, level zapcore.Level) zapcore.Core {
	return zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stdout),
		zap.NewAtomicLevelAt(level),
	)
}

func newFileCore(cfg *config.Logger, encoder zapcore.Encoder, level zapcore.Level) (zapcore.Core, func() error, error) {
	if err := os.MkdirAll(cfg.LogDirectory, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(cfg.LogDirectory, time.Now().Format("2006-01-02")+".log")
	lumberJackLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(lumberJackLogger),
		zap.NewAtomicLevelAt(level),
	)

	return core, lumberJackLogger.Close, nil
}

func buildLogger(cores []zapcore.Core, cfg *config.Logger) (*zap.SugaredLogger, error) {
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	if !cfg.EnableCaller {
		opts = append(opts, zap.WithCaller(false))
	}

	if !cfg.EnableStack {
		opts = append(opts, zap.AddStacktrace(zapcore.FatalLevel+1))
	}

	core := zapcore.NewTee(cores...)
	zapLogger := zap.New(core, opts...).With(
		zap.String("app", "corpord-api"),
	)

	// Redirect stdlib log to zap
	zap.RedirectStdLog(zapLogger)

	return zapLogger.Sugar(), nil
}

func closeAll(closers []func() error) {
	for _, closer := range closers {
		_ = closer()
	}
}
