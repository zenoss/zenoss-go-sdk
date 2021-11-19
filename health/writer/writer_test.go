package writer_test

import (
	"bytes"
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
	"github.com/zenoss/zenoss-go-sdk/health/writer/mocks"
)

var _ = Describe("Writer", func() {
	var (
		healthWriter writer.HealthWriter
		log          zerolog.Logger
		buf          bytes.Buffer
		mockDest     *mocks.Destination
		dests        []writer.Destination
	)

	BeforeEach(func() {
		buf = bytes.Buffer{}
		log = logging.GetLogger().Output(&buf)
		logging.SetLogger(&log)

		logDest := writer.NewLogDestination(&log)
		mockDest = &mocks.Destination{}
		dests = []writer.Destination{logDest, mockDest}

		mockDest.On("Push", mock.AnythingOfType("*target.Health")).Return(
			errors.New("Unable to push health message"),
		)
	})

	Context("New", func() {
		It("should return a new HealthWriter instance", func() {
			healthWriter = writer.New(dests)
			Ω(healthWriter).ShouldNot(BeNil())
		})
	})

	Context("Start", func() {
		It("should log Health info and mocked destination error", func() {
			h := target.NewHealth("1")

			ch := make(chan *target.Health)
			go healthWriter.Start(ch)
			ch <- h
			time.Sleep(1 * time.Second)
			close(ch)
			out := buf.String()

			Ω(strings.Contains(out,
				"\"message\":\"TargetID: 1, Status=Healthy, Counters=map[], Metrics=map[], Messages=[]\"",
			)).Should(BeTrue())
			Ω(strings.Contains(out,
				"\"error\":\"Unable to push health message\"",
			)).Should(BeTrue())
		})
	})
})