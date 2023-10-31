package health_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var _ = Describe("Collector", func() {
	var (
		testTargetID = "test.target"
		cycle        = 200 * time.Millisecond
	)

	Context("simple collector", func() {
		var (
			measureCh chan *health.TargetMeasurement
			collector health.Collector
		)

		BeforeEach(func() {
			measureCh = make(chan *health.TargetMeasurement, 1)
			collector = health.NewCollector(cycle, measureCh)
		})

		AfterEach(func() {
			close(measureCh)
		})

		Context("HeartBeat", func() {
			It("should count a heartbeat and stop after cancel called", func() {
				hbCancel, err := collector.HeartBeat(testTargetID)
				Ω(err).Should(BeNil())

				hbMeasure := <-measureCh
				Ω(err).Should(BeNil())
				Ω(hbMeasure).ShouldNot(BeNil())
				Ω(hbMeasure.MeasureType).Should(Equal(health.Heartbeat))

				hbCancel()
			})
		})

		Context("AddToCounter", func() {
			It("should send counter change measure to the channel", func() {
				counter := int32(13)
				err := collector.AddToCounter(testTargetID, "test.counter", counter)
				Ω(err).Should(BeNil())

				counterMeasure := <-measureCh
				Ω(err).Should(BeNil())
				Ω(counterMeasure.MeasureType).Should(Equal(health.CounterChange))
				Ω(counterMeasure.CounterChange).Should(Equal(counter))
			})
		})

		Context("AddMetricValue", func() {
			It("should send metric measure to the channel", func() {
				metric := float64(25.6)
				err := collector.AddMetricValue(testTargetID, "test.metric", metric)
				Ω(err).Should(BeNil())

				metricMeasure := <-measureCh
				Ω(err).Should(BeNil())
				Ω(metricMeasure.MeasureType).Should(Equal(health.Metric))
				Ω(metricMeasure.MetricValue).Should(Equal(metric))
			})
		})

		Context("HealthMessage", func() {
			It("should send health message to the channel", func() {
				message := target.NewMessage(
					"Error msg",
					errors.New("error"),
					true, target.Unhealthy)

				err := collector.HealthMessage(testTargetID, message)
				Ω(err).Should(BeNil())

				messageMeasure := <-measureCh
				Ω(err).Should(BeNil())
				Ω(messageMeasure.MeasureType).Should(Equal(health.Message))
				Ω(messageMeasure.Message).Should(Equal(message))
			})
		})

		Context("ChangeHealth", func() {
			It("should send update health to the channel", func() {
				healthStatus := target.Degrade
				err := collector.ChangeHealth(testTargetID, healthStatus)
				Ω(err).Should(BeNil())

				hsMeasure := <-measureCh
				Ω(err).Should(BeNil())
				Ω(hsMeasure.MeasureType).Should(Equal(health.HealthStatus))
				Ω(hsMeasure.HealthStatus).Should(Equal(healthStatus))
			})
		})
	})

	// TODO: ZING-19127 add Serial decorator after ginkgo v2 upgrade (singleton staff should be run separately)
	Context("collector as singleton", func() {
		It("should return an error if singleton isn't set", func() {
			health.ResetCollectorSingleton()
			_, err := health.GetCollectorSingleton()
			Ω(err).Should(Equal(utils.ErrDeadCollector))
		})

		It("heartbeat should return an error if collector stopped in the middle of the process", func() {
			measureCh := make(chan *health.TargetMeasurement)
			collector := health.NewCollector(cycle, measureCh)
			health.SetCollectorSingleton(collector)
			collector, err := health.GetCollectorSingleton()
			Ω(err).Should(BeNil())
			_, err = collector.HeartBeat(testTargetID)
			Ω(err).Should(BeNil())
			health.StopCollectorSingleton()
			time.Sleep(cycle)
		})

		Context("dead collector", func() {
			const (
				mockedMeasureID = "mocked.id"
			)

			BeforeEach(func() {
				measureCh := make(chan *health.TargetMeasurement)
				collector := health.NewCollector(cycle, measureCh)
				health.SetCollectorSingleton(collector)
				health.StopCollectorSingleton()
			})

			It("heartbeat should return an error", func() {
				collector, err := health.GetCollectorSingleton()
				Ω(err).Should(BeNil())
				_, err = collector.HeartBeat(testTargetID)
				Ω(err).Should(Equal(utils.ErrDeadCollector))
			})

			It("add to counter should return an error", func() {
				collector, err := health.GetCollectorSingleton()
				Ω(err).Should(BeNil())
				err = collector.AddToCounter(testTargetID, mockedMeasureID, int32(2))
				Ω(err).Should(Equal(utils.ErrDeadCollector))
			})

			It("add metric value should return an error", func() {
				collector, err := health.GetCollectorSingleton()
				Ω(err).Should(BeNil())
				err = collector.AddMetricValue(testTargetID, mockedMeasureID, float64(2.65))
				Ω(err).Should(Equal(utils.ErrDeadCollector))
			})

			It("health message should return an error", func() {
				collector, err := health.GetCollectorSingleton()
				Ω(err).Should(BeNil())
				err = collector.HealthMessage(testTargetID, &target.Message{})
				Ω(err).Should(Equal(utils.ErrDeadCollector))
			})

			It("change health should return an error", func() {
				collector, err := health.GetCollectorSingleton()
				Ω(err).Should(BeNil())
				err = collector.ChangeHealth(testTargetID, target.Healthy)
				Ω(err).Should(Equal(utils.ErrDeadCollector))
			})
		})
	})
})
