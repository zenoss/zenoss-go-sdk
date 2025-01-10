package component_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var _ = Describe("Component's Health Constructor", func() {
	id := "component.id"

	It("should return a new Health", func() {
		health := component.NewHealth(id, utils.DefaultComponentType, utils.DefaultHealthTarget)
		Ω(health).ShouldNot(BeNil())
	})
})

var _ = Describe("HealthStatus enum", func() {
	It("should return correct string value", func() {
		healthy, degrade, unhealthy := component.Healthy, component.Degrade, component.Unhealthy
		Ω(healthy.String()).Should(Equal(utils.HealthyStatus))
		Ω(degrade.String()).Should(Equal(utils.DegradeStatus))
		Ω(unhealthy.String()).Should(Equal(utils.UnhealthyStatus))
	})
})
