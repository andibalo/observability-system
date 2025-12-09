package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger implements the Logger interface using zap
type zapLogger struct {
	logger *zap.Logger
	config Config
}

// NewZapLogger creates a new zap-based logger instance
func NewZapLogger(config Config) (Logger, error) {
	zapConfig := zap.NewProductionConfig()

	// Configure encoding
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapConfig.EncoderConfig.MessageKey = "message"
	zapConfig.EncoderConfig.LevelKey = "level"
	zapConfig.EncoderConfig.CallerKey = "caller"

	// Set log level based on environment
	if config.Environment == "development" {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		zapConfig.Development = true
		zapConfig.Encoding = "console"
	} else {
		zapConfig.Level = zap.NewAtomicLevelAt(toZapLevel(config.Level))
		zapConfig.Encoding = "json"
	}

	logger, err := zapConfig.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
	if err != nil {
		return nil, err
	}

	// Add service name to all logs
	logger = logger.With(
		zap.String("service", config.ServiceName),
		zap.String("environment", config.Environment),
	)

	return &zapLogger{
		logger: logger,
		config: config,
	}, nil
}

// NewDefaultLogger creates a logger with default configuration
func NewDefaultLogger(serviceName, environment string) (Logger, error) {
	return NewZapLogger(Config{
		ServiceName: serviceName,
		Environment: environment,
		Level:       InfoLevel,
	})
}

func toZapLevel(level Level) zapcore.Level {
	switch level {
	case DebugLevel:
		return zap.DebugLevel
	case InfoLevel:
		return zap.InfoLevel
	case WarnLevel:
		return zap.WarnLevel
	case ErrorLevel:
		return zap.ErrorLevel
	case FatalLevel:
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

// Info logs an info message
func (l *zapLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, toZapFields(fields)...)
}

// Debug logs a debug message
func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, toZapFields(fields)...)
}

// Warn logs a warning message
func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, toZapFields(fields)...)
}

// Error logs an error message
func (l *zapLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, toZapFields(fields)...)
}

// Fatal logs a fatal message and exits
func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.logger.Fatal(msg, toZapFields(fields)...)
}

// WithContext returns a logger with context values
func (l *zapLogger) WithContext(ctx context.Context) Logger {
	logger := l.logger

	if requestID := GetRequestID(ctx); requestID != "" {
		logger = logger.With(zap.String("request_id", requestID))
	}

	if userID := GetUserID(ctx); userID != "" {
		logger = logger.With(zap.String("user_id", userID))
	}

	return &zapLogger{
		logger: logger,
		config: l.config,
	}
}

// With returns a logger with additional fields
func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		logger: l.logger.With(toZapFields(fields)...),
		config: l.config,
	}
}

// InfoCtx logs with context
func (l *zapLogger) InfoCtx(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Info(msg, fields...)
}

// DebugCtx logs with context
func (l *zapLogger) DebugCtx(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Debug(msg, fields...)
}

// WarnCtx logs with context
func (l *zapLogger) WarnCtx(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Warn(msg, fields...)
}

// ErrorCtx logs with context
func (l *zapLogger) ErrorCtx(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Error(msg, fields...)
}

// Sync flushes any buffered log entries
func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}
