// internal/observability/observabilityconfig.go
package observability

import "go.uber.org/zap/zapcore"

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "LOG_LEVELS_DEBUGLEVEL"
	LogLevelInfo  LogLevel = "LOG_LEVELS_INFOLEVEL"
	LogLevelWarn  LogLevel = "LOG_LEVELS_WARNLEVEL"
	LogLevelError LogLevel = "LOG_LEVELS_ERRORLEVEL"
)

// GetZapLevel converts LogLevel to zapcore.Level
func (l LogLevel) GetZapLevel() zapcore.Level {
	switch l {
	case LogLevelDebug:
		return zapcore.DebugLevel
	case LogLevelWarn:
		return zapcore.WarnLevel
	case LogLevelError:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Config represents OpenTelemetry configuration
type Config struct {
	ServiceName    string `yaml:"serviceName"`
	ServiceVersion string `yaml:"serviceVersion"`
	Environment    string `yaml:"environment"`
	OTelEndpoint   string `yaml:"otelEndpoint"`
}

// LoggerConfig represents logging configuration
type LoggerConfig struct {
	Level LogLevel `yaml:"level"`
}
