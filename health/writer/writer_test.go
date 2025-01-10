package writer_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/zenoss/zenoss-go-sdk/health/component"
	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/mocks"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

func TestWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Writer Suite")
}

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
		regErr := "Unable to register health component"
		mockDest.On("Push", ctx, mock.AnythingOfType("*component.Health")).Return(
			errors.New(pushErr),
		)
		mockDest.On("Register", ctx, mock.AnythingOfType("*component.Component")).Return(
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
		BeforeEach(func() {
			healthWriter = writer.New(dests)
		})

		It("should start and die with shutdown", func() {
			hCh := make(chan *component.Health)
			tCh := make(chan *component.Component)
			go healthWriter.Start(ctx, hCh, tCh)
			healthWriter.Shutdown()
		})

		It("should log Health and Component info. Mocked destination shouldn't affect writer", func() {
			componentID := "testComponent"
			empty := []string{}
			hComponent, _ := component.New(
				componentID, utils.DefaultComponentType, utils.DefaultHealthTarget, false,
				empty, empty, empty,
			)
			h := component.NewHealth(componentID, utils.DefaultComponentType, utils.DefaultHealthTarget)

			var wg sync.WaitGroup
			hCh := make(chan *component.Health)
			tCh := make(chan *component.Component)

			wg.Add(1)
			go func() {
				defer wg.Done()
				healthWriter.Start(ctx, hCh, tCh)
			}()
			tCh <- hComponent
			hCh <- h
			time.Sleep(1 * time.Second)
			close(hCh)
			close(tCh)
			wg.Wait()
			out := buf.String()

			Ω(out).Should(ContainSubstring(fmt.Sprintf("ComponentID: %s", componentID)))
			Ω(out).Should(ContainSubstring(fmt.Sprintf("Status=Healthy")))
			Ω(out).Should(ContainSubstring(fmt.Sprintf("Got component update ComponentID: %s", componentID)))
		})
	})
})
