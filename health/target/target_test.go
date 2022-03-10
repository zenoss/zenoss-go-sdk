package target_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

func TestHealthTarget(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	RunSpecs(t, "Health Target Suite")
}

var _ = Describe("Target Tests", func() {
	const (
		mockStr = "mocked"
		id      = "targetID"
	)

	Context("constructor", func() {
		var (
			metricIDs, counterIDs, tCounterIDs []string
		)
		BeforeEach(func() {
			metricIDs = []string{"someMetric"}
			counterIDs = []string{"someCounter"}
			tCounterIDs = []string{"totalCounter", mockStr}
		})
		It("should return an error if measure is not unique", func() {
			metricIDs = append(metricIDs, mockStr)
			target, err := target.New(id, "", true, metricIDs, counterIDs, tCounterIDs)
			Ω(target).Should(BeNil())
			Ω(err).Should(Equal(utils.ErrMeasureIDTaken))
		})

		It("should return a new Target", func() {
			target, err := target.New(id, "", true, metricIDs, counterIDs, tCounterIDs)
			Ω(err).Should(BeNil())
			Ω(target).ShouldNot(BeNil())
		})
	})

	Context("IsMeasureIDUnique method", func() {
		var (
			testTarget *target.Target
		)

		BeforeEach(func() {
			metricIDs := []string{"someMetric"}
			counterIDs := []string{"someCounter"}
			tCounterIDs := []string{"totalCounter"}

			testTarget, _ = target.New(id, "", true, metricIDs, counterIDs, tCounterIDs)
		})

		It("should return false if measure is not unique", func() {
			testTarget.CounterIDs = append(testTarget.CounterIDs, mockStr)
			unique := testTarget.IsMeasureIDUnique(mockStr)
			Ω(unique).Should(BeFalse())
		})

		It("should return true if measure is unique", func() {
			unique := testTarget.IsMeasureIDUnique(mockStr)
			Ω(unique).Should(BeTrue())
		})
	})
})
