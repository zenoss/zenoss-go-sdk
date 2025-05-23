package endpoint_test

import (
	"context"
	"errors"
	stdlog "log"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/mock"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/log"
	data_registry "github.com/zenoss/zenoss-protobufs/go/cloud/data-registry"
	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
	_struct "google.golang.org/protobuf/types/known/structpb"
)

func TestEndpoint(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Endpoint Suite")
}

var _ = Describe("Endpoint", func() {
	var e *endpoint.Endpoint
	var out *data_receiver.MockDataReceiverServiceClient
	var regout *data_registry.MockDataRegistryServiceClient
	var err error
	var logOutput *gbytes.Buffer

	BeforeEach(func() {
		out = &data_receiver.MockDataReceiverServiceClient{}
		regout = &data_registry.MockDataRegistryServiceClient{}
		logOutput = gbytes.NewBuffer()
		stdlog.SetOutput(logOutput)
	})

	AfterEach(func() {
		stdlog.SetOutput(os.Stdout)
	})

	Context("with basic configuration", func() {
		BeforeEach(func() {
			e, err = endpoint.New(endpoint.Config{
				APIKey:          "x",
				TestClient:      out,
				TestRegClient:   regout,
				MinTTL:          10000,
				MaxTTL:          100000,
				InitialBackoff:  10 * time.Millisecond,
				NumberOfRetries: 3,
				CacheSizeLimit:  200000,
			})
		})

		It("should be created without error", func() {
			Ω(err).ShouldNot(HaveOccurred())
			Ω(e).ShouldNot(BeNil())
		})

		It("logs with a field", func() {
			log.Error(e, log.Fields{"one": 1}, "an %s", "error")
			Ω(logOutput).Should(gbytes.Say(`an error fields=map\[endpoint:default one:1\]`))
		})

		Context("PutMetric", func() {
			It("is unimplemented", func() {
				c, err := e.PutMetric(context.Background())
				Ω(err).Should(HaveOccurred())
				Ω(err).Should(Equal(status.Error(codes.Unimplemented, "PutMetric is not supported")))
				Ω(c).Should(BeNil())
			})
		})

		Context("PutMetrics", func() {
			It("sends all metric types", func() {
				out.On("PutMetrics", mock.Anything, mock.Anything).
					Return(nil, nil)

				r, err := e.PutMetrics(context.TODO(), &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics: []*data_receiver.Metric{
						{Metric: "canonical1"},
						{Metric: "canonical2"},
					},
					CompactMetrics: []*data_receiver.CompactMetric{
						{Id: "compact1"},
						{Id: "compact2"},
					},
					TaggedMetrics: []*data_receiver.TaggedMetric{
						{Metric: "tagged1"},
						{Metric: "tagged2"},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 6))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				// Skip bundler delay thresholds.
				e.Flush()

				// Each type of metric should be sent to out in a separate call.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
			})
		})

		Context("ConvertMetrics", func() {
			It("converts canonical metrics to compact metrics", func() {
				responses := make([]*data_registry.RegisterMetricVerboseResponse, 0)
				verboseResponse := &data_registry.RegisterMetricVerboseResponse{
					Response: &data_registry.RegisterMetricResponse{
						InstanceId: "id123456canonical1",
						Name:       "canonical1", // replace by default
					},
				}
				responses = append(responses, verboseResponse)
				regCreateOrUpdateStreamingClientMock := &data_registry.MockDataRegistryService_CreateOrUpdateMetricsClient{}
				regCreateOrUpdateStreamingClientMock.On("Send", mock.Anything).Return(nil)
				regCreateOrUpdateStreamingClientMock.On("CloseAndRecv").Return(&data_registry.RegisterMetricsResponse{
					Responses: responses,
				}, nil)

				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(regCreateOrUpdateStreamingClientMock, nil)
				inputMetrics := []*data_receiver.Metric{
					{
						Metric:    "canonical1",
						Value:     1,
						Timestamp: time.Now().UnixNano() / 1e6,
						Dimensions: map[string]string{
							"source": "sdk.zdm.test",
							"app":    "sdktest",
						},
						MetadataFields: &_struct.Struct{
							Fields: map[string]*_struct.Value{
								"srckey": {
									Kind: &_struct.Value_StringValue{
										StringValue: "srcvalue",
									},
								},
							},
						},
					},
				}
				compactMetrics, failedMetrics := e.ConvertMetrics(context.TODO(), inputMetrics)

				Ω(failedMetrics).ShouldNot(BeNil())
				Ω(compactMetrics).ShouldNot(BeNil())
				Ω(len(compactMetrics)).Should(BeNumerically("==", 1))
				Ω(len(failedMetrics)).Should(BeNumerically("==", 0))
				// Skip bundler delay thresholds.
				e.Flush()
			})

			It("returns failed metrics if CreateOrUpdateMetrics fails on first try", func() {
				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(nil, errors.New("initial registration error"))

				inputMetrics := []*data_receiver.Metric{
					{
						Metric:    "failmetric",
						Value:     123,
						Timestamp: time.Now().UnixNano() / 1e6,
						Dimensions: map[string]string{
							"source": "sdk.zdm.test",
							"app":    "sdktest",
						},
					},
				}

				compactMetrics, failedMetrics := e.ConvertMetrics(context.TODO(), inputMetrics)

				Ω(compactMetrics).Should(HaveLen(0))
				Ω(failedMetrics).Should(HaveLen(1))
				Ω(failedMetrics[0].Metric).Should(Equal("failmetric"))

				e.Flush()
			})

			It("returns failed metrics if retry after partial failure also fails", func() {
				responses := make([]*data_registry.RegisterMetricVerboseResponse, 0)
				verboseResponse := &data_registry.RegisterMetricVerboseResponse{
					Response: &data_registry.RegisterMetricResponse{
						InstanceId: "id123456canonical1",
						Name:       "canonical1",
					},
				}
				errorResponse := &data_registry.RegisterMetricVerboseResponse{
					Error: "error occured",
				}
				responses = append(responses, verboseResponse, errorResponse)
				regCreateOrUpdateStreamingClientMock := &data_registry.MockDataRegistryService_CreateOrUpdateMetricsClient{}
				regCreateOrUpdateStreamingClientMock.On("Send", mock.Anything).Return(nil)
				regCreateOrUpdateStreamingClientMock.On("CloseAndRecv").Return(&data_registry.RegisterMetricsResponse{
					Responses: responses,
				}, nil)

				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(regCreateOrUpdateStreamingClientMock, nil).Once()
				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(nil, errors.New("Error closing stream")).Times(3)
				inputMetrics := []*data_receiver.Metric{
					{
						Metric:    "canonical1",
						Value:     1,
						Timestamp: time.Now().UnixNano() / 1e6,
						Dimensions: map[string]string{
							"source": "sdk.zdm.test",
							"app":    "sdktest",
						},
						MetadataFields: &_struct.Struct{
							Fields: map[string]*_struct.Value{
								"srckey": {
									Kind: &_struct.Value_StringValue{
										StringValue: "srcvalue",
									},
								},
							},
						},
					},
					{
						Metric:    "failing1",
						Value:     2,
						Timestamp: time.Now().UnixNano() / 1e6,
						Dimensions: map[string]string{
							"source": "sdk.zdm.test",
							"app":    "sdktest",
						},
						MetadataFields: &_struct.Struct{
							Fields: map[string]*_struct.Value{
								"srckey": {
									Kind: &_struct.Value_StringValue{
										StringValue: "srcvalue",
									},
								},
							},
						},
					},
				}
				compactMetrics, failedMetrics := e.ConvertMetrics(context.TODO(), inputMetrics)

				Ω(failedMetrics).Should(HaveLen(1))
				Ω(compactMetrics).Should(HaveLen(1))
				// Skip bundler delay thresholds.
				e.Flush()
			})

			It("returns compact metrics if failed metrics from batch were registered after retry", func() {
				firstResponses := make([]*data_registry.RegisterMetricVerboseResponse, 0)
				verboseResponse1 := &data_registry.RegisterMetricVerboseResponse{
					Response: &data_registry.RegisterMetricResponse{
						InstanceId: "id123456canonical1",
						Name:       "canonical1",
					},
				}
				errorResponse := &data_registry.RegisterMetricVerboseResponse{
					Error: "error occured",
				}
				firstResponses = append(firstResponses, verboseResponse1, errorResponse)

				firstStreamMock := &data_registry.MockDataRegistryService_CreateOrUpdateMetricsClient{}
				firstStreamMock.On("Send", mock.Anything).Return(nil)
				firstStreamMock.On("CloseAndRecv").Return(&data_registry.RegisterMetricsResponse{
					Responses: firstResponses,
				}, nil)

				retryResponses := make([]*data_registry.RegisterMetricVerboseResponse, 0)
				verboseResponse2 := &data_registry.RegisterMetricVerboseResponse{
					Response: &data_registry.RegisterMetricResponse{
						InstanceId: "id123456canonical2",
						Name:       "canonical2",
					},
				}
				retryResponses = append(retryResponses, verboseResponse2)

				retryStreamMock := &data_registry.MockDataRegistryService_CreateOrUpdateMetricsClient{}
				retryStreamMock.On("Send", mock.Anything).Return(nil)
				retryStreamMock.On("CloseAndRecv").Return(&data_registry.RegisterMetricsResponse{
					Responses: retryResponses,
				}, nil)

				// 1 registered, 1 failed
				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(firstStreamMock, nil).Once()
				// trying to reregister the failed one, error occurred
				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(nil, errors.New("Error closing stream")).Once()
				// retrying with backoff, 1 registered
				regout.On("CreateOrUpdateMetrics", mock.Anything, mock.Anything).
					Return(retryStreamMock, nil).Once()
				inputMetrics := []*data_receiver.Metric{
					{
						Metric:    "canonical1",
						Value:     1,
						Timestamp: time.Now().UnixNano() / 1e6,
						Dimensions: map[string]string{
							"source": "sdk.zdm.test",
							"app":    "sdktest",
						},
						MetadataFields: &_struct.Struct{
							Fields: map[string]*_struct.Value{
								"srckey": {
									Kind: &_struct.Value_StringValue{
										StringValue: "srcvalue",
									},
								},
							},
						},
					},
					{
						Metric:    "canonical2",
						Value:     2,
						Timestamp: time.Now().UnixNano() / 1e6,
						Dimensions: map[string]string{
							"source": "sdk.zdm.test",
							"app":    "sdktest",
						},
						MetadataFields: &_struct.Struct{
							Fields: map[string]*_struct.Value{
								"srckey": {
									Kind: &_struct.Value_StringValue{
										StringValue: "srcvalue",
									},
								},
							},
						},
					},
				}
				compactMetrics, failedMetrics := e.ConvertMetrics(context.TODO(), inputMetrics)

				Ω(failedMetrics).Should(BeEmpty())
				Ω(compactMetrics).Should(HaveLen(2))
				// Skip bundler delay thresholds.
				e.Flush()
			})
		})

		Context("PutModels", func() {
			It("sends models", func() {
				out.On("PutModels", mock.Anything, mock.Anything).
					Return(nil, nil)

				r, err := e.PutModels(context.TODO(), &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{
						{
							Timestamp: time.Now().UnixNano() / 1e9,
							Dimensions: map[string]string{
								"source": "bob",
								"app":    "toolbox",
							},
						},
						{
							Timestamp: time.Now().UnixNano() / 1e9,
							Dimensions: map[string]string{
								"source": "bob",
								"app":    "toolbox",
							},
						},
						{
							Timestamp: time.Now().UnixNano() / 1e9,
							Dimensions: map[string]string{
								"source": "joe",
								"app":    "toolbox",
							},
						},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 2))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				// Skip bundler delay thresholds.
				e.Flush()

				// Models should end up being sent to out.
				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			})
		})

		Context("PutEvent", func() {
			It("is unimplemented", func() {
				c, err := e.PutEvent(context.Background())
				Ω(err).Should(HaveOccurred())
				Ω(err).Should(Equal(status.Error(codes.Unimplemented, "PutEvent is not supported")))
				Ω(c).Should(BeNil())
			})
		})

		Context("PutEvents", func() {
			It("sends events", func() {
				out.On("PutEvents", mock.Anything, mock.Anything).
					Return(nil, nil)

				r, err := e.PutEvents(context.TODO(), &data_receiver.Events{
					DetailedResponse: true,
					Events: []*data_receiver.Event{
						{
							Timestamp: time.Now().UnixNano() / 1e9,
							Dimensions: map[string]string{
								"source": "bob",
								"app":    "toolbox",
							},
						},
						{
							Timestamp: time.Now().UnixNano() / 1e9,
							Dimensions: map[string]string{
								"source": "joe",
								"app":    "toolbox",
							},
						},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 2))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				// Skip bundler delay thresholds.
				e.Flush()

				// Events should end up being sent to out.
				out.AssertNumberOfCalls(GinkgoT(), "PutEvents", 1)
			})
		})
	})

	Context("with a failing server", func() {
		BeforeEach(func() {
			e, err = endpoint.New(endpoint.Config{
				TestClient: out,
			})

			out.On("PutMetrics", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))

			out.On("PutModels", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))

			out.On("PutEvents", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))
		})

		Context("PutMetrics", func() {
			It("appears to succeed", func() {
				r, err := e.PutMetrics(context.TODO(), &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics: []*data_receiver.Metric{
						{Metric: "canonical1"},
						{Metric: "canonical2"},
					},
					CompactMetrics: []*data_receiver.CompactMetric{
						{Id: "compact1"},
						{Id: "compact2"},
					},
					TaggedMetrics: []*data_receiver.TaggedMetric{
						{Metric: "tagged1"},
						{Metric: "tagged2"},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 6))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				e.Flush()

				// One call is made for each type.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
			})
		})

		Context("PutModels", func() {
			It("appears to succeed", func() {
				r, err := e.PutModels(context.TODO(), &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 2))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				e.Flush()

				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			})
		})

		Context("PutEvents", func() {
			It("appears to succeed", func() {
				r, err := e.PutEvents(context.TODO(), &data_receiver.Events{
					DetailedResponse: true,
					Events: []*data_receiver.Event{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 2))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				e.Flush()

				out.AssertNumberOfCalls(GinkgoT(), "PutEvents", 1)
			})
		})
	})

	Context("with partially failing data", func() {
		BeforeEach(func() {
			e, err = endpoint.New(endpoint.Config{
				TestClient: out,
			})

			out.On("PutMetrics", mock.Anything, mock.Anything).
				Return(&data_receiver.StatusResult{
					Failed:    1,
					Succeeded: 1,
				}, nil)

			out.On("PutModels", mock.Anything, mock.Anything).
				Return(&data_receiver.ModelStatusResult{
					Failed:    1,
					Succeeded: 1,
				}, nil)

			out.On("PutEvents", mock.Anything, mock.Anything).
				Return(&data_receiver.EventStatusResult{
					Failed:    1,
					Succeeded: 1,
				}, nil)
		})

		Context("PutMetrics", func() {
			It("logs warnings", func() {
				r, err := e.PutMetrics(context.TODO(), &data_receiver.Metrics{
					Metrics: []*data_receiver.Metric{
						{Metric: "canonical1"},
						{Metric: "canonical2"},
					},
					CompactMetrics: []*data_receiver.CompactMetric{
						{Id: "compact1"},
						{Id: "compact2"},
					},
					TaggedMetrics: []*data_receiver.TaggedMetric{
						{Metric: "tagged1"},
						{Metric: "tagged2"},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 6))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				e.Flush()

				// One call is made for each type.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)

				Ω(logOutput).Should(gbytes.Say(`failed to send some metrics fields=map\[endpoint:default failed:1 succeeded:1\]`))
				Ω(logOutput).Should(gbytes.Say(`failed to send some compact metrics fields=map\[endpoint:default failed:1 succeeded:1\]`))
				Ω(logOutput).Should(gbytes.Say(`failed to send some tagged metrics fields=map\[endpoint:default failed:1 succeeded:1\]`))
			})
		})

		Context("PutModels", func() {
			It("logs a warning", func() {
				r, err := e.PutModels(context.TODO(), &data_receiver.Models{
					Models: []*data_receiver.Model{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 2))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				e.Flush()

				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)

				Ω(logOutput).Should(gbytes.Say(`failed to send some models fields=map\[endpoint:default failed:1 succeeded:1\]`))
			})
		})

		Context("PutEvents", func() {
			It("logs a warning", func() {
				r, err := e.PutEvents(context.TODO(), &data_receiver.Events{
					Events: []*data_receiver.Event{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 2))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				e.Flush()

				out.AssertNumberOfCalls(GinkgoT(), "PutEvents", 1)

				Ω(logOutput).Should(gbytes.Say(`failed to send some events fields=map\[endpoint:default failed:1 succeeded:1\]`))
			})
		})
	})

	Context("with overflowing bundlers", func() {
		BeforeEach(func() {
			e, err = endpoint.New(endpoint.Config{
				BundlerConfig: endpoint.BundlerConfig{
					DelayThreshold:       1 * time.Hour,
					BundleCountThreshold: 1000,
					BundleByteLimit:      1,
					BufferedByteLimit:    1,
					HandlerLimit:         2,
				},
				TestClient: out,
			})
		})

		Context("PutMetrics", func() {
			It("has bundler errors for some metrics", func() {
				out.On("PutMetrics", mock.Anything, mock.Anything).
					Return(nil, nil)

				r, err := e.PutMetrics(context.TODO(), &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics: []*data_receiver.Metric{
						{Metric: "canonical1"},
						{Metric: "canonical2"},
					},
					CompactMetrics: []*data_receiver.CompactMetric{
						{Id: "compact1"},
						{Id: "compact2"},
					},
					TaggedMetrics: []*data_receiver.TaggedMetric{
						{Metric: "tagged1"},
						{Metric: "tagged2"},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())

				// The first of each type succeeds.
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 3))

				// The second of each type fails due to overflow.
				Ω(r.GetFailed()).Should(BeNumerically("==", 3))
				Ω(r.GetFailedMetrics()).Should(HaveLen(1))
				Ω(r.GetFailedCompactMetrics()).Should(HaveLen(1))
				Ω(r.GetFailedTaggedMetrics()).Should(HaveLen(1))

				e.Flush()

				// One call of each type is still made to deliver first of each type.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
			})
		})

		Context("PutModels", func() {
			It("has bundler errors for some models", func() {
				out.On("PutModels", mock.Anything, mock.Anything).
					Return(nil, nil)

				r, err := e.PutModels(context.TODO(), &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())

				// The first model succeeds.
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 1))

				// The second model fails due to overflow.
				Ω(r.GetFailed()).Should(BeNumerically("==", 1))
				Ω(r.GetFailedModels()).Should(HaveLen(1))

				e.Flush()

				// One call is still made to deliver first model.
				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			})
		})

		Context("PutEvents", func() {
			It("has bundler errors for some events", func() {
				out.On("PutEvents", mock.Anything, mock.Anything).
					Return(nil, nil)

				r, err := e.PutEvents(context.TODO(), &data_receiver.Events{
					DetailedResponse: true,
					Events: []*data_receiver.Event{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())

				// The first event succeeds.
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 1))

				// The second event fails due to overflow.
				Ω(r.GetFailed()).Should(BeNumerically("==", 1))
				Ω(r.GetFailedEvents()).Should(HaveLen(1))

				e.Flush()

				// One call is still made to deliver first event.
				out.AssertNumberOfCalls(GinkgoT(), "PutEvents", 1)
			})
		})
	})

	Context("with DisableTLS", func() {
		BeforeEach(func() {
			e, err = endpoint.New(endpoint.Config{
				DisableTLS: true,
			})
		})

		It("should be created without error", func() {
			Ω(err).ShouldNot(HaveOccurred())
			Ω(e).ShouldNot(BeNil())
		})
	})

	Context("with LoggerConfig", func() {
		BeforeEach(func() {
			e, err = endpoint.New(endpoint.Config{
				Name:   "test-logging",
				APIKey: "x",
				BundlerConfig: endpoint.BundlerConfig{
					DelayThreshold:       time.Second,
					BundleCountThreshold: 1000,
				},
				LoggerConfig: log.LoggerConfig{
					Fields: log.Fields{"x": "X"},
					Level:  log.LevelDebug,
					Func: func(level log.Level, fields log.Fields, format string, args ...interface{}) {
						stdlog.Printf("level=%v fields=%v format=%v args=%v", level, fields, format, args)
					},
				},
			})
		})

		It("should be created without error", func() {
			Ω(err).ShouldNot(HaveOccurred())
			Ω(e).ShouldNot(BeNil())
		})

		It("logs with a field", func() {
			log.Debug(e, log.Fields{"y": "Y"}, "one=%v", 1)
			Ω(logOutput).Should(gbytes.Say(`level=1 fields=map\[x:X y:Y\] format=one=%v args=\[1\]`))
		})
	})
})
