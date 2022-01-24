package target_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var _ = Describe("Target's Health Constructor", func() {
	var (
		id = "target.id"
	)

	It("should return a new Health", func() {
		health := target.NewHealth(id, utils.DefaultTargetType)
		Ω(health).ShouldNot(BeNil())
	})
})

var _ = Describe("HealthStatus enum", func() {
	It("should return correct string value", func() {
		healthy, degrade, unhealthy := target.Healthy, target.Degrade, target.Unhealthy
		Ω(healthy.String()).Should(Equal(utils.HealthyStatus))
		Ω(degrade.String()).Should(Equal(utils.DegradeStatus))
		Ω(unhealthy.String()).Should(Equal(utils.UnhealthyStatus))
	})
})
