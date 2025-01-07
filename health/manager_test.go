package health_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var _ = Describe("Health Manager", func() {
	var (
		ctx     context.Context
		manager health.Manager

		mesuresCh   chan *health.ComponentMeasurement
		healthCh    chan *component.Health
		componentCh chan *component.Component

		testComponentID  = "test.component"
		testMetric       = "test.metric"
		testCounter      = "test.counter"
		testTotalCounter = "total.counter"
	)

	addTestComponent := func() {
		comp, _ := component.New(
			testComponentID, utils.DefaultComponentType, true,
			[]string{testMetric},
			[]string{testCounter},
			[]string{testTotalCounter},
		)
		components := []*component.Component{comp}

		manager.AddComponents(components)
	}

	Context("startup + shutdown", func() {
		It("should start and stop by context cancel", func() {
			ctx, cancel := context.WithCancel(context.Background())

			config := health.NewConfig()
			config.CollectionCycle = 200 * time.Millisecond
			// don't spam logs during the test
			config.LogLevel = "fatal"
			manager := health.NewManager(ctx, config)

			mesuresCh := make(chan *health.ComponentMeasurement)
			healthCh := make(chan *component.Health)
			componentCh := make(chan *component.Component)

			manager.Start(ctx, mesuresCh, healthCh, componentCh)
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

			mesuresCh := make(chan *health.ComponentMeasurement)
			healthCh := make(chan *component.Health)
			componentCh := make(chan *component.Component)

			manager.Start(ctx, mesuresCh, healthCh, componentCh)
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

			mesuresCh = make(chan *health.ComponentMeasurement)
			healthCh = make(chan *component.Health)
			componentCh = make(chan *component.Component)
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

		Context("adding components", func() {
			It("should add taret and push it on start", func() {
				controller := make(chan struct{})
				go func() {
					actualComponent := <-componentCh
					Ω(actualComponent).ShouldNot(BeNil())
					Ω(actualComponent.ID).Should(Equal(testComponentID))
					close(controller)
				}()
				addTestComponent()

				manager.Start(ctx, mesuresCh, healthCh, componentCh)
				<-controller
			})

			It("should push taret on add component even if started", func() {
				manager.Start(ctx, mesuresCh, healthCh, componentCh)

				controller := make(chan struct{})
				go func() {
					actualComponent := <-componentCh
					Ω(actualComponent).ShouldNot(BeNil())
					Ω(actualComponent.ID).Should(Equal(testComponentID))
					close(controller)
				}()
				addTestComponent()
				<-controller
			})
		})

		Context("simple health measurements", func() {
			BeforeEach(func() {
				controller := make(chan struct{})
				go func() {
					testComponent := <-componentCh
					Ω(testComponent.ID).Should(Equal(testComponentID))
					close(controller)
				}()
				addTestComponent()
				manager.Start(ctx, mesuresCh, healthCh, componentCh)
				<-controller
			})

			It("should update components heartbeat", func() {
				heartbeatMeasure := &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureType: health.Heartbeat,
				}
				mesuresCh <- heartbeatMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Heartbeat.Beats).Should(BeTrue())
				Ω(actualHealth.Heartbeat.Enabled).Should(BeTrue())
			})

			It("should update simple counter twice and cleanup it for the next time", func() {
				counterValue := int32(2)
				counterMeasure := &health.ComponentMeasurement{
					ComponentID:   testComponentID,
					MeasureID:     testCounter,
					MeasureType:   health.CounterChange,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Counters[testCounter]).Should(Equal(counterValue * 2))

				actualHealth = <-healthCh
				Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))
			})

			It("should update total counter twice and don't cleanup after", func() {
				counterValue := int32(2)
				counterMeasure := &health.ComponentMeasurement{
					ComponentID:   testComponentID,
					MeasureID:     testTotalCounter,
					MeasureType:   health.CounterChange,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue * 2))

				actualHealth = <-healthCh
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue * 2))
			})

			It("should calculate metric value and cleanup after the cycle", func() {
				metricValue1 := float64(2)
				metricValue2 := float64(6.4)
				metricMeasure := &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureID:   testMetric,
					MeasureType: health.Metric,
					MetricValue: metricValue1,
				}
				mesuresCh <- metricMeasure
				metricMeasure = &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureID:   testMetric,
					MeasureType: health.Metric,
					MetricValue: metricValue2,
				}
				mesuresCh <- metricMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Metrics[testMetric]).Should(Equal((metricValue1 + metricValue2) / 2))

				actualHealth = <-healthCh
				Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
			})

			It("should forward message that affects health and cleanup after the cycle", func() {
				summary := "mock"
				msg := &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureType: health.Message,
					Message: &component.Message{
						Summary:      summary,
						AffectHealth: true,
						HealthStatus: component.Unhealthy,
					},
				}
				mesuresCh <- msg

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Status).Should(Equal(component.Unhealthy))
				actualMsg := actualHealth.Messages[0]
				Ω(actualMsg.HealthStatus).Should(Equal(component.Unhealthy))
				Ω(actualMsg.Summary).Should(Equal(summary))

				actualHealth = <-healthCh
				Ω(len(actualHealth.Messages)).Should(Equal(0))
				Ω(actualHealth.Status).Should(Equal(component.Unhealthy))
			})

			It("should forward message that don't affect health", func() {
				summary := "mock"
				msg := &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureType: health.Message,
					Message: &component.Message{
						Summary:      summary,
						AffectHealth: false,
						HealthStatus: component.Unhealthy,
					},
				}
				mesuresCh <- msg

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Status).Should(Equal(component.Healthy))
				Ω(actualHealth.Messages[0].Summary).Should(Equal(summary))
			})

			It("should change health status if it is called manually and shouldn't cleanup after", func() {
				hStatus := &health.ComponentMeasurement{
					ComponentID:  testComponentID,
					MeasureType:  health.HealthStatus,
					HealthStatus: component.Degrade,
				}
				mesuresCh <- hStatus

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Status).Should(Equal(component.Degrade))

				actualHealth = <-healthCh
				Ω(actualHealth.Status).Should(Equal(component.Degrade))
			})
		})

		Context("missing component or its component", func() {
			BeforeEach(func() {
				manager.Start(ctx, mesuresCh, healthCh, componentCh)
			})

			It("the first push shouldn't affect anything", func() {
				counterValue := int32(2)
				counterMeasure := &health.ComponentMeasurement{
					ComponentID:   testComponentID,
					MeasureType:   health.CounterChange,
					MeasureID:     testTotalCounter,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure
				time.Sleep(200 * time.Microsecond)

				go func() {
					testComponent := <-componentCh
					Ω(testComponent.ID).Should(Equal(testComponentID))
				}()
				addTestComponent()

				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue))
			})

			It("the counter with unregistered id shouldn't affect anything", func() {
				go func() {
					testComponent := <-componentCh
					Ω(testComponent.ID).Should(Equal(testComponentID))
				}()
				addTestComponent()
				fakeID := "fakerID"

				counterValue := int32(2)
				counterMeasure := &health.ComponentMeasurement{
					ComponentID:   testComponentID,
					MeasureType:   health.CounterChange,
					MeasureID:     fakeID,
					CounterChange: counterValue,
				}
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(int32(0)))
				_, ok := actualHealth.Counters[fakeID]
				Ω(ok).Should(BeFalse())
			})

			It("the metric with unregistered id shouldn't affect anything", func() {
				go func() {
					testComponent := <-componentCh
					Ω(testComponent.ID).Should(Equal(testComponentID))
				}()
				addTestComponent()
				fakeID := "fakerID"

				counterValue := float64(2)
				counterMeasure := &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureType: health.Metric,
					MeasureID:   fakeID,
					MetricValue: counterValue,
				}
				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
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

			mesuresCh = make(chan *health.ComponentMeasurement)
			healthCh = make(chan *component.Health)
			componentCh = make(chan *component.Component)

			manager.Start(ctx, mesuresCh, healthCh, componentCh)
		})

		AfterEach(func() {
			go func() {
				<-healthCh
			}()
			close(mesuresCh)
			manager.Shutdown()
		})

		It("should add the component dynamicaly by heartbeat", func() {
			heartbeatMeasure := &health.ComponentMeasurement{
				ComponentID: testComponentID,
				MeasureType: health.Heartbeat,
			}
			mesuresCh <- heartbeatMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Heartbeat.Beats).Should(BeTrue())
			Ω(actualHealth.Heartbeat.Enabled).Should(BeTrue())
		})

		It("should add the component dynamicaly by metric", func() {
			metricValue := float64(6.4)
			metricMeasure := &health.ComponentMeasurement{
				ComponentID: testComponentID,
				MeasureID:   testMetric,
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Metrics[testMetric]).Should(Equal(metricValue))
		})

		It("should add the component dynamicaly by counter", func() {
			counterValue := int32(2)
			counterMeasure := &health.ComponentMeasurement{
				ComponentID:   testComponentID,
				MeasureID:     testCounter,
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}
			mesuresCh <- counterMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Counters[testCounter]).Should(Equal(counterValue))
		})

		It("shouldn't add the counter if id is not unique per component", func() {
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent()
			counterValue := int32(2)
			counterMeasure := &health.ComponentMeasurement{
				ComponentID:   testComponentID,
				MeasureID:     testMetric, // here we use metric id but its already taken
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}
			mesuresCh <- counterMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Counters[testMetric]).Should(Equal(int32(0)))
			Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
		})

		It("should add the counter if id is unique per component", func() {
			newMeasureID := "mocked.id"
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent()
			counterValue := int32(3)
			counterMeasure := &health.ComponentMeasurement{
				ComponentID:   testComponentID,
				MeasureID:     newMeasureID,
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}
			mesuresCh <- counterMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))
			Ω(actualHealth.Counters[newMeasureID]).Should(Equal(counterValue))
		})

		It("shouldn't add the metric if id is not unique per component", func() {
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent()
			metricValue := float64(2)
			metricMeasure := &health.ComponentMeasurement{
				ComponentID: testComponentID,
				MeasureID:   testCounter, // here we use counter id but its already taken
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Metrics[testCounter]).Should(Equal(float64(0)))
			Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))
		})

		It("should add the metric if id is unique per component", func() {
			newMeasureID := "mocked.id"
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent()
			metricValue := float64(2.3)
			metricMeasure := &health.ComponentMeasurement{
				ComponentID: testComponentID,
				MeasureID:   newMeasureID,
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
			Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))
			Ω(actualHealth.Metrics[newMeasureID]).Should(Equal(metricValue))
		})
	})

	Context("dynamic config update", func() {
		var (
			unknownComponentID = "unknown.component"
			unknownMetric      = "unknown.metric"

			hbCancel context.CancelFunc
			err      error
		)

		BeforeEach(func() {
			ctx = context.Background()

			config := health.NewConfig()
			config.CollectionCycle = 200 * time.Millisecond
			config.RegistrationOnCollect = false
			config.LogLevel = "fatal"
			manager = health.NewManager(ctx, config)

			mesuresCh = make(chan *health.ComponentMeasurement)
			healthCh = make(chan *component.Health)
			componentCh = make(chan *component.Component)
			manager.Start(ctx, mesuresCh, healthCh, componentCh)

			collector := health.NewCollector(config.CollectionCycle, mesuresCh)
			health.SetCollectorSingleton(collector)

			hbCancel, err = collector.HeartBeat(unknownComponentID)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			health.StopCollectorSingleton()
			manager.Shutdown()
			close(mesuresCh)
		})

		It("should fail to update with nil config", func() {
			err := manager.UpdateConfig(nil)
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("config should not be nil"))
		})

		It("should fail to update with 0 cycle duration", func() {
			newCfg := health.NewConfig()
			newCfg.CollectionCycle = 0

			err := manager.UpdateConfig(newCfg)
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("collection cycle must be positive"))
		})

		It("should fail to update with dead collector", func() {
			hbCancel()
			health.ResetCollectorSingleton()

			newCfg := health.NewConfig()
			newCfg.CollectionCycle = 300 * time.Millisecond

			err := manager.UpdateConfig(newCfg)
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(ContainSubstring("collector is not running"))
		})

		It("should not send health data for unregistered component first", func() {
			metricValue := float64(2)
			counterMeasure := &health.ComponentMeasurement{
				ComponentID: unknownComponentID,
				MeasureType: health.Metric,
				MeasureID:   unknownMetric,
				MetricValue: metricValue,
			}
			mesuresCh <- counterMeasure

			Consistently(func() bool {
				select {
				case <-healthCh:
					return true
				default:
					return false // no health message received
				}
			}).Should(BeFalse())
		})

		It("should add the component dynamically by metric after config update", func() {
			newCfg := health.NewConfig()
			newCfg.CollectionCycle = 400 * time.Millisecond
			newCfg.RegistrationOnCollect = true
			newCfg.LogLevel = "info"
			err := manager.UpdateConfig(newCfg)
			Ω(err).ShouldNot(HaveOccurred())

			metricValue := float64(6.4)
			metricMeasure := &health.ComponentMeasurement{
				ComponentID: unknownComponentID,
				MeasureType: health.Metric,
				MeasureID:   unknownMetric,
				MetricValue: metricValue,
			}
			mesuresCh <- metricMeasure

			actualHealth := <-healthCh
			defer func() {
				go func() {
					<-healthCh
				}()
			}()

			Ω(actualHealth).ShouldNot(BeNil())
			Ω(actualHealth.ComponentID).Should(Equal(unknownComponentID))
			Ω(actualHealth.Metrics[unknownMetric]).Should(Equal(metricValue))
		})
	})
})
