package logger

import (
	"log"
	"os"
)

// Level represents the severity of a log message.
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// Logger is a simple structured logger wrapping the standard log package.
// Replace with Zap or Zerolog for production.
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new Logger at the given level.
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", log.LstdFlags|log.Lmsgprefix),
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.logger.Printf("[WARN] "+format, args...)
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.logger.Printf("[ERROR] "+format, args...)
	}
}
