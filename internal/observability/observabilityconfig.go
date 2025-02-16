package observability

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "LOG_LEVELS_DEBUGLEVEL"
	LogLevelInfo  LogLevel = "LOG_LEVELS_INFOLEVEL"
	LogLevelWarn  LogLevel = "LOG_LEVELS_WARNLEVEL"
	LogLevelError LogLevel = "LOG_LEVELS_ERRORLEVEL"
)

// ObservabilityConfig represents OpenTelemetry configuration
type ObservabilityConfig struct {
	ServiceName    string `json:"serviceName" mapstructure:"serviceName"`
	ServiceVersion string `json:"serviceVersion" mapstructure:"serviceVersion"`
	Environment    string `json:"environment" mapstructure:"environment"`
	OTelEndpoint   string `json:"otelEndpoint" mapstructure:"otelEndpoint"`
}
