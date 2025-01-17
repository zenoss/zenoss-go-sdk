package health_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var _ = Describe("Health Manager", Ordered, func() {
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

	addTestComponent := func(target string) {
		comp, _ := component.New(
			testComponentID, utils.DefaultComponentType, target, true,
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
		verifyTargetHealth := func(expStatus component.HealthStatus, heartbeat, beats bool) {
			targetHealth := <-healthCh
			Ω(targetHealth).ShouldNot(BeNil())
			Ω(targetHealth.ComponentID).Should(Equal(utils.DefaultHealthTarget))

			switch expStatus {
			case component.Healthy:
				Ω(targetHealth.Status).Should(Equal(component.Healthy))
				Ω(len(targetHealth.Messages)).Should(Equal(0))
			case component.Degrade:
				Ω(targetHealth.Status).Should(Equal(component.Degrade))
				Ω(len(targetHealth.Messages)).Should(Equal(1))
				Ω(targetHealth.Messages[0].HealthStatus).Should(Equal(component.Degrade))
				Ω(targetHealth.Messages[0].Summary).Should(Equal(fmt.Sprintf("%s degraded", testComponentID)))
			case component.Unhealthy:
				Ω(targetHealth.Status).Should(Equal(component.Unhealthy))
				Ω(len(targetHealth.Messages)).Should(Equal(1))
				Ω(targetHealth.Messages[0].HealthStatus).Should(Equal(component.Unhealthy))
				Ω(targetHealth.Messages[0].Summary).Should(Equal(fmt.Sprintf("%s unhealthy", testComponentID)))
			}

			Ω(targetHealth.Heartbeat.Enabled).Should(Equal(heartbeat))
			Ω(targetHealth.Heartbeat.Beats).Should(Equal(beats))
		}

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
				<-healthCh // component health
				<-healthCh // general (target) health
				close(controller)
			}()
			close(mesuresCh)
			manager.Shutdown()
			<-controller
		})

		Context("adding components", func() {
			It("should fail if components limit is exceeded", func() {
				components := make([]*component.Component, utils.ComponentsLimit/2+1)
				for i := 0; i < len(components); i++ {
					components[i] = &component.Component{
						ID:       fmt.Sprintf("component-%d", i),
						TargetID: fmt.Sprintf("targetComponent-%d", i),
					}
				}
				manager.Start(ctx, mesuresCh, healthCh, componentCh)
				err := manager.AddComponents(components)
				Ω(err).Should(Equal(utils.ErrComponentsLimitExceeded))
			})

			It("should add taret and push it on start", func() {
				controller := make(chan struct{})
				go func() {
					c1 := <-componentCh
					Ω(c1).ShouldNot(BeNil())
					c2 := <-componentCh
					Ω(c2).ShouldNot(BeNil())

					if actualComponent := c1; actualComponent.ID == testComponentID {
						targetComponent := c2
						Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))
					} else {
						actualComponent, targetComponent := c2, c1
						Ω(actualComponent.ID).Should(Equal(testComponentID))
						Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))
					}
					close(controller)
				}()
				addTestComponent(utils.DefaultHealthTarget)

				manager.Start(ctx, mesuresCh, healthCh, componentCh)
				<-controller
			})

			It("should push component on add component even if started", func() {
				manager.Start(ctx, mesuresCh, healthCh, componentCh)

				controller := make(chan struct{})
				go func() {
					actualComponent := <-componentCh
					Ω(actualComponent).ShouldNot(BeNil())
					Ω(actualComponent.ID).Should(Equal(testComponentID))

					targetComponent := <-componentCh
					Ω(targetComponent).ShouldNot(BeNil())
					Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))

					close(controller)
				}()
				addTestComponent(utils.DefaultHealthTarget)
				<-controller
			})
		})

		Context("deleting components", func() {
			It("should delete specified components and affected target", func() {
				var (
					compToDeleteID0  = "component.to.delete-0"
					compToDeleteID1  = "component.to.delete-1"
					compToRetainID   = "component.to.retain"
					targetToDeleteID = "target.component.to.delete"
					targetToRetainID = "target.component.to.retain"

					h *component.Health
				)

				initComponents := []*component.Component{
					{
						ID:       compToDeleteID0,
						Type:     utils.DefaultComponentType,
						TargetID: targetToDeleteID,
					},
					{
						ID:       compToDeleteID1,
						Type:     utils.DefaultComponentType,
						TargetID: targetToRetainID,
					},
					{
						ID:       compToRetainID,
						Type:     utils.DefaultComponentType,
						TargetID: targetToRetainID,
					},
				}
				manager.Start(ctx, mesuresCh, healthCh, componentCh)
				controller := make(chan struct{})
				go func() {
					for range 5 {
						c := <-componentCh
						Ω(c).ShouldNot(BeNil())
					}
					close(controller)
				}()
				err := manager.AddComponents(initComponents)
				Ω(err).ShouldNot(HaveOccurred())
				<-controller

				uniqueComponentsHealth := make(map[string]*component.Health, 5)
				for range 5 {
					h = <-healthCh
					Ω(h).ShouldNot(BeNil())
					uniqueComponentsHealth[h.ComponentID] = h
				}
				Ω(len(uniqueComponentsHealth)).Should(Equal(5))

				manager.DeleteComponents([]string{compToDeleteID0, compToDeleteID1})
				uniqueComponentsHealth = make(map[string]*component.Health, 2)
				for range 10 {
					h = <-healthCh
					Ω(h).ShouldNot(BeNil())
					uniqueComponentsHealth[h.ComponentID] = h
				}
				Ω(len(uniqueComponentsHealth)).Should(Equal(2))
				Ω(uniqueComponentsHealth[compToRetainID]).ShouldNot(BeNil())
				Ω(uniqueComponentsHealth[targetToRetainID]).ShouldNot(BeNil())
			})
		})

		Context("simple health measurements", func() {
			BeforeEach(func() {
				controller := make(chan struct{})
				go func() {
					testComponent := <-componentCh
					Ω(testComponent.ID).Should(Equal(testComponentID))

					targetComponent := <-componentCh
					Ω(targetComponent).ShouldNot(BeNil())
					Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))

					close(controller)
				}()
				manager.Start(ctx, mesuresCh, healthCh, componentCh)
				addTestComponent(utils.DefaultHealthTarget)
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

				verifyTargetHealth(component.Healthy, true, true)
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

				verifyTargetHealth(component.Healthy, true, false)

				actualHealth = <-healthCh
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Counters[testCounter]).Should(Equal(int32(0)))

				verifyTargetHealth(component.Healthy, true, false)
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

				verifyTargetHealth(component.Healthy, true, false)

				actualHealth = <-healthCh
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue * 2))

				verifyTargetHealth(component.Healthy, true, false)
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

				verifyTargetHealth(component.Healthy, true, false)

				actualHealth = <-healthCh
				Ω(actualHealth.Metrics[testMetric]).Should(Equal(float64(0)))

				verifyTargetHealth(component.Healthy, true, false)
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

				verifyTargetHealth(component.Unhealthy, true, false)

				actualHealth = <-healthCh
				Ω(len(actualHealth.Messages)).Should(Equal(0))
				Ω(actualHealth.Status).Should(Equal(component.Unhealthy))

				verifyTargetHealth(component.Unhealthy, true, false)
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

				verifyTargetHealth(component.Healthy, true, false)
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

				verifyTargetHealth(component.Degrade, true, false)

				actualHealth = <-healthCh
				Ω(actualHealth.Status).Should(Equal(component.Degrade))

				verifyTargetHealth(component.Degrade, true, false)
			})
		})

		Context("missing component or its measure", func() {
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
					Ω(testComponent).ShouldNot(BeNil())
					Ω(testComponent.ID).Should(Equal(testComponentID))

					targetComponent := <-componentCh
					Ω(targetComponent).ShouldNot(BeNil())
					Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))
				}()
				addTestComponent(utils.DefaultHealthTarget)

				mesuresCh <- counterMeasure

				actualHealth := <-healthCh
				Ω(actualHealth).ShouldNot(BeNil())
				Ω(actualHealth.ComponentID).Should(Equal(testComponentID))
				Ω(actualHealth.Counters[testTotalCounter]).Should(Equal(counterValue))

				verifyTargetHealth(component.Healthy, true, false)
			})

			It("the counter with unregistered id shouldn't affect anything", func() {
				go func() {
					testComponent := <-componentCh
					Ω(testComponent).ShouldNot(BeNil())
					Ω(testComponent.ID).Should(Equal(testComponentID))

					targetComponent := <-componentCh
					Ω(targetComponent).ShouldNot(BeNil())
					Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))
				}()
				addTestComponent(utils.DefaultHealthTarget)
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

				verifyTargetHealth(component.Healthy, true, false)
			})

			It("the metric with unregistered id shouldn't affect anything", func() {
				go func() {
					testComponent := <-componentCh
					Ω(testComponent).ShouldNot(BeNil())
					Ω(testComponent.ID).Should(Equal(testComponentID))

					targetComponent := <-componentCh
					Ω(targetComponent).ShouldNot(BeNil())
					Ω(targetComponent.ID).Should(Equal(utils.DefaultHealthTarget))
				}()
				addTestComponent(utils.DefaultHealthTarget)
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

				verifyTargetHealth(component.Healthy, true, false)
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
			if manager.IsStarted() {
				go func() {
					<-healthCh
				}()
				close(mesuresCh)
				manager.Shutdown()
			}
		})

		It("should add the component dynamically by heartbeat", func() {
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

		It("should add the component dynamically by metric", func() {
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

		It("should add the component dynamically by counter", func() {
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

		It("shouldn't add the component if the limit is exceeded", func() {
			go func() {
				for i := 0; i < utils.ComponentsLimit; i++ {
					testComponent := <-componentCh
					Ω(testComponent).ShouldNot(BeNil())
					Ω(testComponent.ID).ShouldNot(BeEmpty())
				}
			}()
			components := make([]*component.Component, utils.ComponentsLimit)
			for i := 0; i < utils.ComponentsLimit; i++ {
				components[i] = &component.Component{
					ID:         fmt.Sprintf("component-%d", i),
					CounterIDs: []string{fmt.Sprintf("%s-%d", testCounter, i)},
				}
			}
			err := manager.AddComponents(components)
			Ω(err).ShouldNot(HaveOccurred())

			mesuresCh <- &health.ComponentMeasurement{
				ComponentID:   testComponentID,
				MeasureID:     testCounter,
				MeasureType:   health.CounterChange,
				CounterChange: int32(2),
			}

			var actualHealth *component.Health
			uniqueComponents := make(map[string]struct{}, utils.ComponentsLimit)
			for i := 0; i < utils.ComponentsLimit; i++ {
				actualHealth = <-healthCh
				uniqueComponents[actualHealth.ComponentID] = struct{}{}
			}
			Ω(len(uniqueComponents)).Should(Equal(utils.ComponentsLimit))
			_, testComponentAdded := uniqueComponents[testComponentID]
			Ω(testComponentAdded).Should(BeFalse())

			go func() {
				for i := 0; i < utils.ComponentsLimit; i++ {
					<-healthCh
				}
			}()
			close(mesuresCh)
			manager.Shutdown()
		})

		It("shouldn't add the counter if id is not unique per component", func() {
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent("")
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

		It("shouldn't add the counter if measures limit is exceeded", func() {
			go func() {
				testComponent := <-componentCh
				Ω(testComponent).ShouldNot(BeNil())
				Ω(testComponent.ID).ShouldNot(BeEmpty())
			}()
			counterIDs := make([]string, utils.ComponentMeasuresLimit)
			for i := 0; i < len(counterIDs); i++ {
				counterIDs[i] = fmt.Sprintf("%s-%d", testCounter, i)
			}
			components := []*component.Component{{
				ID:         testComponentID,
				CounterIDs: counterIDs,
			}}
			err := manager.AddComponents(components)
			Ω(err).ShouldNot(HaveOccurred())

			counterValue := int32(2)
			for i := 0; i < utils.ComponentMeasuresLimit; i++ {
				mesuresCh <- &health.ComponentMeasurement{
					ComponentID:   testComponentID,
					MeasureID:     fmt.Sprintf("%s-%d", testCounter, i),
					MeasureType:   health.CounterChange,
					CounterChange: counterValue,
				}
			}

			mesuresCh <- &health.ComponentMeasurement{
				ComponentID:   testComponentID,
				MeasureID:     fmt.Sprintf("%s-%d", testCounter, utils.ComponentMeasuresLimit),
				MeasureType:   health.CounterChange,
				CounterChange: counterValue,
			}

			actualHealth := <-healthCh
			Ω(len(actualHealth.Counters)).Should(Equal(utils.ComponentMeasuresLimit))
			Ω(actualHealth.Counters[fmt.Sprintf("%s-%d", testCounter, utils.ComponentMeasuresLimit-1)]).Should(Equal(counterValue))
			Ω(actualHealth.Counters[fmt.Sprintf("%s-%d", testCounter, utils.ComponentMeasuresLimit)]).Should(Equal(int32(0)))
		})

		It("should add the counter if id is unique per component", func() {
			newMeasureID := "mocked.id"
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent("")
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
			addTestComponent("")
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

		It("shouldn't add the metric if measures limit is exceeded", func() {
			go func() {
				testComponent := <-componentCh
				Ω(testComponent).ShouldNot(BeNil())
				Ω(testComponent.ID).ShouldNot(BeEmpty())
			}()
			counterIDs := make([]string, utils.ComponentMeasuresLimit/2)
			metricIDs := make([]string, utils.ComponentMeasuresLimit/2)
			for i := 0; i < utils.ComponentMeasuresLimit/2; i++ {
				counterIDs[i] = fmt.Sprintf("%s-%d", testCounter, i)
				metricIDs[i] = fmt.Sprintf("%s-%d", testMetric, i)
			}
			components := []*component.Component{{
				ID:         testComponentID,
				CounterIDs: counterIDs,
				MetricIDs:  metricIDs,
			}}
			err := manager.AddComponents(components)
			Ω(err).ShouldNot(HaveOccurred())

			counterValue := int32(2)
			metricValue := float64(2.3)
			for i := 0; i < utils.ComponentMeasuresLimit/2; i++ {
				mesuresCh <- &health.ComponentMeasurement{
					ComponentID:   testComponentID,
					MeasureID:     fmt.Sprintf("%s-%d", testCounter, i),
					MeasureType:   health.CounterChange,
					CounterChange: counterValue,
				}

				mesuresCh <- &health.ComponentMeasurement{
					ComponentID: testComponentID,
					MeasureID:   fmt.Sprintf("%s-%d", testMetric, i),
					MeasureType: health.Metric,
					MetricValue: metricValue,
				}
			}

			mesuresCh <- &health.ComponentMeasurement{
				ComponentID: testComponentID,
				MeasureID:   fmt.Sprintf("%s-%d", testMetric, utils.ComponentMeasuresLimit),
				MeasureType: health.Metric,
				MetricValue: metricValue,
			}

			actualHealth := <-healthCh
			Ω(len(actualHealth.Counters)).Should(Equal(utils.ComponentMeasuresLimit / 2))
			Ω(len(actualHealth.Metrics)).Should(Equal(utils.ComponentMeasuresLimit / 2))
			Ω(actualHealth.Counters[fmt.Sprintf("%s-%d", testCounter, utils.ComponentMeasuresLimit/2-1)]).Should(Equal(counterValue))
			Ω(actualHealth.Metrics[fmt.Sprintf("%s-%d", testMetric, utils.ComponentMeasuresLimit/2-1)]).Should(Equal(metricValue))
			Ω(actualHealth.Metrics[fmt.Sprintf("%s-%d", testMetric, utils.ComponentMeasuresLimit)]).Should(Equal(float64(0)))
		})

		It("should add the metric if id is unique per component", func() {
			newMeasureID := "mocked.id"
			go func() {
				testComponent := <-componentCh
				Ω(testComponent.ID).Should(Equal(testComponentID))
			}()
			addTestComponent("")
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

	Context("hierarchical components impact", func() {
		BeforeEach(func() {
			ctx = context.Background()

			config := health.NewConfig()
			config.CollectionCycle = 1000 * time.Millisecond
			config.LogLevel = "fatal"
			manager = health.NewManager(ctx, config)

			mesuresCh = make(chan *health.ComponentMeasurement)
			healthCh = make(chan *component.Health)
			componentCh = make(chan *component.Component)
		})

		AfterEach(func() {
			controller := make(chan struct{})
			go func() {
				for range 8 {
					<-healthCh
				}
				close(controller)
			}()
			close(mesuresCh)
			manager.Shutdown()
			<-controller
		})

		It("should produce correct health results for targets from lower level components impact", func() {
			//                target.high -> (U+HB)
			//                ^         ^
			//          (D+HB)^      (U)^
			//                ^         ^
			//      target.mid0         target.mid1
			//        ^     ^           ^     ^    ^
			//  (H+HB)^  (D)^        (H)^  (U)^    ^(H)
			//        ^     ^           ^     ^    ^
			//       low0  low1        low2  low3  low4
			//

			mid0, _ := component.New(
				"target.mid0", "mid", "target.high", true,
				nil, []string{testCounter}, nil,
			)
			mid1, _ := component.New(
				"target.mid1", "mid", "target.high", false,
				nil, nil, nil,
			)

			low0, _ := component.New(
				"low0", "low", mid0.ID, true,
				nil, nil, nil,
			)
			low1, _ := component.New(
				"low1", "low", mid0.ID, false,
				nil, nil, nil,
			)
			low2, _ := component.New(
				"low2", "low", mid1.ID, false,
				[]string{testMetric}, nil, nil,
			)
			low3, _ := component.New(
				"low3", "low", mid1.ID, false,
				nil, nil, nil,
			)
			low4, _ := component.New(
				"low4", "low", mid1.ID, false,
				nil, []string{testCounter}, nil,
			)

			manager.Start(ctx, mesuresCh, healthCh, componentCh)
			controller := make(chan struct{})
			go func() {
				m0 := <-componentCh
				Ω(m0).ShouldNot(BeNil())
				Ω(m0.ID).Should(Equal(mid0.ID))

				m1 := <-componentCh
				Ω(m1).ShouldNot(BeNil())
				Ω(m1.ID).Should(Equal(mid1.ID))

				l0 := <-componentCh
				Ω(l0).ShouldNot(BeNil())
				Ω(l0.ID).Should(Equal(low0.ID))

				l1 := <-componentCh
				Ω(l1).ShouldNot(BeNil())
				Ω(l1.ID).Should(Equal(low1.ID))

				l2 := <-componentCh
				Ω(l2).ShouldNot(BeNil())
				Ω(l2.ID).Should(Equal(low2.ID))

				l3 := <-componentCh
				Ω(l3).ShouldNot(BeNil())
				Ω(l3.ID).Should(Equal(low3.ID))

				l4 := <-componentCh
				Ω(l4).ShouldNot(BeNil())
				Ω(l4.ID).Should(Equal(low4.ID))

				h := <-componentCh
				Ω(h).ShouldNot(BeNil())
				Ω(h.ID).Should(Equal("target.high"))

				close(controller)
			}()
			components := []*component.Component{mid0, mid1, low0, low1, low2, low3, low4}
			manager.AddComponents(components)
			<-controller

			// low0
			heartbeatMeasure := &health.ComponentMeasurement{
				ComponentID: low0.ID,
				MeasureType: health.Heartbeat,
			}
			mesuresCh <- heartbeatMeasure

			// low1
			hStatus := &health.ComponentMeasurement{
				ComponentID:  low1.ID,
				MeasureType:  health.HealthStatus,
				HealthStatus: component.Degrade,
			}
			mesuresCh <- hStatus

			// low2
			low2metricValue := float64(6.4)
			metricMeasure := &health.ComponentMeasurement{
				ComponentID: low2.ID,
				MeasureID:   testMetric,
				MeasureType: health.Metric,
				MetricValue: low2metricValue,
			}
			mesuresCh <- metricMeasure

			// low3
			msg := &health.ComponentMeasurement{
				ComponentID: low3.ID,
				MeasureType: health.Message,
				Message: &component.Message{
					Summary:      "custom message low3",
					AffectHealth: true,
					HealthStatus: component.Unhealthy,
				},
			}
			mesuresCh <- msg

			// low4
			low4counterValue := int32(1)
			counterMeasure := &health.ComponentMeasurement{
				ComponentID:   low4.ID,
				MeasureID:     testCounter,
				MeasureType:   health.CounterChange,
				CounterChange: low4counterValue,
			}
			mesuresCh <- counterMeasure

			// target.mid0
			mid0counterValue := int32(2)
			counterMeasure = &health.ComponentMeasurement{
				ComponentID:   mid0.ID,
				MeasureID:     testCounter,
				MeasureType:   health.CounterChange,
				CounterChange: mid0counterValue,
			}
			mesuresCh <- counterMeasure

			// target.mid1
			msg = &health.ComponentMeasurement{
				ComponentID: mid1.ID,
				MeasureType: health.Message,
				Message: &component.Message{
					Summary:      "custom message mid1",
					HealthStatus: component.Healthy,
				},
			}
			mesuresCh <- msg

			lowComponentsHealth := map[string]*component.Health{}
			for range 5 {
				h := <-healthCh
				lowComponentsHealth[h.ComponentID] = h
			}

			targetComponentsHealth := map[string]*component.Health{}
			for range 3 {
				h := <-healthCh
				targetComponentsHealth[h.ComponentID] = h
			}

			Ω(lowComponentsHealth["low0"]).ShouldNot(BeNil())
			Ω(lowComponentsHealth["low0"].Status).Should(Equal(component.Healthy))
			Ω(lowComponentsHealth["low0"].Heartbeat.Enabled).Should(BeTrue())
			Ω(lowComponentsHealth["low0"].Heartbeat.Beats).Should(BeTrue())

			Ω(lowComponentsHealth["low1"]).ShouldNot(BeNil())
			Ω(lowComponentsHealth["low1"].Status).Should(Equal(component.Degrade))

			Ω(lowComponentsHealth["low2"]).ShouldNot(BeNil())
			Ω(lowComponentsHealth["low2"].Status).Should(Equal(component.Healthy))
			Ω(lowComponentsHealth["low2"].Metrics[testMetric]).Should(Equal(low2metricValue))

			Ω(lowComponentsHealth["low3"]).ShouldNot(BeNil())
			Ω(lowComponentsHealth["low3"].Status).Should(Equal(component.Unhealthy))
			Ω(lowComponentsHealth["low3"].Messages).Should(ContainElement(&component.Message{
				Summary:      "custom message low3",
				AffectHealth: true,
				HealthStatus: component.Unhealthy,
			}))
			Ω(len(lowComponentsHealth["low3"].Messages)).Should(Equal(1))

			Ω(lowComponentsHealth["low4"]).ShouldNot(BeNil())
			Ω(lowComponentsHealth["low4"].Status).Should(Equal(component.Healthy))
			Ω(lowComponentsHealth["low4"].Counters[testCounter]).Should(Equal(low4counterValue))

			Ω(targetComponentsHealth["target.mid0"]).ShouldNot(BeNil())
			Ω(targetComponentsHealth["target.mid0"].Status).Should(Equal(component.Degrade))
			Ω(targetComponentsHealth["target.mid0"].Heartbeat.Enabled).Should(BeTrue())
			Ω(targetComponentsHealth["target.mid0"].Heartbeat.Beats).Should(BeTrue())
			Ω(targetComponentsHealth["target.mid0"].Counters[testCounter]).Should(Equal(mid0counterValue))
			Ω(targetComponentsHealth["target.mid0"].Messages).Should(ContainElement(&component.Message{
				Summary:      "low1 degraded",
				AffectHealth: true,
				HealthStatus: component.Degrade,
			}))
			Ω(len(targetComponentsHealth["target.mid0"].Messages)).Should(Equal(1))

			Ω(targetComponentsHealth["target.mid1"]).ShouldNot(BeNil())
			Ω(targetComponentsHealth["target.mid1"].Status).Should(Equal(component.Unhealthy))
			Ω(targetComponentsHealth["target.mid1"].Heartbeat.Enabled).Should(BeFalse())
			Ω(targetComponentsHealth["target.mid1"].Heartbeat.Beats).Should(BeFalse())
			Ω(targetComponentsHealth["target.mid1"].Messages).Should(ContainElement(&component.Message{
				Summary:      "custom message mid1",
				HealthStatus: component.Healthy,
			}))
			Ω(targetComponentsHealth["target.mid1"].Messages).Should(ContainElement(&component.Message{
				Summary:      "low3 unhealthy",
				AffectHealth: true,
				HealthStatus: component.Unhealthy,
			}))
			Ω(len(targetComponentsHealth["target.mid1"].Messages)).Should(Equal(2))

			Ω(targetComponentsHealth["target.high"]).ShouldNot(BeNil())
			Ω(targetComponentsHealth["target.high"].Status).Should(Equal(component.Unhealthy))
			Ω(targetComponentsHealth["target.high"].Heartbeat.Enabled).Should(BeTrue())
			Ω(targetComponentsHealth["target.high"].Heartbeat.Beats).Should(BeTrue())
			Ω(targetComponentsHealth["target.high"].Messages).Should(ContainElement(&component.Message{
				Summary:      "target.mid0 degraded",
				AffectHealth: true,
				HealthStatus: component.Degrade,
			}))
			Ω(targetComponentsHealth["target.high"].Messages).Should(ContainElement(&component.Message{
				Summary:      "target.mid1 unhealthy",
				AffectHealth: true,
				HealthStatus: component.Unhealthy,
			}))
			Ω(len(targetComponentsHealth["target.high"].Messages)).Should(Equal(2))
		})
	})
})
