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

var _ = Describe("Collector", func() {

	var (
		err       error
		ctx       context.Context
		cancel    context.CancelFunc
		config    *health.Config
		targets   []*target.Target
		dest      *mocks.Destination
		wr        writer.HealthWriter
		manager   health.Manager
		collector health.Collector
		msg       *target.Message
	)

	const (
		testTargetID = "test.target"

		testMetricID   = "test.metric"
		testCounterID  = "test.counter"
		totalCounterID = "total.counter"
	)

	BeforeEach(func() {
		config = health.NewConfig()
		config.CollectionCycle = 1 * time.Second

		tar, _ := target.New(
			"1", true,
			[]string{testMetricID},
			[]string{testCounterID},
			[]string{totalCounterID},
		)

		targets = []*target.Target{
			tar,
		}

		dest = &mocks.Destination{}
		dest.On("Push", mock.Anything).Return(nil)

		wr = writer.New([]writer.Destination{dest})

		msg = target.NewMessage(
			"Error msg",
			errors.New("error"),
			true, target.Unhealthy)
	})

	Context("Running collector test", func() {
		It("should return a health collector instance", func() {
			ctx, cancel = context.WithCancel(context.Background())
			manager = health.NewManager(ctx, config, wr)
			manager.AddTargets(targets)

			go manager.Start(ctx)
			time.Sleep(1 * time.Second)

			collector, err = health.GetCollector()
			time.Sleep(1 * time.Second)

			Ω(collector).ShouldNot(BeNil())
			Ω(err).Should(BeNil())
		})

		It("should return nil", func() {
			go func() {
				err = collector.HeartBeat(testTargetID)
			}()
			time.Sleep(2 * time.Second)
			Ω(err).Should(BeNil())

			err = collector.AddToCounter(testTargetID, totalCounterID, 8)
			Ω(err).Should(BeNil())

			err = collector.AddMetricValue(testTargetID, testMetricID, 35.4)
			Ω(err).Should(BeNil())

			err = collector.HealthMessage(testTargetID, msg)
			Ω(err).Should(BeNil())

			err = collector.ChangeHealth(testTargetID, target.Healthy)
			Ω(err).Should(BeNil())
		})
	})

	Context("Dead collector test", func() {
		It("should return an error", func() {
			errDeadCollector := errors.New("Collector is not running")

			cancel()
			time.Sleep(1 * time.Second)

			err = collector.HeartBeat(testTargetID)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.AddToCounter(testTargetID, totalCounterID, -2)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.AddMetricValue(testTargetID, testMetricID, 25.0)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.HealthMessage(testTargetID, msg)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.ChangeHealth(testTargetID, target.Healthy)
			Ω(err).Should(Equal(errDeadCollector))
		})
	})
})
