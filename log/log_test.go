package log_test

import (
	"fmt"
	stdlog "log"
	"math/rand"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/zenoss/zenoss-go-sdk/log"
)

func TestLog(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	RunSpecs(t, "Log Suite")
}

var (
	// Ensure MockLogger implements log.Logger interface.
	_ log.Logger = (*MockLogger)(nil)
)

type MockLogger struct {
	LoggerConfig log.LoggerConfig
}

func (l *MockLogger) GetLoggerConfig() log.LoggerConfig {
	return l.LoggerConfig
}

var _ = Describe("Log", func() {
	var logOutput *gbytes.Buffer
	var mockLogger log.Logger

	BeforeEach(func() {
		logOutput = gbytes.NewBuffer()
		stdlog.SetOutput(logOutput)
	})

	AfterEach(func() {
		stdlog.SetOutput(os.Stdout)
	})

	Context("with nil logger", func() {
		It("logs error, warning, and info", func() {
			log.Error(nil, log.Fields{"level": 4}, "test %s %d", "one", 1)
			Ω(logOutput).Should(gbytes.Say(`test one 1 fields=map\[level:4\]`))

			log.Warning(nil, log.Fields{"level": 3}, "test %s %d", "one", 1)
			Ω(logOutput).Should(gbytes.Say(`test one 1 fields=map\[level:3\]`))

			log.Info(nil, log.Fields{"level": 2}, "test %s %d", "one", 1)
			Ω(logOutput).Should(gbytes.Say(`.+`))
		})

		It("doesn't log debug", func() {
			log.Debug(nil, log.Fields{"level": 1}, "test %s %d", "one", 1)
			Ω(logOutput).ShouldNot(gbytes.Say(`.+`))
		})
	})

	Context("with nil log func", func() {
		BeforeEach(func() {
			mockLogger = &MockLogger{}
		})

		It("logs errors", func() {
			log.Error(mockLogger, nil, "test error")
			Ω(logOutput).Should(gbytes.Say("test error"))
		})
	})

	Context("with nil log funcs", func() {
		BeforeEach(func() {
			log.GlobalFunc = nil
			mockLogger = &MockLogger{}
		})

		AfterEach(func() {
			log.GlobalFunc = log.DefaultFunc
		})

		It("logs nothing", func() {
			log.Error(mockLogger, nil, "test")
			Ω(logOutput).ShouldNot(gbytes.Say(`.+`))
		})
	})

	Context("with logging disabled", func() {
		BeforeEach(func() {
			log.GlobalLevel = log.LevelDisabled
			mockLogger = &MockLogger{}
		})

		AfterEach(func() {
			log.GlobalLevel = log.LevelInfo
		})

		It("logs nothing", func() {
			log.Error(mockLogger, nil, "test")
			Ω(logOutput).ShouldNot(gbytes.Say(`.+`))
		})
	})

	Context("with log func", func() {
		BeforeEach(func() {
			mockLogger = &MockLogger{
				LoggerConfig: log.LoggerConfig{
					Func: func(level log.Level, fields log.Fields, format string, args ...interface{}) {
						stdlog.Printf("custom %s", format)
					},
				},
			}
		})

		It("logs custom debug", func() {
			log.Debug(mockLogger, nil, "test")
			Ω(logOutput).Should(gbytes.Say("custom test"))
		})
	})

	Context("with full configuration", func() {
		var mockLogger log.Logger

		BeforeEach(func() {
			mockLogger = &MockLogger{
				LoggerConfig: log.LoggerConfig{
					Fields: log.Fields{"logger": "value"},
					Func: func(level log.Level, fields log.Fields, format string, args ...interface{}) {
						stdlog.Printf(fmt.Sprintf("custom %s fields=%v", format, fields))
					},
					Level: log.LevelDebug,
				},
			}
		})

		It("logs logger fields", func() {
			log.Debug(mockLogger, nil, "test")
			Ω(logOutput).Should(gbytes.Say(`custom test fields=map\[logger:value\]`))
		})

		It("logs merged fields", func() {
			log.Debug(mockLogger, log.Fields{"log": "value"}, "test")
			Ω(logOutput).Should(gbytes.Say(`custom test fields=map\[log:value logger:value\]`))
		})
	})
})
