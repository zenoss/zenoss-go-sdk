package writer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

var _ = Describe("Destination", func() {
	var (
		ctx      context.Context
		logDest  writer.Destination
		buf      bytes.Buffer
		targetID string
	)

	BeforeEach(func() {
		ctx = context.Background()
		buf = bytes.Buffer{}
		targetID = "test"
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
			health := target.NewHealth(targetID, "")
			summaryMessage := "Test summary"
			health.Messages = []*target.Message{target.NewMessage(
				summaryMessage, errors.New("err"), true, target.Unhealthy,
			)}
			expectedMsg := fmt.Sprintf(
				"TargetID: %s, Status=Healthy, Counters=map[], Metrics=map[], Messages=[%s]",
				targetID, summaryMessage)

			err := logDest.Push(ctx, health)
			var output map[string]interface{}
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(err).Should(BeNil())
			Ω(output["message"]).Should(Equal(expectedMsg))
		})

		It("should output Health and HeartBeat info", func() {
			health := target.NewHealth(targetID, "")
			health.Heartbeat = &target.HeartBeat{
				Enabled: true,
				Beats:   true,
			}
			expectedMsg := fmt.Sprintf(
				"TargetID: %s, Status=Healthy, Heartbeat=true, Counters=map[], Metrics=map[], Messages=[]",
				targetID)

			err := logDest.Push(ctx, health)
			var output map[string]interface{}
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(err).Should(BeNil())
			Ω(output["message"]).Should(Equal(expectedMsg))
		})
	})
})
