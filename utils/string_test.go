package utils_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/utils"
)

func TestStringUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "String Utils Suite", []Reporter{junitReporter})
}

var _ = Describe("ListContainsString", func() {
	It("string in list", func() {
		val := "string"
		strSlice := []string{"cool", "staff", val}
		exist := utils.ListContainsString(strSlice, val)
		Ω(exist).Should(BeTrue())
	})

	It("string not in list", func() {
		val := "string"
		strSlice := []string{"cool", "staff"}
		exist := utils.ListContainsString(strSlice, val)
		Ω(exist).Should(BeFalse())
	})
})
