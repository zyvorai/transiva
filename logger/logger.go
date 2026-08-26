// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

type Format int

const (
	FormatText Format = iota
	FormatJSON
)

type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

type Config struct {
	Level  string
	Format string // "text" or "json"
	Output io.Writer
}

type StandardLogger struct {
	level  Level
	format Format
	logger *log.Logger
}

func New(levelStr string) Logger {
	return NewWithConfig(Config{
		Level:  levelStr,
		Format: "text",
		Output: os.Stderr,
	})
}

func NewWithConfig(cfg Config) Logger {
	level := INFO
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn", "warning":
		level = WARN
	case "error":
		level = ERROR
	}

	format := FormatText
	switch strings.ToLower(cfg.Format) {
	case "json":
		format = FormatJSON
	case "text":
		format = FormatText
	}

	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	return &StandardLogger{
		level:  level,
		format: format,
		logger: log.New(output, "", 0),
	}
}

func (l *StandardLogger) log(level Level, levelStr, msg string, keysAndValues ...interface{}) {
	if level < l.level {
		return
	}

	if l.format == FormatJSON {
		l.logJSON(levelStr, msg, keysAndValues...)
	} else {
		l.logText(levelStr, msg, keysAndValues...)
	}
}

func (l *StandardLogger) logText(levelStr, msg string, keysAndValues ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	prefix := fmt.Sprintf("[%s] %s: %s", timestamp, levelStr, msg)

	if len(keysAndValues) > 0 {
		var pairs []string
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				pairs = append(pairs, fmt.Sprintf("%v=%v", keysAndValues[i], keysAndValues[i+1]))
			}
		}
		if len(pairs) > 0 {
			prefix = fmt.Sprintf("%s | %s", prefix, strings.Join(pairs, ", "))
		}
	}

	l.logger.Println(prefix)
}

func (l *StandardLogger) logJSON(levelStr, msg string, keysAndValues ...interface{}) {
	entry := make(map[string]interface{})
	entry["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	entry["level"] = levelStr
	entry["msg"] = msg

	// Add key-value pairs to the entry
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := fmt.Sprintf("%v", keysAndValues[i])
			entry[key] = keysAndValues[i+1]
		}
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		// Fallback to text format if JSON marshaling fails
		l.logText(levelStr, msg, keysAndValues...)
		return
	}

	l.logger.Println(string(jsonBytes))
}

func (l *StandardLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.log(DEBUG, "DEBUG", msg, keysAndValues...)
}

func (l *StandardLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log(INFO, "INFO", msg, keysAndValues...)
}

func (l *StandardLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(WARN, "WARN", msg, keysAndValues...)
}

func (l *StandardLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log(ERROR, "ERROR", msg, keysAndValues...)
}
