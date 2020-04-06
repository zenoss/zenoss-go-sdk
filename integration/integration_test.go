package integration_test

import (
	"context"
	"io/ioutil"
	stdlog "log"
	"math/rand"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v2"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/metadata"
	"github.com/zenoss/zenoss-go-sdk/processor"
	"github.com/zenoss/zenoss-go-sdk/splitter"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Integration Suite", []Reporter{junitReporter})
}

var _ = Describe("Integration", func() {
	var logOutput *gbytes.Buffer

	BeforeEach(func() {
		logOutput = gbytes.NewBuffer()
		stdlog.SetOutput(logOutput)
	})

	AfterEach(func() {
		stdlog.SetOutput(os.Stdout)
	})

	Context("OpenCensus gRPC", func() {
		var clients []*data_receiver.MockDataReceiverServiceClient
		var endpoints []data_receiver.DataReceiverServiceClient
		var metrics *data_receiver.Metrics
		var timestamp int64

		var metricStatusResult *data_receiver.StatusResult
		var metricError error

		BeforeEach(func() {
			timestamp = time.Now().UnixNano() / 1e6

			// Define multiple endpoints to test splitter.
			clients = []*data_receiver.MockDataReceiverServiceClient{{}, {}}
			endpoints = make([]data_receiver.DataReceiverServiceClient, len(clients), len(clients))
			for i, client := range clients {
				e, err := endpoint.New(endpoint.Config{TestClient: client})
				Ω(err).ShouldNot(HaveOccurred())
				endpoints[i] = e
			}

			// Define tagged metrics that OpenCensus might send.
			metrics = &data_receiver.Metrics{
				DetailedResponse: true,
				TaggedMetrics: []*data_receiver.TaggedMetric{
					{
						Metric:    "grpc.io/server/completed_rpcs",
						Timestamp: timestamp,
						Value:     123.0,
						Tags: map[string]string{
							"source":             "example-grpc-service",
							"source-type":        "zenoss/opencensus-go-exporter-zenoss",
							"grpc_server_method": "com.example.ExampleService.ListThings",
							"grpc_server_status": "OK",
							"description":        "Completed RPCs by method and status.",
							"units":              "1",
							"k8s.cluster":        "cluster77",
							"k8s.namespace":      "namespace77",
							"k8s.pod":            "pod77",
						},
					},
					{
						Metric:    "grpc.io/server/server_latency/mean",
						Timestamp: timestamp,
						Value:     56.7,
						Tags: map[string]string{
							"source":             "example-grpc-service",
							"source-type":        "zenoss/opencensus-go-exporter-zenoss",
							"grpc_server_method": "com.example.ExampleService.ListThings",
							"description":        "RPC latency by method.",
							"units":              "ms",
							"k8s.cluster":        "cluster77",
							"k8s.namespace":      "namespace77",
							"k8s.pod":            "pod77",
						},
					},
					{
						Metric:    "example.com/app/requests",
						Timestamp: timestamp,
						Value:     123456789.0,
						Tags: map[string]string{
							"source":        "example-grpc-service",
							"source-type":   "zenoss/opencensus-go-exporter-zenoss",
							"customer-id":   "876123876",
							"description":   "Requests by customer.",
							"units":         "1",
							"k8s.cluster":   "cluster77",
							"k8s.namespace": "namespace77",
							"k8s.pod":       "pod77",
						},
					},
				},
			}

			// Setup expectations for transformed data.
			for _, client := range clients {
				client.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					Metrics: []*data_receiver.Metric{
						{
							Metric:    "grpc.io/server/completed_rpcs",
							Timestamp: timestamp,
							Value:     123.0,
							Dimensions: map[string]string{
								"source":             "example-grpc-service",
								"grpc_server_method": "com.example.ExampleService.ListThings",
								"grpc_server_status": "OK",
							},
							MetadataFields: metadata.MustFromMap(map[string]interface{}{
								"source-type":   "zenoss/opencensus-go-exporter-zenoss",
								"description":   "Completed RPCs by method and status.",
								"units":         "1",
								"k8s.cluster":   "cluster77",
								"k8s.namespace": "namespace77",
								"k8s.pod":       "pod77",
								"processed-by":  []string{"my-processor"},
							}),
						},
						{
							Metric:    "grpc.io/server/server_latency/mean",
							Timestamp: timestamp,
							Value:     56.7,
							Dimensions: map[string]string{
								"source":             "example-grpc-service",
								"grpc_server_method": "com.example.ExampleService.ListThings",
							},
							MetadataFields: metadata.MustFromMap(map[string]interface{}{
								"source-type":   "zenoss/opencensus-go-exporter-zenoss",
								"description":   "RPC latency by method.",
								"units":         "ms",
								"k8s.cluster":   "cluster77",
								"k8s.namespace": "namespace77",
								"k8s.pod":       "pod77",
								"processed-by":  []string{"my-processor"},
							}),
						},
					},
				}).Return(nil, nil)

				client.On("PutModels", mock.Anything, &data_receiver.Models{
					Models: []*data_receiver.Model{
						{
							Timestamp: timestamp,
							Dimensions: map[string]string{
								"source":             "example-grpc-service",
								"grpc_server_method": "com.example.ExampleService.ListThings",
								"grpc_server_status": "OK",
							},
							MetadataFields: metadata.MustFromMap(map[string]interface{}{
								"name":          "example-grpc-service/com.example.ExampleService.ListThings/OK",
								"source-type":   "zenoss/opencensus-go-exporter-zenoss",
								"k8s.cluster":   "cluster77",
								"k8s.namespace": "namespace77",
								"k8s.pod":       "pod77",
								"processed-by":  []string{"my-processor"},
								"impactFromDimensions": []string{
									"k8s.cluster=cluster77,k8s.namespace=namespace77,k8s.pod=pod77",
								},
							}),
						},
						{
							Timestamp: timestamp,
							Dimensions: map[string]string{
								"source":             "example-grpc-service",
								"grpc_server_method": "com.example.ExampleService.ListThings",
							},
							MetadataFields: metadata.MustFromMap(map[string]interface{}{
								"source-type":   "zenoss/opencensus-go-exporter-zenoss",
								"k8s.cluster":   "cluster77",
								"k8s.namespace": "namespace77",
								"k8s.pod":       "pod77",
								"name":          "example-grpc-service/com.example.ExampleService.ListThings",
								"processed-by":  []string{"my-processor"},
								"impactFromDimensions": []string{
									"k8s.cluster=cluster77,k8s.namespace=namespace77,k8s.pod=pod77",
								},
							}),
						},
					},
				}).Return(nil, nil)
			}

			splitter, err := splitter.New(splitter.Config{Outputs: endpoints})
			Ω(err).ShouldNot(HaveOccurred())

			// Load processor configuration from opencensus-grpc.yaml.
			var config processor.Config

			yamlFile, err := ioutil.ReadFile("opencensus-grpc.yaml")
			Ω(err).ShouldNot(HaveOccurred())

			err = yaml.Unmarshal(yamlFile, &config)
			Ω(err).ShouldNot(HaveOccurred())

			config.Output = splitter

			p, err := processor.New(config)
			Ω(err).ShouldNot(HaveOccurred())

			metricStatusResult, metricError = p.PutMetrics(context.TODO(), metrics)
		})

		It("accepts metrics with no error", func() {
			Ω(metricError).ShouldNot(HaveOccurred())
		})

		It("reports all metrics sent successfully", func() {
			Ω(metricStatusResult.Succeeded).Should(BeNumerically("==", 3))
			Ω(metricStatusResult.Failed).Should(BeNumerically("==", 0))
		})

		It("sent expected metrics and models to all endpoints", func() {
			for _, endpoint := range endpoints {
				endpoint.(interface{ Flush() }).Flush()
			}

			for _, client := range clients {
				client.AssertExpectations(GinkgoT())
			}
		})

		It("logs dropped metrics", func() {
			Ω(logOutput).Should(gbytes.Say(
				`Metric{Metric: "example.com/app/requests", Dimensions: map\[customer-id:876123876 source:example-grpc-service\], MetadataFields: map\[description:Requests by customer. k8s.cluster:cluster77 k8s.namespace:namespace77 k8s.pod:pod77 processed-by:\[my-processor\] source-type:zenoss/opencensus-go-exporter-zenoss units:1\]}`))
		})
	})
})
