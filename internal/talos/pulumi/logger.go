package pulumi

import (
	"github.com/sirupsen/logrus"
)

// LogrusAdapter adapts logrus.Entry to the Logger interface.
type LogrusAdapter struct {
	entry *logrus.Entry
}

// NewLogrusAdapter creates a new LogrusAdapter.
func NewLogrusAdapter(entry *logrus.Entry) *LogrusAdapter {
	return &LogrusAdapter{entry: entry}
}

// Info logs an info message with key-value pairs.
func (l *LogrusAdapter) Info(msg string, keysAndValues ...interface{}) {
	l.entry.WithFields(toFields(keysAndValues...)).Info(msg)
}

// Error logs an error message with key-value pairs.
func (l *LogrusAdapter) Error(msg string, keysAndValues ...interface{}) {
	l.entry.WithFields(toFields(keysAndValues...)).Error(msg)
}

// Debug logs a debug message with key-value pairs.
func (l *LogrusAdapter) Debug(msg string, keysAndValues ...interface{}) {
	l.entry.WithFields(toFields(keysAndValues...)).Debug(msg)
}

// toFields converts key-value pairs to logrus.Fields.
func toFields(keysAndValues ...interface{}) logrus.Fields {
	fields := logrus.Fields{}
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key, ok := keysAndValues[i].(string)
			if ok {
				fields[key] = keysAndValues[i+1]
			}
		}
	}
	return fields
}
