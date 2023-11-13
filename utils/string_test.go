package utils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/utils"
)

func TestStringUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "String Utils Suite")
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
