// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"strings"
)

// TestLogger is a logger that outputs to testing.T/B
type TestLogger struct {
	t interface {
		Logf(format string, args ...interface{})
	}
}

// NewTestLogger creates a logger for tests
func NewTestLogger(t interface {
	Logf(format string, args ...interface{})
}) Logger {
	return &TestLogger{t: t}
}

func (l *TestLogger) format(level, msg string, keysAndValues ...interface{}) string {
	prefix := fmt.Sprintf("[%s] %s", level, msg)

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

	return prefix
}

func (l *TestLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.t.Logf("%s", l.format("DEBUG", msg, keysAndValues...))
}

func (l *TestLogger) Info(msg string, keysAndValues ...interface{}) {
	l.t.Logf("%s", l.format("INFO", msg, keysAndValues...))
}

func (l *TestLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.t.Logf("%s", l.format("WARN", msg, keysAndValues...))
}

func (l *TestLogger) Error(msg string, keysAndValues ...interface{}) {
	l.t.Logf("%s", l.format("ERROR", msg, keysAndValues...))
}
