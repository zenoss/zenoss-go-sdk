package proxy_test

import (
	"context"
	stdlog "log"
	"math/rand"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/mock"
	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/zenoss/zenoss-go-sdk/log"
	"github.com/zenoss/zenoss-go-sdk/proxy"
)

func TestProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	RunSpecs(t, "Proxy Suite")
}

var _ = Describe("Proxy", func() {
	var logOutput *gbytes.Buffer
	var out *data_receiver.MockDataReceiverServiceClient
	var p *proxy.Proxy
	var err error
	var ctx context.Context

	BeforeEach(func() {
		logOutput = gbytes.NewBuffer()
		stdlog.SetOutput(logOutput)

		out = &data_receiver.MockDataReceiverServiceClient{}
	})

	AfterEach(func() {
		stdlog.SetOutput(os.Stdout)
	})

	Context("with no output", func() {
		BeforeEach(func() {
			p, err = proxy.New(proxy.Config{})
		})

		It("should return an error", func() {
			Ω(p).Should(BeNil())
			Ω(err).Should(MatchError("Config.Output is nil"))
		})
	})

	Context("with a minimal configuration", func() {
		BeforeEach(func() {
			p, err = proxy.New(proxy.Config{Output: out})
		})

		It("should return a proxy", func() {
			Ω(p).ShouldNot(BeNil())
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should log with default fields", func() {
			log.Error(p, nil, "one=%v", 1)
			Ω(logOutput).Should(gbytes.Say(`one=1 fields=map\[proxy:default\]`))
		})

		Context("SetConfig", func() {
			It("should change allowed API keys", func() {
				metrics := &data_receiver.Metrics{
					Metrics: []*data_receiver.Metric{{Metric: "test"}},
				}

				out.On("PutMetrics", mock.Anything, metrics).Return(nil, nil)

				err := p.SetConfig(proxy.Config{
					AllowedAPIKeys: []string{"test"},
					Output:         out,
				})

				r, err := p.PutMetrics(context.TODO(), metrics)
				Ω(r).Should(BeNil())
				Ω(err).Should(Equal(
					status.Error(
						codes.Unauthenticated,
						`no metadata in context`)))

				out.AssertNotCalled(GinkgoT(), "PutMetrics")
			})

			Context("with no output", func() {
				It("should return an error", func() {
					err = p.SetConfig(proxy.Config{})
					Ω(err).Should(MatchError("Config.Output is nil"))
				})
			})
		})

		Context("PutMetric", func() {
			It("is unimplemented", func() {
				err := p.PutMetric(nil)
				Ω(err).Should(HaveOccurred())
				Ω(err).Should(Equal(status.Error(codes.Unimplemented, "PutMetric is not supported")))
			})
		})

		Context("PutMetrics", func() {
			It("sends metrics to output", func() {
				out.On("PutMetrics", mock.Anything, mock.Anything).
					Return(&data_receiver.StatusResult{Failed: 0, Succeeded: 2}, nil)

				r, err := p.PutMetrics(context.TODO(), &data_receiver.Metrics{
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

				// One call for each type of metric.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
			})
		})

		Context("PutModels", func() {
			It("sends models to output", func() {
				out.On("PutModels", mock.Anything, mock.Anything).
					Return(&data_receiver.ModelStatusResult{Failed: 0, Succeeded: 1}, nil)

				r, err := p.PutModels(context.TODO(), &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{
						{
							Timestamp: time.Now().UnixNano() / 1e9,
							Dimensions: map[string]string{
								"source": "bob",
								"app":    "toolbox",
							},
						},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 1))
				Ω(r.GetFailed()).Should(BeNumerically("==", 0))

				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			})
		})
	})

	Context("with a failing output", func() {
		BeforeEach(func() {
			p, _ = proxy.New(proxy.Config{Output: out})

			out.On("PutMetrics", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))

			out.On("PutModels", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))
		})

		Context("PutMetrics", func() {
			It("fails for all metrics", func() {
				r, err := p.PutMetrics(context.TODO(), &data_receiver.Metrics{
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
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 0))
				Ω(r.GetFailed()).Should(BeNumerically("==", 6))

				// One call is made for each type.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
			})

			It("fails for all models", func() {
				r, err := p.PutModels(context.TODO(), &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{
						{Dimensions: map[string]string{"test": "value1"}},
						{Dimensions: map[string]string{"test": "value2"}},
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())
				Ω(r.GetSucceeded()).Should(BeNumerically("==", 0))
				Ω(r.GetFailed()).Should(BeNumerically("==", 2))

				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			})
		})
	})

	Context("with allowed API keys", func() {
		BeforeEach(func() {
			p, err = proxy.New(proxy.Config{
				AllowedAPIKeys: []string{"test"},
				Output:         out,
			})

			Ω(p).ShouldNot(BeNil())
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with no metadata", func() {
			Context("PutMetrics", func() {
				It("should return an error", func() {
					r, err := p.PutMetrics(context.TODO(), &data_receiver.Metrics{})
					Ω(r).Should(BeNil())
					Ω(err).Should(Equal(
						status.Error(
							codes.Unauthenticated,
							`no metadata in context`)))
				})
			})

			Context("PutModels", func() {
				It("should return an error", func() {
					r, err := p.PutModels(context.TODO(), &data_receiver.Models{})
					Ω(r).Should(BeNil())
					Ω(err).Should(Equal(
						status.Error(
							codes.Unauthenticated,
							`no metadata in context`)))
				})
			})
		})

		Context("with no API key", func() {
			BeforeEach(func() {
				ctx = context.Background()
				md := metadata.Pairs("wrong-api-key", "test")
				ctx = metadata.NewIncomingContext(ctx, md)
			})

			Context("PutMetrics", func() {
				It("should return an error", func() {
					r, err := p.PutMetrics(ctx, &data_receiver.Metrics{})
					Ω(r).Should(BeNil())
					Ω(err).Should(Equal(
						status.Error(
							codes.Unauthenticated,
							`no "zenoss-api-key" in context`)))
				})
			})

			Context("PutModels", func() {
				It("should return an error", func() {
					r, err := p.PutModels(ctx, &data_receiver.Models{})
					Ω(r).Should(BeNil())
					Ω(err).Should(Equal(
						status.Error(
							codes.Unauthenticated,
							`no "zenoss-api-key" in context`)))
				})
			})
		})

		Context("with invalid API key", func() {
			BeforeEach(func() {
				ctx = context.Background()
				md := metadata.Pairs("zenoss-api-key", "invalid")
				ctx = metadata.NewIncomingContext(ctx, md)
			})

			Context("PutMetrics", func() {
				It("should return an error", func() {
					r, err := p.PutMetrics(ctx, &data_receiver.Metrics{})
					Ω(r).Should(BeNil())
					Ω(err).Should(Equal(
						status.Error(
							codes.PermissionDenied,
							`unauthorized API key`)))
				})
			})

			Context("PutModels", func() {
				It("should return an error", func() {
					r, err := p.PutModels(ctx, &data_receiver.Models{})
					Ω(r).Should(BeNil())
					Ω(err).Should(Equal(
						status.Error(
							codes.PermissionDenied,
							`unauthorized API key`)))
				})
			})
		})

		Context("with valid API keys", func() {
			BeforeEach(func() {
				ctx = context.Background()
				md := metadata.Pairs("zenoss-api-key", "test")
				ctx = metadata.NewIncomingContext(ctx, md)
			})

			Context("PutMetrics", func() {
				It("should succeed", func() {
					out.On("PutMetrics", mock.Anything, mock.Anything).
						Return(&data_receiver.StatusResult{}, nil)

					r, err := p.PutMetrics(ctx, &data_receiver.Metrics{Metrics: []*data_receiver.Metric{{}}})
					Ω(r).ShouldNot(BeNil())
					Ω(err).ShouldNot(HaveOccurred())

					out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 1)
				})
			})

			Context("PutModels", func() {
				It("should succeed", func() {
					out.On("PutModels", mock.Anything, mock.Anything).
						Return(&data_receiver.ModelStatusResult{}, nil)

					r, err := p.PutModels(ctx, &data_receiver.Models{Models: []*data_receiver.Model{{}}})
					Ω(r).ShouldNot(BeNil())
					Ω(err).ShouldNot(HaveOccurred())

					out.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
				})
			})
		})
	})
})
