package log

import (
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

var (
	logger zerolog.Logger
	mutex  *sync.Mutex
	once   sync.Once
)

// GetLogger returns copy of the configured logger
func GetLogger() *zerolog.Logger {
	once.Do(initLogger)
	mutex.Lock()
	defer mutex.Unlock()
	// we want to return link to the zerolog.Logger but
	// not to the initial logger object here because we can update it during runtime
	loggerCopy := logger
	return &loggerCopy
}

// SetLogLevel updates health logger log level
func SetLogLevel(level string) {
	once.Do(initLogger)
	mutex.Lock()
	defer mutex.Unlock()

	logLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		GetLogger().Error().Msgf("Invalid log level: %s", level)
		return
	}
	logger = logger.Level(logLevel)
}

// SetLogger updates health logger with your own zerolog.Logger instance
func SetLogger(log *zerolog.Logger) {
	once.Do(initLogger)
	mutex.Lock()
	defer mutex.Unlock()

	if log != nil {
		logger = *log
	}
}

func initLogger() {
	mutex = &sync.Mutex{}
	logger = zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Str("daemon", "health").
		Logger()
}
