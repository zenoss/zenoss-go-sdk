package target_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

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
