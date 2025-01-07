package writer_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
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

		componentID string
		mockedErr   error
	)

	BeforeEach(func() {
		ctx = context.Background()

		componentID = "testComponent"
		mockedErr = errors.New("mocked")

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
		var (
			metricName string
			value      float64
		)

		BeforeEach(func() {
			metricName = "testMetric"
			value = 2.3
		})

		It("should push Health info as a metric", func() {
			counterName := "testCounter"
			cValue := int32(10)

			health := component.NewHealth(componentID, "", "")
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
			health := component.NewHealth(componentID, "", "")
			health.Metrics = make(map[string]float64)
			health.Metrics[metricName] = value

			out.On("PutMetrics", mock.Anything, mock.Anything).Return(nil, mockedErr)

			err := zcDest.Push(ctx, health)
			zcDest.Endpoint.Flush()

			out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 1)
			Ω(err).Should(BeNil())
		})
	})

	Context("Register", func() {
		It("should push Component info as a model", func() {
			empty := []string{}
			component, err := component.New(
				componentID, utils.DefaultComponentType, utils.DefaultHealthTarget, false,
				empty, empty, empty,
			)
			Ω(err).Should(BeNil())

			out.On("PutModels", mock.Anything, mock.Anything).Return(&data_receiver.ModelStatusResult{
				Failed:    0,
				Succeeded: 1,
			}, nil)

			err = zcDest.Register(ctx, component)
			zcDest.Endpoint.Flush()

			Ω(err).Should(BeNil())
			lastPush := out.Calls[len(out.Calls)-1].Arguments[1].(*data_receiver.Models)
			Ω(len(lastPush.Models)).Should(Equal(1))
			Ω(lastPush.Models[0].Dimensions[utils.ComponentKey]).Should(Equal(componentID))
		})

		It("should not fail even if zc is not avilable", func() {
			empty := []string{}
			component, err := component.New(
				componentID, utils.DefaultComponentType, utils.DefaultHealthTarget, false,
				empty, empty, empty,
			)
			Ω(err).Should(BeNil())

			out.On("PutModels", mock.Anything, mock.Anything).Return(nil, mockedErr)

			err = zcDest.Register(ctx, component)
			zcDest.Endpoint.Flush()

			out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			Ω(err).Should(BeNil())
		})
	})
})
