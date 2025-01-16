package component_test

import (
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
		var metrics map[string]component.Aggregator
		var counterIDs, tCounterIDs []string
		BeforeEach(func() {
			metrics = map[string]component.Aggregator{"someMetric": component.DefaultAggregator}
			counterIDs = []string{"someCounter"}
			tCounterIDs = []string{"totalCounter", mockStr}
		})
		It("should return an error if measure is not unique", func() {
			metrics[mockStr] = component.DefaultAggregator
			component, err := component.New(id, "", "", true, metrics, counterIDs, tCounterIDs)
			Ω(component).Should(BeNil())
			Ω(err).Should(Equal(utils.ErrMeasureIDTaken))
		})

		It("should return a new Component", func() {
			component, err := component.New(id, "", "", true, metrics, counterIDs, tCounterIDs)
			Ω(err).Should(BeNil())
			Ω(component).ShouldNot(BeNil())
		})
	})

	Context("IsMeasureIDUnique method", func() {
		var testComponent *component.Component

		BeforeEach(func() {
			metrics := map[string]component.Aggregator{"someMetric": component.DefaultAggregator}
			counterIDs := []string{"someCounter"}
			tCounterIDs := []string{"totalCounter"}

			testComponent, _ = component.New(id, "", "", true, metrics, counterIDs, tCounterIDs)
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
