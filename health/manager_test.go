package health_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var _ = Describe("Health Manager", func() {

	var (
		ctx     context.Context
		manager health.Manager

		mesuresCh chan *health.TargetMeasurement
		healthCh  chan *target.Health
		targetCh  chan *target.Target

		testTargetID     = "test.target"
		testMetric       = "test.metric"
		testCounter      = "test.counter"
		testTotalCounter = "total.counter"
	)

	addTestTarget := func() {
		tar, _ := target.New(
			testTargetID, utils.DefaultTargetType, true,
			[]string{testMetric},
			[]string{testCounter},
			[]string{testTotalCounter},
		)
		targets := []*target.Target{tar}

		manager.AddTargets(targets)
	}

	Context("startup + shutdown", func() {
		It("should start and stop by context cancel", func() {
			ctx, cancel := context.WithCancel(context.Background())

			config := health.NewConfig()
			config.CollectionCycle = 200 * time.Millisecond
			// don't spam logs during the test
			config.LogLevel = "fatal"
			manager := health.NewManager(ctx, config)

			mesuresCh := make(chan *health.TargetMeasurement)
			healthCh := make(chan *target.Health)
			targetCh := make(chan *target.Target)

			manager.Start(ctx, mesuresCh, healthCh, targetCh)
			Ω(manager.IsStarted()).Should(BeTrue())
			cancel()
			time.Sleep(1 * time.Second)
			Ω(manager.IsStarted()).Should(BeFalse())
		})

		It("should start and stop by shutdown call", func() {
			ctx := context.Background()

			config := health.NewConfig()
			config.CollectionCycle = 200 * time.Millisecond
			// don't spam logs during the test
			config.LogLevel = "fatal"
			manager := health.NewManager(ctx, config)

			mesuresCh := make(chan *health.TargetMeasurement)
			healthCh := make(chan *target.Health)
			targetCh := make(chan *target.Target)

			manager.Start(ctx, mesuresCh, healthCh, targetCh)
			Ω(manager.IsStarted()).Should(BeTrue())
			manager.Shutdown()
			Ω(manager.IsStarted()).Should(BeFalse())
		})
	})

	Context("dynamic registration off", func() {
		BeforeEach(func() {
			ctx = context.Background()

			config := health.NewConfig()
			config.CollectionCycle = 200 * time.Millisecond
			// don't spam logs during the test
			config.LogLevel = "fatal"
			manager = health.NewManager(ctx, config)

			mesuresCh = make(chan *health.TargetMeasurement)
			healthCh = make(chan *target.Health)
			targetCh = make(chan *target.Target)
		})

		AfterEach(func() {
			controller := make(chan struct{})
			go func() {
				<-healthCh
				close(controller)
			}()
			close(mesuresCh)
			manager.Shutdown()
			<-controller
		})

		Context("adding targets", func() {
			It("should add taret and push it on start", func() {
				controller := make(chan struct{})
				go func() {
					actualTarget := <-targetCh
					Ω(actualTarget).ShouldNot(BeNil())
					Ω(actualTarget.ID).Should(Equal(testTargetID))
					close(controller)
				}()
				addTestTarget()

				manager.Start(ctx, mesuresCh, healthCh, targetCh)
				<-controller
			})

			It("should push taret on add target even if started", func() {
				manager.Start(ctx, mesuresCh, healthCh, targetCh)

				controller := make(chan struct{})
				go func() {
					actualTarget := <-targetCh
					Ω(actualTarget).ShouldNot(BeNil())
					Ω(actualTarget.ID).Should(Equal(testTargetID))
					close(controller)
				}()
				addTestTarget()
				<-controller
			})
		})

		Context("simple health measurements", func() {
			BeforeEach(func() {
				controller := make(chan struct{})
				go func() {
					testTarget := <-targetCh
					Ω(testTarget.ID).Should(Equal(testTargetID))
					close(controller)
				}()
				addTestTarget()
				manager.Start(ctx, mesuresCh, healthCh, targetCh)
				<-controller
			})

			It("should update targets heartbeat", func() {
				heartbeatMeasure := &health.TargetMeasurement{
					TargetID:    testTargetID,
					MeasureType: health.Heartbeat,
				}
				mesuresCh <- heartbeatMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Heartbeat.Beats).Should(BeTrue())
				Ω(actualHealth.Heartbeat.Enabled).Should(BeTrue())
			})

			It("should update simple counter twice and cleanup it for the next time", func() {
				counterValue := int32(2)
				counterMeasure := &health.TargetMeasurement{
					TargetID:      testTargetID,
					MeasureID:     testCounter,
					MeasureType:   health.CounterChange,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Counters[testCounter]).Should(Equal(counterValue * 2))

				actualHealth = <-healthCh
				Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))
			})

			It("should update total counter twice and don't cleanup after", func() {
				counterValue := int32(2)
				counterMeasure := &health.TargetMeasurement{
					TargetID:      testTargetID,
					MeasureID:     testTotalCounter,
					MeasureType:   health.CounterChange,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue * 2))

				actualHealth = <-healthCh
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue * 2))
			})

			It("should calculate metric value and cleanup after the cycle", func() {
				metricValue1 := float64(2)
				metricValue2 := float64(6.4)
				metricMeasure := &health.TargetMeasurement{
					TargetID:    testTargetID,
					MeasureID:   testMetric,
					MeasureType: health.Metric,
					MetricValue: metricValue1,
				}
				mesuresCh <- metricMeasure
				metricMeasure = &health.TargetMeasurement{
					TargetID:    testTargetID,
					MeasureID:   testMetric,
					MeasureType: health.Metric,
					MetricValue: metricValue2,
				}
				mesuresCh <- metricMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Metrics[testMetric]).Should(Equal((metricValue1 + metricValue2) / 2))

				actualHealth = <-healthCh
				Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
			})

			It("should forward message that affects health and cleanup after the cycle", func() {
				summary := "mock"
				msg := &health.TargetMeasurement{
					TargetID:    testTargetID,
					MeasureType: health.Message,
					Message: &target.Message{
						Summary:      summary,
						AffectHealth: true,
						HealthStatus: target.Unhealthy,
					},
				}
				mesuresCh <- msg

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Status).Should(Equal(target.Unhealthy))
				actualMsg := actualHealth.Messages[0]
				Ω(actualMsg.HealthStatus).Should(Equal(target.Unhealthy))
				Ω(actualMsg.Summary).Should(Equal(summary))

				actualHealth = <-healthCh
				Ω(len(actualHealth.Messages)).Should(Equal(0))
				Ω(actualHealth.Status).Should(Equal(target.Unhealthy))
			})

			It("should forward message that don't affect health", func() {
				summary := "mock"
				msg := &health.TargetMeasurement{
					TargetID:    testTargetID,
					MeasureType: health.Message,
					Message: &target.Message{
						Summary:      summary,
						AffectHealth: false,
						HealthStatus: target.Unhealthy,
					},
				}
				mesuresCh <- msg

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Status).Should(Equal(target.Healthy))
				Ω(actualHealth.Messages[0].Summary).Should(Equal(summary))
			})

			It("should change health status if it is called manually and shouldn't cleanup after", func() {
				hStatus := &health.TargetMeasurement{
					TargetID:     testTargetID,
					MeasureType:  health.HealthStatus,
					HealthStatus: target.Degrade,
				}
				mesuresCh <- hStatus

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Status).Should(Equal(target.Degrade))

				actualHealth = <-healthCh
				Ω(actualHealth.Status).Should(Equal(target.Degrade))
			})
		})

		Context("missing target or its component", func() {
			BeforeEach(func() {
				manager.Start(ctx, mesuresCh, healthCh, targetCh)
			})

			It("the first push shouldn't affect anything", func() {
				counterValue := int32(2)
				counterMeasure := &health.TargetMeasurement{
					TargetID:      testTargetID,
					MeasureType:   health.CounterChange,
					MeasureID:     testTotalCounter,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure

				go func() {
					testTarget := <-targetCh
					Ω(testTarget.ID).Should(Equal(testTargetID))
				}()
				addTestTarget()

				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue))
			})

			It("the counter with unregistered id shouldn't affect anything", func() {
				go func() {
					testTarget := <-targetCh
					Ω(testTarget.ID).Should(Equal(testTargetID))
				}()
				addTestTarget()
				fakeID := "fakerID"

				counterValue := int32(2)
				counterMeasure := &health.TargetMeasurement{
					TargetID:      testTargetID,
					MeasureType:   health.CounterChange,
					MeasureID:     fakeID,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(int32(0)))
				_, ok := actualHealth.Counters[fakeID]
				Ω(ok).Should(BeFalse())
			})

			It("the metric with unregistered id shouldn't affect anything", func() {
				go func() {
					testTarget := <-targetCh
					Ω(testTarget.ID).Should(Equal(testTargetID))
				}()
				addTestTarget()
				fakeID := "fakerID"

				counterValue := float64(2)
				counterMeasure := &health.TargetMeasurement{
					TargetID:    testTargetID,
					MeasureType: health.Metric,
					MeasureID:   fakeID,
					MetricValue: counterValue,
				}
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.TargetID).Should(Equal(testTargetID))
				Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
				_, ok := actualHealth.Metrics[fakeID]
				Ω(ok).Should(BeFalse())
			})
		})
	})

	Context("dynamic registration on", func() {
		BeforeEach(func() {
			ctx = context.Background()

			config := health.NewConfig()
			config.CollectionCycle = 200 * time.Millisecond
			config.RegistrationOnCollect = true
			// don't spam logs during the test
			config.LogLevel = "fatal"
			manager = health.NewManager(ctx, config)

			mesuresCh = make(chan *health.TargetMeasurement)
			healthCh = make(chan *target.Health)
			targetCh = make(chan *target.Target)

			manager.Start(ctx, mesuresCh, healthCh, targetCh)
		})

		AfterEach(func() {
			go func() {
				<-healthCh
			}()
			close(mesuresCh)
			manager.Shutdown()
		})

		It("should add the target dynamicaly by heartbeat", func() {
			heartbeatMeasure := &health.TargetMeasurement{
				TargetID:    testTargetID,
				MeasureType: health.Heartbeat,
			}
			mesuresCh <- heartbeatMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Heartbeat.Beats).Should(BeTrue())
			Ω(actualHealth.Heartbeat.Enabled).Should(BeTrue())
		})

		It("should add the target dynamicaly by metric", func() {
			metricValue := float64(6.4)
			metricMeasure := &health.TargetMeasurement{
				TargetID:    testTargetID,
				MeasureID:   testMetric,
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Metrics[testMetric]).Should(Equal(metricValue))
		})

		It("should add the target dynamicaly by counter", func() {
			counterValue := int32(2)
			counterMeasure := &health.TargetMeasurement{
				TargetID:      testTargetID,
				MeasureID:     testCounter,
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}
			mesuresCh <- counterMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Counters[testCounter]).Should(Equal(counterValue))
		})

		It("shouldn't add the counter if id is not unique per target", func() {
			go func() {
				testTarget := <-targetCh
				Ω(testTarget.ID).Should(Equal(testTargetID))
			}()
			addTestTarget()
			counterValue := int32(2)
			counterMeasure := &health.TargetMeasurement{
				TargetID:      testTargetID,
				MeasureID:     testMetric, // here we use metric id but its already taken
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}
			mesuresCh <- counterMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Counters[testMetric]).Should(Equal(int32(0)))
			Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
		})

		It("should add the counter if id is unique per target", func() {
			newMeasureID := "mocked.id"
			go func() {
				testTarget := <-targetCh
				Ω(testTarget.ID).Should(Equal(testTargetID))
			}()
			addTestTarget()
			counterValue := int32(3)
			counterMeasure := &health.TargetMeasurement{
				TargetID:      testTargetID,
				MeasureID:     newMeasureID,
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}
			mesuresCh <- counterMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))
			Ω(actualHealth.Counters[newMeasureID]).Should(Equal(counterValue))
		})

		It("shouldn't add the metric if id is not unique per target", func() {
			go func() {
				testTarget := <-targetCh
				Ω(testTarget.ID).Should(Equal(testTargetID))
			}()
			addTestTarget()
			metricValue := float64(2)
			metricMeasure := &health.TargetMeasurement{
				TargetID:    testTargetID,
				MeasureID:   testCounter, // here we use counter id but its already taken
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Metrics[testCounter]).Should(Equal(float64(0)))
			Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))
		})

		It("should add the metric if id is unique per target", func() {
			newMeasureID := "mocked.id"
			go func() {
				testTarget := <-targetCh
				Ω(testTarget.ID).Should(Equal(testTargetID))
			}()
			addTestTarget()
			metricValue := float64(2.3)
			metricMeasure := &health.TargetMeasurement{
				TargetID:    testTargetID,
				MeasureID:   newMeasureID,
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.TargetID).Should(Equal(testTargetID))
			Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
			Ω(actualHealth.Metrics[newMeasureID]).Should(Equal(metricValue))
		})
	})
})
