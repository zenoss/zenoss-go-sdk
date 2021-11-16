package health_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
	"github.com/zenoss/zenoss-go-sdk/health/writer/mocks"
)

var _ = Describe("Manager", func() {

	var (
		err        error
		ctx        context.Context
		cancel     context.CancelFunc
		config     *health.Config
		targets    []*target.Target
		dest       *mocks.Destination
		wr         writer.HealthWriter
		manager    health.Manager
		collector  health.Collector
		msg1, msg2 *target.Message
	)

	const (
		testTargetID = "test.target"

		testMetricID   = "test.metric"
		testCounterID1 = "test.counter1"
		testCounterID2 = "test.counter2"
		totalCounterID = "total.counter"
	)

	Context("RegistrationOnCollect", func() {
		It("should return nil", func() {
			config = health.NewConfig()
			config.CollectionCycle = 1 * time.Second

			tar, _ := target.New(
				"1", true,
				[]string{testMetricID},
				[]string{testCounterID1, testCounterID2},
				[]string{totalCounterID},
			)

			targets = []*target.Target{
				tar,
			}

			dest = &mocks.Destination{}
			dest.On("Push", mock.Anything).Return(nil)

			wr = writer.New([]writer.Destination{dest})
			ctx, cancel = context.WithCancel(context.Background())
			config.RegistrationOnCollect = true
			manager = health.NewManager(ctx, config, wr)
			manager.AddTargets(targets)

			msg1 = target.NewMessage(
				"Error msg",
				errors.New("error"),
				false, target.Healthy)
			msg2 = target.NewMessage(
				"Info msg",
				errors.New("info"),
				false, target.Healthy)

			go manager.Start(ctx)
			time.Sleep(1 * time.Second)

			collector, err = health.GetCollector()
			time.Sleep(1 * time.Second)

			go func() {
				err = collector.HeartBeat(testTargetID)
			}()
			time.Sleep(2 * time.Second)

			err = collector.AddToCounter(testTargetID, totalCounterID, 3)
			err = collector.AddToCounter(testTargetID, totalCounterID, 1)

			err = collector.AddToCounter(testTargetID, testCounterID1, 2)
			err = collector.AddToCounter(testTargetID, testCounterID1, 6)

			err = collector.AddToCounter(testTargetID, testCounterID2, 5)
			err = collector.AddToCounter(testTargetID, testCounterID2, 3)

			err = collector.AddMetricValue(testTargetID, testMetricID, 5)
			err = collector.AddMetricValue(testTargetID, testMetricID, 0)

			err = collector.HealthMessage(testTargetID, msg1)
			err = collector.HealthMessage(testTargetID, msg2)

			err = collector.ChangeHealth(testTargetID, target.Healthy)

			cancel()

			Ω(err).Should(BeNil())
		})
	})
})
