package target_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Health Suite", []Reporter{junitReporter})
}

var _ = Describe("Target's Health Constructor", func() {
	var (
		id = "target.id"
	)

	It("should return a new Health", func() {
		health := target.NewHealth(id)
		Ω(health).ShouldNot(BeNil())
	})
})

var _ = Describe("HealthStatus enum", func() {
	It("should return correct string value", func() {
		healthy, degrade, unhealthy := target.Healthy, target.Degrade, target.Unhealthy
		Ω(healthy.String()).Should(Equal("Healthy"))
		Ω(degrade.String()).Should(Equal("Degrade"))
		Ω(unhealthy.String()).Should(Equal("Unhealthy"))
	})
})
