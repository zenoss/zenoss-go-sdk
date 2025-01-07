package component_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health/component"
)

var _ = Describe("Message Constructor", func() {
	var (
		msgSummary = "Some summary"
		mockErr    = errors.New("you are mocked")
	)

	It("should return a new Message", func() {
		message := component.NewMessage(msgSummary, mockErr, true, component.Unhealthy)
		Ω(message).ShouldNot(BeNil())
	})
})
