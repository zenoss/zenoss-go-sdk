package log

import (
	"fmt"
	"log"
)

type (
	// Fields is a type alias for a map of logging fields.
	Fields = map[string]interface{}

	// Func is a type alias for a log handling function.
	Func = func(level Level, fields Fields, format string, args ...interface{})

	// Level is a type alias for a logging level.
	Level = int
)

const (
	// LevelUnset falls back to globally configured level.
	LevelUnset Level = 0

	// LevelDebug includes low-level information useful for troubleshooting.
	LevelDebug Level = 1

	// LevelInfo includes infrequent messages of interest in normal operation.
	LevelInfo Level = 2

	// LevelWarning includes messages related to misconfiguration and other non-critical problems.
	LevelWarning Level = 3

	// LevelError includes messages related to failure of operation.
	LevelError Level = 4

	// LevelDisabled disables all logging.
	LevelDisabled Level = 99
)

var (
	// GlobalFields can be set to globally set global fields for zenoss-go-sdk logging.
	GlobalFields map[string]interface{}

	// GlobalFunc can be set to globally set a custom logging function for zenoss-go-sdk.
	// Default: DefaultFunc
	GlobalFunc = DefaultFunc

	// GlobalLevel can be set to globally control logging level for zenoss-go-sdk.
	// Default: LevelInfo
	GlobalLevel = LevelInfo
)

// Logger defines an interface for a thing that supports logging.
type Logger interface {

	// Return the logger's LoggerConfig.
	GetLoggerConfig() LoggerConfig
}

// LoggerConfig specifies configuration for loggers.
type LoggerConfig struct {

	// LogFields are fields to be included with each log.
	Fields Fields

	// LogFunc is a custom logging callback function.
	Func Func

	// LogLevel is the minimum level to log.
	Level Level
}

// Error logs an ErrorLevel message.
func Error(o Logger, fields Fields, format string, args ...interface{}) {
	Log(o, LevelError, fields, format, args...)
}

// Warning logs a WarningLevel message.
func Warning(o Logger, fields Fields, format string, args ...interface{}) {
	Log(o, LevelWarning, fields, format, args...)
}

// Info logs an InfoLevel message.
func Info(o Logger, fields Fields, format string, args ...interface{}) {
	Log(o, LevelInfo, fields, format, args...)
}

// Debug logs a DebugLevel message.
func Debug(o Logger, fields Fields, format string, args ...interface{}) {
	Log(o, LevelDebug, fields, format, args...)
}

// Log logs a message at the specified level.
func Log(o Logger, level Level, fields Fields, format string, args ...interface{}) {
	lev := getEffectiveLevel(o)
	if lev == LevelUnset || lev == LevelDisabled {
		return
	}

	fn := getEffectiveFunc(o)
	if fn == nil {
		return
	}

	fn(level, getEffectiveFields(o, fields), format, args...)
}

// DefaultFunc is the default logging function.
// Overridden by GlobalFunc.
func DefaultFunc(level Level, fields Fields, format string, args ...interface{}) {
	if level >= GlobalLevel {
		if len(fields) > 0 {
			log.Printf("%s fields=%v", fmt.Sprintf(format, args...), fields)
		} else {
			log.Printf(format, args...)
		}
	}
}

func getEffectiveFields(o Logger, fields Fields) Fields {
	if o == nil {
		return mergeFields(GlobalFields, fields)
	}

	return mergeFields(GlobalFields, o.GetLoggerConfig().Fields, fields)
}

func getEffectiveLevel(o Logger) Level {
	if o == nil {
		return GlobalLevel
	}

	if level := o.GetLoggerConfig().Level; level != LevelUnset {
		return level
	}

	return GlobalLevel
}

func getEffectiveFunc(o Logger) Func {
	if o == nil {
		return GlobalFunc
	}

	if fn := o.GetLoggerConfig().Func; fn != nil {
		return fn
	}

	return GlobalFunc
}

func mergeFields(sources ...Fields) Fields {
	f := make(Fields)
	for _, source := range sources {
		for k, v := range source {
			f[k] = v
		}
	}
	return f
}
