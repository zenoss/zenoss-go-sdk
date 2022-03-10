package health_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/mocks"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	RunSpecs(t, "Health Framework Suite")
}

var _ = Describe("Health Framework", func() {
	var (
		ctx   context.Context
		cycle = 200 * time.Millisecond
	)

	// TODO: ZING-19127 add Serial decorator after ginkgo v2 upgrade (singleton staff should be run separately)
	It("framework should start and die", func() {
		ctx = context.Background()

		config := health.NewConfig()
		config.CollectionCycle = cycle
		config.LogLevel = "fatal"

		dest := &mocks.Destination{}

		wr := writer.New([]writer.Destination{dest})

		manager := health.NewManager(ctx, config)

		fameworkStop := health.FrameworkStart(ctx, config, manager, wr)

		time.Sleep(cycle)

		fameworkStop()
	})
})
