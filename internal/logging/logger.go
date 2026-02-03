// Package logging provides structured logging for spinup.
// It uses zerolog for structured JSON logging and supports log rotation.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps zerolog.Logger with convenience methods
type Logger struct {
	zerolog.Logger
	fileWriter io.WriteCloser
}

// Config holds logging configuration
type Config struct {
	// LogFile is the path to the log file
	LogFile string
	// Verbosity level: 0=ERROR+WARN, 1=INFO (-v), 2=DEBUG (-vv)
	Verbosity int
	// ConsoleOutput enables colored console output to stderr
	ConsoleOutput bool
}

// DefaultLogFile returns the default log file path (spinup.log in current directory)
func DefaultLogFile() string {
	exe, err := os.Executable()
	if err != nil {
		return "spinup.log"
	}
	return filepath.Join(filepath.Dir(exe), "spinup.log")
}

// levelFilterWriter wraps an io.Writer and filters based on log level
type levelFilterWriter struct {
	w     io.Writer
	level zerolog.Level
}

func (lfw levelFilterWriter) Write(p []byte) (n int, err error) {
	return lfw.w.Write(p)
}

func (lfw levelFilterWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if level >= lfw.level {
		return lfw.w.Write(p)
	}
	return len(p), nil
}

// New creates a new Logger with the given configuration.
// It sets up file logging with rotation and optional console output.
func New(cfg Config) (*Logger, error) {
	if cfg.LogFile == "" {
		cfg.LogFile = DefaultLogFile()
	}

	// Configure log file with rotation
	// Rotates when max size is reached, keeps 7 backups, max age 7 days
	fileLogger := &lumberjack.Logger{
		Filename:   cfg.LogFile,
		MaxSize:    100, // megabytes
		MaxBackups: 7,   // keep 7 backups
		MaxAge:     7,   // 7 days
		Compress:   false,
		LocalTime:  true,
	}

	// Set global time format for zerolog to match PRD format
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var writers []io.Writer

	// File writer always logs DEBUG and above (everything)
	writers = append(writers, levelFilterWriter{w: fileLogger, level: zerolog.DebugLevel})

	// Configure console output based on verbosity
	if cfg.ConsoleOutput {
		consoleLevel := zerolog.WarnLevel // Default: ERROR + WARN
		switch cfg.Verbosity {
		case 1:
			consoleLevel = zerolog.InfoLevel // -v: INFO and above
		case 2:
			consoleLevel = zerolog.DebugLevel // -vv: DEBUG and above
		default:
			if cfg.Verbosity >= 2 {
				consoleLevel = zerolog.DebugLevel
			}
		}

		// Console writer with custom format to match PRD Section 7.2:
		// 2026-02-02T08:32:15.123Z INFO  Starting deployment provider=vast model=qwen2.5-coder:32b
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02T15:04:05.000Z07:00",
			FormatLevel: func(i any) string {
				level := fmt.Sprintf("%s", i)
				switch level {
				case "debug":
					return "DEBUG"
				case "info":
					return "INFO "
				case "warn":
					return "WARN "
				case "error":
					return "ERROR"
				case "fatal":
					return "FATAL"
				default:
					return fmt.Sprintf("%-5s", level)
				}
			},
			FormatMessage: func(i any) string {
				return fmt.Sprintf("%s", i)
			},
			FormatFieldName: func(i any) string {
				return fmt.Sprintf("%s=", i)
			},
			FormatFieldValue: func(i any) string {
				return fmt.Sprintf("%s", i)
			},
		}

		// Add filtered console writer
		writers = append(writers, levelFilterWriter{w: consoleWriter, level: consoleLevel})
	}

	// Combine all writers using MultiLevelWriter
	multi := zerolog.MultiLevelWriter(writers...)
	logger := zerolog.New(multi).With().Timestamp().Logger()

	return &Logger{
		Logger:     logger,
		fileWriter: fileLogger,
	}, nil
}

// Close closes the log file writer
func (l *Logger) Close() error {
	if l.fileWriter != nil {
		return l.fileWriter.Close()
	}
	return nil
}

// DebugMsg logs a debug message
func (l *Logger) DebugMsg(msg string) {
	l.Logger.Debug().Msg(msg)
}

// InfoMsg logs an info message
func (l *Logger) InfoMsg(msg string) {
	l.Logger.Info().Msg(msg)
}

// WarnMsg logs a warning message
func (l *Logger) WarnMsg(msg string) {
	l.Logger.Warn().Msg(msg)
}

// ErrorMsg logs an error message
func (l *Logger) ErrorMsg(msg string) {
	l.Logger.Error().Msg(msg)
}

// FatalMsg logs a fatal message and exits
func (l *Logger) FatalMsg(msg string) {
	l.Logger.Fatal().Msg(msg)
}

// Debug returns a debug event for structured logging
func (l *Logger) Debug() *zerolog.Event {
	return l.Logger.Debug()
}

// Info returns an info event for structured logging
func (l *Logger) Info() *zerolog.Event {
	return l.Logger.Info()
}

// Warn returns a warn event for structured logging
func (l *Logger) Warn() *zerolog.Event {
	return l.Logger.Warn()
}

// Error returns an error event for structured logging
func (l *Logger) Error() *zerolog.Event {
	return l.Logger.Error()
}

// Fatal returns a fatal event for structured logging
func (l *Logger) Fatal() *zerolog.Event {
	return l.Logger.Fatal()
}

// Global logger instance
var globalLogger *Logger

// Init initializes the global logger with the given configuration
func Init(cfg Config) error {
	logger, err := New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	globalLogger = logger
	return nil
}

// Get returns the global logger
func Get() *Logger {
	if globalLogger == nil {
		// Create a default logger if not initialized
		globalLogger, _ = New(Config{
			LogFile:       DefaultLogFile(),
			Verbosity:     0,
			ConsoleOutput: false,
		})
	}
	return globalLogger
}

// Close closes the global logger
func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// Convenience functions that use the global logger

// Debug logs a debug message using the global logger
func Debug() *zerolog.Event {
	return Get().Logger.Debug()
}

// Info logs an info message using the global logger
func Info() *zerolog.Event {
	return Get().Logger.Info()
}

// Warn logs a warning message using the global logger
func Warn() *zerolog.Event {
	return Get().Logger.Warn()
}

// Error logs an error message using the global logger
func Error() *zerolog.Event {
	return Get().Logger.Error()
}

// Fatal logs a fatal message using the global logger
func Fatal() *zerolog.Event {
	return Get().Logger.Fatal()
}
