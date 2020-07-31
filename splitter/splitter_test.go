package splitter_test

import (
	"context"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/log"
	"github.com/zenoss/zenoss-go-sdk/splitter"
)

func TestSplitter(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Splitter Suite", []Reporter{junitReporter})
}

var _ = Describe("Splitter", func() {
	var logOutput *gbytes.Buffer
	var outs []data_receiver.DataReceiverServiceClient
	var out0 *data_receiver.MockDataReceiverServiceClient
	var out1 *data_receiver.MockDataReceiverServiceClient
	var s *splitter.Splitter
	var err error

	BeforeEach(func() {
		logOutput = gbytes.NewBuffer()
		stdlog.SetOutput(logOutput)

		out0 = &data_receiver.MockDataReceiverServiceClient{}
		out1 = &data_receiver.MockDataReceiverServiceClient{}
		outs = []data_receiver.DataReceiverServiceClient{out0, out1}
	})

	AfterEach(func() {
		stdlog.SetOutput(os.Stdout)
	})

	Context("with a complete configuration", func() {
		BeforeEach(func() {
			config := splitter.Config{
				Name:    "splitter1",
				Outputs: outs,
				LoggerConfig: log.LoggerConfig{
					Fields: log.Fields{"x": "X"},
					Level:  log.LevelDebug,
					Func: func(level log.Level, fields log.Fields, format string, args ...interface{}) {
						stdlog.Printf("level=%v fields=%v format=%v args=%v", level, fields, format, args)
					},
				},
			}

			s, err = splitter.New(config)
		})
		It("should be created without error", func() {
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("logs with a field", func() {
			log.Debug(s, log.Fields{"y": "Y"}, "one=%v", 1)
			Ω(logOutput).Should(gbytes.Say(`level=1 fields=map\[x:X y:Y\] format=one=%v args=\[1\]`))
		})

		Context("PutMetric", func() {
			It("is unimplemented", func() {
				c, err := s.PutMetric(nil)
				Ω(err).Should(HaveOccurred())
				Ω(err).Should(Equal(status.Error(codes.Unimplemented, "PutMetric is not supported")))
				Ω(c).Should(BeNil())
			})
		})

		Context("PutMetrics", func() {
			It("sends metrics to working outputs", func() {
				out0.On("PutMetrics", mock.Anything, mock.Anything).Return(nil, nil)
				out1.On("PutMetrics", mock.Anything, mock.Anything).Return(
					nil, status.Error(codes.Internal, "internal error"))

				r, err := s.PutMetrics(context.TODO(), &data_receiver.Metrics{
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

				// Each type of metric should be sent to each output in a separate call.
				out0.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
				out1.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 3)
			})
		})

		Context("PutModels", func() {
			It("sends models to all outputs", func() {
				out0.On("PutModels", mock.Anything, mock.Anything).Return(nil, nil)
				out1.On("PutModels", mock.Anything, mock.Anything).Return(
					nil, status.Error(codes.Internal, "internal error"))

				r, err := s.PutModels(context.TODO(), &data_receiver.Models{
					DetailedResponse: true,
					Models:           []*data_receiver.Model{{Timestamp: time.Now().Unix() / 1e6}},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())

				// Models should be sent to each output..
				out0.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
				out1.AssertNumberOfCalls(GinkgoT(), "PutModels", 1)
			})
		})

		Context("PutEvent", func() {
			It("is unimplemented", func() {
				c, err := s.PutEvent(nil)
				Ω(err).Should(HaveOccurred())
				Ω(err).Should(Equal(status.Error(codes.Unimplemented, "PutEvent is not supported")))
				Ω(c).Should(BeNil())
			})
		})

		Context("PutEvents", func() {
			It("sends events to all outputs", func() {
				out0.On("PutEvents", mock.Anything, mock.Anything).Return(nil, nil)
				out1.On("PutEvents", mock.Anything, mock.Anything).Return(
					nil, status.Error(codes.Internal, "internal error"))

				r, err := s.PutEvents(context.TODO(), &data_receiver.Events{
					DetailedResponse: true,
					Events:           []*data_receiver.Event{{Timestamp: time.Now().Unix() / 1e6}},
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(r).ShouldNot(BeNil())

				// Models should be sent to each output..
				out0.AssertNumberOfCalls(GinkgoT(), "PutEvents", 1)
				out1.AssertNumberOfCalls(GinkgoT(), "PutEvents", 1)
			})
		})
	})

	Context("with a minimal configuration", func() {
		BeforeEach(func() {
			config := splitter.Config{
				Outputs: outs,
			}

			s, err = splitter.New(config)
		})

		It("should be created without error", func() {
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should log with default fields", func() {
			log.Error(s, nil, "one=%v", 1)
			Ω(logOutput).Should(gbytes.Say(`one=1 fields=map\[splitter:default\]`))
		})
	})

	Context("with no outputs", func() {
		It("returns an error", func() {
			s, err = splitter.New(splitter.Config{})
			Ω(err).Should(MatchError("Config.Outputs is empty"))
		})
	})
})
