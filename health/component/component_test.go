package component_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

func TestHealthComponent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Component Suite")
}

var _ = Describe("Component Tests", func() {
	const (
		mockStr = "mocked"
		id      = "componentID"
	)

	Context("constructor", func() {
		var metricIDs, counterIDs, tCounterIDs []string
		BeforeEach(func() {
			metricIDs = []string{"someMetric"}
			counterIDs = []string{"someCounter"}
			tCounterIDs = []string{"totalCounter", mockStr}
		})

		It("should return an error if measure is not unique", func() {
			metricIDs = append(metricIDs, mockStr)
			component, err := component.New(id, "", "", true, metricIDs, counterIDs, tCounterIDs)
			Ω(component).Should(BeNil())
			Ω(err).Should(Equal(utils.ErrMeasureIDTaken))
		})

		It("should return an error if measures limit is exceeded", func() {
			for i := 0; i < utils.ComponentMeasuresLimit/3+1; i++ {
				metricIDs = append(metricIDs, fmt.Sprintf("metric-%d", i))
				counterIDs = append(counterIDs, fmt.Sprintf("counter-%d", i))
				tCounterIDs = append(tCounterIDs, fmt.Sprintf("totalCounter-%d", i))
			}
			component, err := component.New(id, "", "", true, metricIDs, counterIDs, tCounterIDs)
			Ω(component).Should(BeNil())
			Ω(err).Should(Equal(utils.ErrComponentMeasuresLimitExceeded))
		})

		It("should return a new Component", func() {
			component, err := component.New(id, "", "", true, metricIDs, counterIDs, tCounterIDs)
			Ω(err).Should(BeNil())
			Ω(component).ShouldNot(BeNil())
		})
	})

	Context("IsMeasureIDUnique method", func() {
		var testComponent *component.Component

		BeforeEach(func() {
			metricIDs := []string{"someMetric"}
			counterIDs := []string{"someCounter"}
			tCounterIDs := []string{"totalCounter"}

			testComponent, _ = component.New(id, "", "", true, metricIDs, counterIDs, tCounterIDs)
		})

		It("should return false if measure is not unique", func() {
			testComponent.CounterIDs = append(testComponent.CounterIDs, mockStr)
			unique := testComponent.IsMeasureIDUnique(mockStr)
			Ω(unique).Should(BeFalse())
		})

		It("should return true if measure is unique", func() {
			unique := testComponent.IsMeasureIDUnique(mockStr)
			Ω(unique).Should(BeTrue())
		})
	})
})
