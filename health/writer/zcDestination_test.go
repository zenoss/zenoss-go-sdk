package writer_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
	data_registry "github.com/zenoss/zenoss-protobufs/go/cloud/data-registry"
	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
)

var _ = Describe("Destination", func() {
	var (
		ctx      context.Context
		out      *data_receiver.MockDataReceiverServiceClient
		regout   *data_registry.MockDataRegistryServiceClient
		epConfig *endpoint.Config
		zcDest   *writer.ZCDestination

		targetID string
	)

	BeforeEach(func() {
		ctx = context.Background()

		targetID = "testTarget"

		out = &data_receiver.MockDataReceiverServiceClient{}
		regout = &data_registry.MockDataRegistryServiceClient{}

		epConfig = &endpoint.Config{
			APIKey:         "x",
			TestClient:     out,
			TestRegClient:  regout,
			MinTTL:         10000,
			MaxTTL:         100000,
			CacheSizeLimit: 200000,
		}
		zcDest, _ = writer.NewZCDestination(&writer.ZCDestinationConfig{EndpointConfig: epConfig})
	})

	Context("NewZCDestination", func() {
		It("should return a new ZCDestination", func() {
			config := &writer.ZCDestinationConfig{EndpointConfig: epConfig}
			zcDest, err := writer.NewZCDestination(config)

			Ω(err).Should(BeNil())
			Ω(zcDest).ShouldNot(BeNil())
		})
	})

	Context("Push", func() {
		It("should push Health info as a metric", func() {
			metricName := "testMetric"
			value := 2.3
			counterName := "testCounter"
			cValue := int32(10)

			health := target.NewHealth(targetID, "")
			health.Metrics = make(map[string]float64)
			health.Metrics[metricName] = value
			health.Counters = make(map[string]int32)
			health.Counters[counterName] = cValue

			out.On("PutMetrics", mock.Anything, mock.Anything).Return(&data_receiver.StatusResult{
				Failed:    0,
				Succeeded: 1,
			}, nil)

			err := zcDest.Push(ctx, health)
			zcDest.Endpoint.Flush()

			Ω(err).Should(BeNil())
			lastPush := out.Calls[len(out.Calls)-1].Arguments[1].(*data_receiver.Metrics)
			Ω(len(lastPush.Metrics)).Should(Equal(2))
			names := []string{lastPush.Metrics[0].Metric, lastPush.Metrics[1].Metric}
			Ω(names).Should(ContainElements(metricName, counterName))
		})

		It("should not fail even if zc is not avilable", func() {
			metricName := "testMetric"
			value := 2.3
			mockedErr := errors.New("mocked")

			health := target.NewHealth(targetID, "")
			health.Metrics = make(map[string]float64)
			health.Metrics[metricName] = value

			out.On("PutMetrics", mock.Anything, mock.Anything).Return(nil, mockedErr)

			err := zcDest.Push(ctx, health)
			zcDest.Endpoint.Flush()

			out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 1)
			Ω(err).Should(BeNil())
		})
	})
})
