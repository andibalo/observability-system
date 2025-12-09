package logger

import (
	"context"

	"go.uber.org/zap"
)

// Logger defines the interface for logging operations
type Logger interface {
	// Basic logging methods
	Info(msg string, fields ...Field)
	Debug(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	// Context-aware logging methods
	InfoCtx(ctx context.Context, msg string, fields ...Field)
	DebugCtx(ctx context.Context, msg string, fields ...Field)
	WarnCtx(ctx context.Context, msg string, fields ...Field)
	ErrorCtx(ctx context.Context, msg string, fields ...Field)

	// Context operations
	WithContext(ctx context.Context) Logger
	With(fields ...Field) Logger

	// Lifecycle
	Sync() error
}

// Field represents a logging field (abstraction over zap.Field)
type Field interface {
	Key() string
	Value() interface{}
}

// Config holds logger configuration
type Config struct {
	ServiceName string
	Environment string
	Level       Level
}

// Level represents log level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return "info"
	}
}

// zapField wraps zap.Field to implement our Field interface
type zapField struct {
	field zap.Field
}

func (f zapField) Key() string {
	return f.field.Key
}

func (f zapField) Value() interface{} {
	return f.field.Interface
}

// Field constructors (compatible with zap)
func String(key, value string) Field {
	return zapField{field: zap.String(key, value)}
}

func Int(key string, value int) Field {
	return zapField{field: zap.Int(key, value)}
}

func Int64(key string, value int64) Field {
	return zapField{field: zap.Int64(key, value)}
}

func Bool(key string, value bool) Field {
	return zapField{field: zap.Bool(key, value)}
}

func Any(key string, value interface{}) Field {
	return zapField{field: zap.Any(key, value)}
}

func Err(err error) Field {
	return zapField{field: zap.Error(err)}
}

func Duration(key string, value interface{}) Field {
	return zapField{field: zap.Any(key, value)}
}

// Helper to convert Field slice to zap.Field slice
func toZapFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		if zf, ok := f.(zapField); ok {
			zapFields[i] = zf.field
		} else {
			zapFields[i] = zap.Any(f.Key(), f.Value())
		}
	}
	return zapFields
}
