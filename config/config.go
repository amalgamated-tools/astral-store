package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// GitCommit is used as the application version string, set by LD flags.
var GitCommit string

// LogLevelType represents a log level.
type LogLevelType string

// List all available log levels.
const (
	LogLevelInfo  LogLevelType = "info"
	LogLevelWarn  LogLevelType = "warn"
	LogLevelError LogLevelType = "error"
	LogLevelDebug LogLevelType = "debug"
	LogLevelTrace LogLevelType = "trace"
)

// LogFormatType represents a log format.
type LogFormatType string

// List all available log formats.
const (
	LogFormatText LogFormatType = "text"
	LogFormatJSON LogFormatType = "json"
)

type Sentry struct {
	Debug   bool   `json:"debug"`
	Release string `json:"release"`
}

// Config represents the service configuration.
type Config struct {
	Server *Server `json:"server"`
	Sentry *Sentry `json:"sentry"`
}

// Server describes the server related config options.
type Server struct {
	Address   string        `json:"address"`
	LogLevel  LogLevelType  `json:"log_level"`
	LogFormat LogFormatType `json:"log_format"`
}

// Default returns an initialized default config.
func Default() *Config {
	return &Config{
		Server: &Server{
			Address:   "0.0.0.0:7677",
			LogLevel:  LogLevelDebug,
			LogFormat: LogFormatText,
		},
		Sentry: &Sentry{
			Debug:   true,
			Release: GitCommit,
		},
	}
}

func (c *Config) String() string {
	jsonObj, _ := json.Marshal(c)
	return string(jsonObj)
}

// Parse loads and parses a given configuration file.
// Precedence is ENV -> config.json -> default
func Parse(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err = json.Unmarshal(content, cfg); err != nil {
		return nil, err
	}

	serverAddressEnv := os.Getenv("ADDRESS")
	if serverAddressEnv != "" {
		cfg.Server.Address = serverAddressEnv
	}

	return cfg, nil
}
