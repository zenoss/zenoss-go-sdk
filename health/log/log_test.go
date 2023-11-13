package log_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
)

func TestLog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log Suite")
}

var _ = Describe("Log", func() {
	var log *zerolog.Logger

	Context("GetLogger", func() {
		It("should return configured logger instance", func() {
			log = logging.GetLogger()

			Ω(log).ShouldNot(BeNil())
			Ω(log.GetLevel()).Should(Equal(zerolog.InfoLevel))
		})
	})

	Context("SetLogger", func() {
		It("should return a set logger instance", func() {
			newLog := zerolog.New(os.Stdout)
			logging.SetLogger(&newLog)
			log = logging.GetLogger()

			Ω(log).Should(Equal(&newLog))
		})

		It("should return initial logger instance", func() {
			logging.SetLogger(nil)
			log = logging.GetLogger()

			Ω(log).ShouldNot(BeNil())
		})
	})

	Context("SetLogLevel", func() {
		It("should return updated log level", func() {
			logging.SetLogLevel("Debug")
			log := logging.GetLogger()

			Ω(log.GetLevel()).Should(Equal(zerolog.DebugLevel))
		})

		It("should output an error", func() {
			buf := bytes.Buffer{}
			log := logging.GetLogger().Output(&buf)
			logging.SetLogger(&log)

			logging.SetLogLevel("WrongLevel")
			var output map[string]string
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(output["message"]).Should(Equal("Invalid log level: WrongLevel"))
		})
	})
})
