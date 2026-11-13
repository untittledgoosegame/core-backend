package logger

import (
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

type Config struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Setup(cfg Config) {
	level, err := log.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = log.InfoLevel
	}

	formatter := log.TextFormatter
	if strings.EqualFold(cfg.Format, "json") {
		formatter = log.JSONFormatter
	}

	log.SetDefault(log.NewWithOptions(os.Stderr, log.Options{
		Level:           level,
		Formatter:       formatter,
		ReportTimestamp: true,
		TimeFormat:      time.DateTime,
	}))
}
