package health_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/mocks"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Health Manager Suite", []Reporter{junitReporter})
}

var _ = Describe("Manager", func() {

	const (
		testTargetID = "test.target"

		testMetric1  = "test.metric.1"
		testMetric2  = "test.metric.2"
		testCounter1 = "test.counter.1"
		testCounter2 = "test.counter.2"
		totalCounter = "total.counter"

		wrongID = "wrong.ID"

		cycle = 200 * time.Millisecond
	)

	Context("When collector is not initialized", func() {
		It("GetCollector should return an error", func() {
			collector, err := health.GetCollector()

			Ω(collector).Should(BeNil())
			Ω(err).Should(Equal(utils.ErrDeadCollector))
		})
	})

	Context("Manager tests", func() {
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
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			config = health.NewConfig()
			config.CollectionCycle = cycle
			// don't spam logs during the test
			config.LogLevel = "fatal"

			dest = &mocks.Destination{}
			dest.On("Push", ctx, mock.Anything).Return(nil)
			dest.On("Register", ctx, mock.Anything).Return(nil)

			wr = writer.New([]writer.Destination{dest})
			tar, _ := target.New(
				testTargetID, "", true,
				[]string{testMetric1, testMetric2},
				[]string{testCounter1, testCounter2},
				[]string{totalCounter},
			)
			targets = []*target.Target{tar}

			manager = health.NewManager(ctx, config)
			manager.AddTargets(targets)
			go health.FrameworkStart(ctx, config, manager, wr)
			time.Sleep(cycle / 2)

			collector, err = health.GetCollector()
			time.Sleep(cycle)
		})

		AfterEach(func() {
			cancel()
			time.Sleep(1 * time.Second)
		})

		It("collector should be not nil", func() {
			Ω(collector).ShouldNot(BeNil())
		})

		Context("HeartBeat test", func() {
			It("should push heartbeat to the registered target", func() {
				go func() {
					err = collector.HeartBeat(testTargetID)
				}()
				time.Sleep(3 * cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(len(dest.Calls) > 2).Should(BeTrue())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Heartbeat.Enabled).Should(BeTrue())
				Ω(lastPush.Heartbeat.Beats).Should(BeTrue())
			})
		})

		Context("AddToCounter test", func() {
			It("should push counter change measure to the registered target", func() {
				var counterIncr int32

				counterIncr = 1
				err = collector.AddToCounter(testTargetID, testCounter1, counterIncr)
				time.Sleep(cycle)
				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Counters[testCounter1]).Should(Equal(counterIncr))

				counterIncr = 2
				err = collector.AddToCounter(testTargetID, testCounter2, counterIncr)
				time.Sleep(cycle)
				lastPush = dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Counters[testCounter2]).Should(Equal(counterIncr))

				counterIncr = 3
				err = collector.AddToCounter(testTargetID, totalCounter, counterIncr)
				time.Sleep(cycle)
				lastPush = dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Counters[totalCounter]).Should(Equal(counterIncr))
			})

			// "Unable to update target counter" error check
			It("should raise an error while updating unregistered counter", func() {
				var counterIncr int32 = 4
				err = collector.AddToCounter(testTargetID, wrongID, counterIncr)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				_, ok := lastPush.Counters[wrongID]

				Ω(ok).Should(BeFalse())
			})
		})

		Context("AddMetricValue test", func() {
			It("should push metric measure to the registered target", func() {
				var metricValue float64

				metricValue = 1.1
				err = collector.AddMetricValue(testTargetID, testMetric1, metricValue)
				time.Sleep(cycle)
				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Metrics[testMetric1]).Should(Equal(metricValue))

				metricValue = 1.2
				err = collector.AddMetricValue(testTargetID, testMetric2, metricValue)
				time.Sleep(cycle)
				lastPush = dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Metrics[testMetric2]).Should(Equal(metricValue))
			})

			// "Unable to update target metric" error check
			It("should raise an error while updating unregistered metric", func() {
				metricValue := 1.3
				err = collector.AddMetricValue(testTargetID, wrongID, metricValue)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				_, ok := lastPush.Metrics[wrongID]

				Ω(ok).Should(BeFalse())
			})
		})

		Context("HealthMessage test", func() {
			It("should push health-affecting message to the registered target", func() {
				affectHealthMsg := target.NewMessage(
					"Error msg",
					errors.New("error"),
					true, target.Unhealthy)

				err = collector.HealthMessage(testTargetID, affectHealthMsg)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(lastPush.Messages[0]).Should(Equal(affectHealthMsg))
			})

			It("should push non-health-affecting message to the registered target", func() {
				infoMsg := target.NewMessage(
					"Info msg",
					errors.New("info"),
					false, target.Healthy)

				err = collector.HealthMessage(testTargetID, infoMsg)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(lastPush.Messages[0]).Should(Equal(infoMsg))
			})
		})

		Context("ChangeHealth test", func() {
			It("should push updated health status to the registered target", func() {
				err = collector.ChangeHealth(testTargetID, target.Degrade)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(lastPush.Status).Should(Equal(target.Degrade))
			})

			// TargetNotRegistered error check
			It("should raise errTargetNotRegistered", func() {
				err = collector.ChangeHealth(wrongID, target.Degrade)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).ShouldNot(Equal(wrongID))
			})
		})
	})

	Context("Manager with registration on collect enabled test", func() {
		var (
			err       error
			ctx       context.Context
			cancel    context.CancelFunc
			config    *health.Config
			targets   []*target.Target
			dest      *mocks.Destination
			wr        writer.HealthWriter
			collector health.Collector
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			config = health.NewConfig()
			config.CollectionCycle = cycle
			config.RegistrationOnCollect = true

			dest = &mocks.Destination{}
			dest.On("Push", ctx, mock.Anything).Return(nil)
			dest.On("Register", ctx, mock.Anything).Return(nil)

			wr = writer.New([]writer.Destination{dest})
			tar, _ := target.New(
				testTargetID, "", true,
				[]string{testMetric1, testMetric2},
				[]string{testCounter1, testCounter2},
				[]string{totalCounter},
			)
			targets = []*target.Target{tar}

			manager := health.NewManager(ctx, config)

			manager.AddTargets(targets)

			go health.FrameworkStart(ctx, config, manager, wr)
			time.Sleep(cycle / 2) // let it init and start

			collector, err = health.GetCollector()
			time.Sleep(cycle)
		})

		AfterEach(func() {
			cancel()
			time.Sleep(1 * time.Second)
		})

		Context("Dynamic measure ID registration test", func() {
			It("should register new counter id and push the measure", func() {
				newMeasureId := "new.id.1"
				var counterIncr int32 = 5
				err = collector.AddToCounter(testTargetID, newMeasureId, counterIncr)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Counters[newMeasureId]).Should(Equal(counterIncr))
			})

			It("should register new metric id and push the measure", func() {
				newMeasureId := "new.id.2"
				metricValue := 1.4
				err = collector.AddMetricValue(testTargetID, newMeasureId, metricValue)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)

				Ω(err).Should(BeNil())
				Ω(lastPush.TargetID).Should(Equal(testTargetID))
				Ω(lastPush.Metrics[newMeasureId]).Should(Equal(metricValue))
			})

			It("AddToCounter should raise errMeasureIDTaken", func() {
				takenMeasureId := "new.id.3"
				var counterIncr int32 = 6
				metricValue := 1.5

				err = collector.AddMetricValue(testTargetID, takenMeasureId, metricValue)
				err = collector.AddToCounter(testTargetID, takenMeasureId, counterIncr)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				_, ok := lastPush.Counters[takenMeasureId]

				Ω(ok).Should(BeFalse())
			})

			It("AddMetricValue should raise errMeasureIDTaken", func() {
				takenMeasureId := "new.id.4"
				var counterIncr int32 = 7
				metricValue := 1.6

				err = collector.AddToCounter(testTargetID, takenMeasureId, counterIncr)
				err = collector.AddMetricValue(testTargetID, takenMeasureId, metricValue)
				time.Sleep(cycle)

				lastPush := dest.Calls[len(dest.Calls)-1].Arguments[1].(*target.Health)
				_, ok := lastPush.Metrics[takenMeasureId]

				Ω(ok).Should(BeFalse())
			})
		})

		Context("Build target from measure test", func() {
			It("should build target from HeartBeat measure", func() {
				newTargetID := "new.target.1"

				go func() {
					err = collector.HeartBeat(newTargetID)
				}()
				time.Sleep(3 * cycle)

				var newTargetPush *target.Health
				for _, call := range dest.Calls {
					if call.Method == "Push" {
						push := call.Arguments[1].(*target.Health)
						if push.TargetID == newTargetID && push.Heartbeat.Enabled == true {
							newTargetPush = push
						}
					}
				}

				Ω(err).Should(BeNil())
				Ω(newTargetPush).ShouldNot(BeNil())
				Ω(newTargetPush.Heartbeat.Beats).Should(BeTrue())
			})

			It("should build target from counter change measure", func() {
				newTargetID := "new.target.2"
				newMeasureID := "new.id.5"
				var counterIncr int32 = 8

				err = collector.AddToCounter(newTargetID, newMeasureID, counterIncr)
				time.Sleep(2 * cycle)

				var newTargetPush *target.Health
				for _, call := range dest.Calls {
					if call.Method == "Push" {
						push := call.Arguments[1].(*target.Health)
						if push.TargetID == newTargetID && push.Counters[newMeasureID] == counterIncr {
							newTargetPush = push
						}
					}
				}

				Ω(err).Should(BeNil())
				Ω(newTargetPush).ShouldNot(BeNil())
			})

			It("should build target from metric measure", func() {
				newTargetID := "new.target.3"
				newMeasureID := "new.id.6"
				metricValue := 1.7

				err = collector.AddMetricValue(newTargetID, newMeasureID, metricValue)
				time.Sleep(2 * cycle)

				var newTargetPush *target.Health
				for _, call := range dest.Calls {
					if call.Method == "Push" {
						push := call.Arguments[1].(*target.Health)
						if push.TargetID == newTargetID && push.Metrics[newMeasureID] == metricValue {
							newTargetPush = push
						}
					}
				}

				Ω(err).Should(BeNil())
				Ω(newTargetPush).ShouldNot(BeNil())
			})
		})
	})

	Context("Dead collector test", func() {
		var (
			err       error
			ctx       context.Context
			cancel    context.CancelFunc
			config    *health.Config
			targets   []*target.Target
			dest      *mocks.Destination
			wr        writer.HealthWriter
			collector health.Collector
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			config = health.NewConfig()
			config.CollectionCycle = cycle
			config.RegistrationOnCollect = true

			dest = &mocks.Destination{}
			dest.On("Push", ctx, mock.Anything).Return(nil)
			dest.On("Register", ctx, mock.Anything).Return(nil)

			wr = writer.New([]writer.Destination{dest})
			tar, _ := target.New(
				testTargetID, "", true,
				[]string{testMetric1, testMetric2},
				[]string{testCounter1, testCounter2},
				[]string{totalCounter},
			)
			targets = []*target.Target{tar}

			manager := health.NewManager(ctx, config)
			manager.AddTargets(targets)
			go health.FrameworkStart(ctx, config, manager, wr)
			time.Sleep(cycle / 2) // let it init and start

			collector, err = health.GetCollector()
			time.Sleep(cycle)
		})

		It("dead collector calls should return an error", func() {
			cancel()
			time.Sleep(1 * time.Second)

			errDeadCollector := errors.New("Collector is not running")

			err = collector.HeartBeat(testTargetID)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.AddToCounter(testTargetID, totalCounter, -2)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.AddMetricValue(testTargetID, testMetric1, 25.0)
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.HealthMessage(testTargetID, &target.Message{})
			Ω(err).Should(Equal(errDeadCollector))

			err = collector.ChangeHealth(testTargetID, target.Healthy)
			Ω(err).Should(Equal(errDeadCollector))
		})
	})

	Context("frameworkStop test", func() {
		var (
			ctx     context.Context
			config  *health.Config
			targets []*target.Target
			dest    *mocks.Destination
			wr      writer.HealthWriter
		)

		It("should gracefully shut down", func() {
			ctx = context.Background()

			config = health.NewConfig()
			config.CollectionCycle = cycle
			config.RegistrationOnCollect = true

			dest = &mocks.Destination{}
			dest.On("Push", ctx, mock.Anything).Return(nil)
			dest.On("Register", ctx, mock.Anything).Return(nil)

			wr = writer.New([]writer.Destination{dest})
			tar, _ := target.New(
				testTargetID, "", true,
				[]string{testMetric1, testMetric2},
				[]string{testCounter1, testCounter2},
				[]string{totalCounter},
			)
			targets = []*target.Target{tar}

			manager := health.NewManager(ctx, config)
			manager.AddTargets(targets)
			fameworkStop := health.FrameworkStart(ctx, config, manager, wr)

			fameworkStop()
		})
	})
})
