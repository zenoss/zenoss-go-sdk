package target_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

func TestHealthTarget(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Health Target Suite", []Reporter{junitReporter})
}

var _ = Describe("Target Constructor", func() {
	var (
		mockStr = "mocked"
		id      = "targetID"

		metricIDs, counterIDs, tCounterIDs []string
	)

	BeforeEach(func() {
		metricIDs = []string{"someMetric"}
		counterIDs = []string{"someCounter"}
		tCounterIDs = []string{"totalCounter", mockStr}
	})

	It("should return an error if measure is not unique", func() {
		metricIDs = append(metricIDs, mockStr)
		target, err := target.New(id, true, metricIDs, counterIDs, tCounterIDs)
		Ω(target).Should(BeNil())
		Ω(err).ShouldNot(BeNil()) // TODO: check for exact error
	})

	It("should return a new Target", func() {
		target, err := target.New(id, true, metricIDs, counterIDs, tCounterIDs)
		Ω(err).Should(BeNil())
		Ω(target).ShouldNot(BeNil())
	})
})

var _ = Describe("Target IsMeasureIDUnique method", func() {
	var (
		mockStr = "mocked"

		testTarget *target.Target
	)

	BeforeEach(func() {
		id := "targetID"
		metricIDs := []string{"someMetric"}
		counterIDs := []string{"someCounter"}
		tCounterIDs := []string{"totalCounter"}

		testTarget, _ = target.New(id, true, metricIDs, counterIDs, tCounterIDs)
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
