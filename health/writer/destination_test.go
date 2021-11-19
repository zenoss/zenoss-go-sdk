package writer_test

import (
	"bytes"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

var _ = Describe("Destination", func() {
	var logDest writer.Destination
	var buf bytes.Buffer

	BeforeEach(func() {
		buf = bytes.Buffer{}
	})

	Context("NewLogDestination", func() {
		It("should return a new LogDestination", func() {
			log := logging.GetLogger().Output(&buf)
			logDest = writer.NewLogDestination(&log)

			Ω(logDest).ShouldNot(BeNil())
		})
	})

	Context("Push", func() {
		It("should output Health info", func() {
			health := target.NewHealth("1")
			health.Messages = []*target.Message{target.NewMessage(
				"sum", errors.New("err"), true, target.Unhealthy,
			)}
			expectedMsg := "TargetID: 1, Status=Healthy, Counters=map[], Metrics=map[], Messages=[sum]"

			err := logDest.Push(health)
			var output map[string]interface{}
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(err).Should(BeNil())
			Ω(output["message"]).Should(Equal(expectedMsg))
		})

		It("should output Health and HeartBeat info", func() {
			health := target.NewHealth("2")
			health.Heartbeat = &target.HeartBeat{
				Enabled: true,
				Beats:   true,
			}
			expectedMsg := "TargetID: 2, Status=Healthy, Heartbeat=true, Counters=map[], Metrics=map[], Messages=[]"

			err := logDest.Push(health)
			var output map[string]interface{}
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(err).Should(BeNil())
			Ω(output["message"]).Should(Equal(expectedMsg))
		})
	})
})
