package writer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zenoss/zenoss-go-sdk/health/component"
	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

var _ = Describe("Destination", func() {
	var (
		ctx         context.Context
		logDest     writer.Destination
		buf         bytes.Buffer
		componentID string
	)

	BeforeEach(func() {
		ctx = context.Background()
		buf = bytes.Buffer{}
		componentID = "test"
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
			health := component.NewHealth(componentID, "", "")
			summaryMessage := "Test summary"
			health.Messages = []*component.Message{component.NewMessage(
				summaryMessage, errors.New("err"), true, component.Unhealthy,
			)}
			expectedMsg := fmt.Sprintf(
				"ComponentID: %s, Status=Healthy, Counters=map[], Metrics=map[], Messages=[%s]",
				componentID, summaryMessage)

			err := logDest.Push(ctx, health)
			var output map[string]interface{}
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(err).Should(BeNil())
			Ω(output["message"]).Should(Equal(expectedMsg))
		})

		It("should output Health and HeartBeat info", func() {
			health := component.NewHealth(componentID, "", "")
			health.Heartbeat = &component.HeartBeat{
				Enabled: true,
				Beats:   true,
			}
			expectedMsg := fmt.Sprintf(
				"ComponentID: %s, Status=Healthy, Heartbeat=true, Counters=map[], Metrics=map[], Messages=[]",
				componentID)

			err := logDest.Push(ctx, health)
			var output map[string]interface{}
			_ = json.Unmarshal(buf.Bytes(), &output)

			Ω(err).Should(BeNil())
			Ω(output["message"]).Should(Equal(expectedMsg))
		})
	})
})
