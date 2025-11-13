package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithGroup(name string) Logger
}

type LoggingConfig struct {
	Format    string // "json" или "text"
	ToFile    bool
	LogDir    string
	Debug     bool
	MaxSizeMB int
	MaxFiles  int
	Level     slog.Level
}

type slogLogger struct {
	debug bool
	slog  *slog.Logger
}

func (l *slogLogger) Debug(msg string, args ...any) {
	if !l.debug {
		return
	}
	l.slog.Debug(msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{slog: l.slog.With(args...)}
}

func (l *slogLogger) WithGroup(name string) Logger {
	return &slogLogger{slog: l.slog.WithGroup(name)}
}

func New(cfg LoggingConfig) (Logger, error) {
	var writers []io.Writer

	consoleWriter := os.Stdout
	writers = append(writers, consoleWriter)

	if cfg.ToFile {
		if cfg.LogDir == "" {
			cfg.LogDir = "./logs"
		}
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return nil, err
		}

		logFile := filepath.Join(cfg.LogDir, time.Now().Format("2006-01-02")+".log")

		rotating := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxFiles,
			MaxAge:     30,
			Compress:   true,
		}
		writers = append(writers, rotating)
	}

	writer := io.MultiWriter(writers...)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if cfg.Debug {
		opts.Level = slog.LevelDebug
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	s := slog.New(handler)
	slog.SetDefault(s)

	return &slogLogger{slog: s, debug: cfg.Debug}, nil
}

func NewDefault() Logger {
	s := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	return &slogLogger{slog: s, debug: false}
}
