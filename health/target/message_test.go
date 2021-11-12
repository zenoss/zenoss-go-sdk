package target_test

import (
	"errors"
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

func TestHealthMessage(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Health Message Suite", []Reporter{junitReporter})
}

var _ = Describe("Message Constructor", func() {
	var (
		msgSummary = "Some summary"
		mockErr    = errors.New("you are mocked")
	)

	It("should return a new Message", func() {
		message := target.NewMessage(msgSummary, mockErr, true, target.Unhealthy)
		Ω(message).ShouldNot(BeNil())
	})
})
