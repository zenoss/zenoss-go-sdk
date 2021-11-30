package writer_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/mocks"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

var _ = Describe("Writer", func() {
	var (
		ctx          context.Context
		healthWriter writer.HealthWriter
		log          zerolog.Logger
		buf          bytes.Buffer
		mockDest     *mocks.Destination
		dests        []writer.Destination
	)

	BeforeEach(func() {
		ctx = context.Background()
		buf = bytes.Buffer{}
		log = logging.GetLogger().Output(&buf)
		logging.SetLogger(&log)

		logDest := writer.NewLogDestination(&log)
		mockDest = &mocks.Destination{}
		dests = []writer.Destination{logDest, mockDest}

		pushErr := "Unable to push health message"
		regErr := "Unable to register health target"
		mockDest.On("Push", ctx, mock.AnythingOfType("*target.Health")).Return(
			errors.New(pushErr),
		)
		mockDest.On("Register", ctx, mock.AnythingOfType("*target.Target")).Return(
			errors.New(regErr),
		)
	})

	Context("New", func() {
		It("should return a new HealthWriter instance", func() {
			healthWriter = writer.New(dests)
			Ω(healthWriter).ShouldNot(BeNil())
		})
	})

	Context("Start", func() {
		It("should log Health and Target info. Mocked destination shouldn't affect writer", func() {
			targetID := "testTarget"
			empty := []string{}
			hTarget, _ := target.New(
				targetID, utils.DefaultTargetType, false,
				empty, empty, empty,
			)
			h := target.NewHealth(targetID, utils.DefaultTargetType)

			hCh := make(chan *target.Health)
			tCh := make(chan *target.Target)
			go healthWriter.Start(ctx, hCh, tCh)
			tCh <- hTarget
			hCh <- h
			time.Sleep(1 * time.Second)
			close(hCh)
			close(tCh)
			out := buf.String()

			Ω(out).Should(ContainSubstring(fmt.Sprintf("TargetID: %s, Status=Healthy", targetID)))
			Ω(out).Should(ContainSubstring(fmt.Sprintf("Got target update TargetID: %s", targetID)))
		})
	})
})
