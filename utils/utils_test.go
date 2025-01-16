package utils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/utils"
)

func TestStringUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

var _ = Describe("Sum", func() {
	It("should return sum of integers", func() {
		Ω(utils.Sum([]int{1, 2, 3, 4, 5})).Should(Equal(15))
	})

	It("should return sum of floats", func() {
		Ω(utils.Sum([]float64{1.1, 1.9, 2.8, 4.2, 5.0})).Should(Equal(float64(15)))
	})
})
