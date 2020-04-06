package processor_test

import (
	"context"
	stdlog "log"
	"math/rand"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/mock"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/log"
	"github.com/zenoss/zenoss-go-sdk/metadata"
	"github.com/zenoss/zenoss-go-sdk/processor"
)

func TestProcessor(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Processor Suite", []Reporter{junitReporter})
}

var _ = Describe("Processor", func() {
	var p *processor.Processor
	var ctx context.Context
	var out *data_receiver.MockDataReceiverServiceClient
	var err error
	var logOutput *gbytes.Buffer

	BeforeEach(func() {
		ctx = context.TODO()
		out = &data_receiver.MockDataReceiverServiceClient{}
		p, _ = processor.New(processor.Config{Output: out})

		logOutput = gbytes.NewBuffer()
		stdlog.SetOutput(logOutput)
	})

	AfterEach(func() {
		stdlog.SetOutput(os.Stdout)
	})

	Context("with no output", func() {
		BeforeEach(func() {
			p, err = processor.New(processor.Config{})
		})

		It("should return an error", func() {
			Ω(err).Should(MatchError("Config.Output is nil"))
		})
	})

	Context("with default configuration", func() {
		BeforeEach(func() {
			p, err = processor.New(processor.Config{Output: out})
		})

		Context("New", func() {
			It("shouldn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should return a processor", func() {
				Ω(p).ShouldNot(BeNil())
			})
		})

		Context("Logger", func() {
			It("logs error, warning, and info", func() {
				log.Error(p, log.Fields{"one": 1}, "an %s", "error")
				Ω(logOutput).Should(gbytes.Say(`an error fields=map\[one:1 processor:default\]`))

				log.Warning(p, log.Fields{"one": 1}, "a %s", "warning")
				Ω(logOutput).Should(gbytes.Say(`a warning fields=map\[one:1 processor:default\]`))

				log.Info(p, log.Fields{"one": 1}, "an %s", "info")
				Ω(logOutput).Should(gbytes.Say(`an info fields=map\[one:1 processor:default\]`))
			})

			It("doesn't log debug", func() {
				log.Debug(p, log.Fields{"one": 1}, "a %s", "debug")
				Ω(logOutput).ShouldNot(gbytes.Say(`a debug fields=map\[one:1 processor:default\]`))
			})
		})

		Context("PutMetric", func() {
			It("is unimplemented", func() {
				c, err := p.PutMetric(context.TODO())
				Ω(err).Should(MatchError(status.Error(codes.Unimplemented, "PutMetric is not supported")))
				Ω(c).Should(BeNil())
			})
		})

		Context("PutMetrics", func() {
			var metrics *data_receiver.Metrics
			var statusResult *data_receiver.StatusResult

			BeforeEach(func() {
				metrics = &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics:          []*data_receiver.Metric{{Metric: "m1"}, {Metric: "m2"}},
					CompactMetrics:   []*data_receiver.CompactMetric{{Id: "cm1"}, {Id: "cm2"}},
					TaggedMetrics:    []*data_receiver.TaggedMetric{{Metric: "tm1"}, {Metric: "tm2"}},
				}

				out.On("PutMetrics", ctx, &data_receiver.Metrics{
					DetailedResponse: true,
					CompactMetrics:   []*data_receiver.CompactMetric{{Id: "cm1"}, {Id: "cm2"}},
				}).Return(&data_receiver.StatusResult{Succeeded: 2}, nil)

				out.On("PutMetrics", ctx, mock.Anything).Return(nil, nil)

				statusResult, err = p.PutMetrics(ctx, metrics)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for all metric types", func() {
				Ω(statusResult.GetSucceeded()).Should(BeNumerically("==", 6))
				Ω(statusResult.GetFailed()).Should(BeNumerically("==", 0))
				Ω(len(statusResult.GetFailedCompactMetrics())).Should(Equal(0))
				Ω(len(statusResult.GetFailedTaggedMetrics())).Should(Equal(0))
				Ω(len(statusResult.GetFailedMetrics())).Should(Equal(0))
			})

			It("sends metrics to out", func() {
				// 2 for metrics, 2 for tagged metrics, 1 for compact metrics.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 5)
			})
		})

		Context("PutModels", func() {
			var models *data_receiver.Models
			var modelStatusResult *data_receiver.ModelStatusResult

			BeforeEach(func() {
				models = &data_receiver.Models{
					Models: []*data_receiver.Model{
						{
							Timestamp:      time.Now().UnixNano() / 1e6,
							Dimensions:     map[string]string{"source": "bob"},
							MetadataFields: metadata.FromStringMap(map[string]string{"name": "Bob"}),
						},
					},
				}

				out.On("PutModels", ctx, models).Return(nil, nil)
				modelStatusResult, err = p.PutModels(ctx, models)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for all models", func() {
				Ω(modelStatusResult.Succeeded).Should(BeNumerically("==", 1))
			})

			It("fails for no models", func() {
				Ω(modelStatusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(modelStatusResult.FailedModels)).Should(Equal(0))
			})

			It("sends models to out", func() {
				out.AssertExpectations(GinkgoT())
			})
		})
	})

	Context("with custom logging", func() {
		BeforeEach(func() {
			config := processor.Config{
				Name:   "processor1",
				Output: out,
				LoggerConfig: log.LoggerConfig{
					Fields: log.Fields{"x": "X"},
					Level:  log.LevelDebug,
					Func: func(level log.Level, fields log.Fields, format string, args ...interface{}) {
						stdlog.Printf("level=%v fields=%v format=%v args=%v", level, fields, format, args)
					},
				},
			}

			p, _ = processor.New(config)
		})

		It("logs with a field", func() {
			log.Debug(p, log.Fields{"y": "Y"}, "one=%v", 1)
			Ω(logOutput).Should(gbytes.Say(`level=1 fields=map\[x:X y:Y\] format=one=%v args=\[1\]`))
		})
	})

	Context("with a failing output", func() {
		BeforeEach(func() {
			p, _ = processor.New(processor.Config{Output: out})

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

				// One call for all compact metrics plus one call for each other metric.
				out.AssertNumberOfCalls(GinkgoT(), "PutMetrics", 5)
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

				// One call for each model.
				out.AssertNumberOfCalls(GinkgoT(), "PutModels", 2)
			})
		})
	})

	Context("with non-matching rules", func() {
		BeforeEach(func() {
			config := processor.Config{
				Output: out,
				MetricRules: []processor.MetricRuleConfig{
					{
						Matches: []processor.MetricMatchConfig{{Metric: "mX"}},
						Actions: []processor.MetricActionConfig{{Type: "drop"}},
					},
				},
				TaggedMetricRules: []processor.TaggedMetricRuleConfig{
					{
						Matches: []processor.TaggedMetricMatchConfig{{Metric: "tmX"}},
						Actions: []processor.TaggedMetricActionConfig{{Type: "drop"}},
					},
				},
				ModelRules: []processor.ModelRuleConfig{
					{
						Matches: []processor.ModelMatchConfig{{DimensionKeys: []string{"sourceX"}}},
						Actions: []processor.ModelActionConfig{{Type: "drop"}},
					},
				},
			}

			p, err = processor.New(config)
		})

		Context("PutMetrics", func() {
			var metrics *data_receiver.Metrics
			var statusResult *data_receiver.StatusResult

			BeforeEach(func() {
				metrics = &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics:          []*data_receiver.Metric{{Metric: "m1"}},
					TaggedMetrics:    []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}

				out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					Metrics: []*data_receiver.Metric{{Metric: "m1"}},
				}).Return(nil, nil)

				out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					TaggedMetrics: []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}).Return(nil, nil)

				statusResult, err = p.PutMetrics(context.TODO(), metrics)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for metric and tagged metric", func() {
				Ω(statusResult.Succeeded).Should(BeNumerically("==", 2))
				Ω(statusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(statusResult.FailedMetrics)).Should(Equal(0))
				Ω(len(statusResult.FailedTaggedMetrics)).Should(Equal(0))
			})

			It("sends metrics to output", func() {
				out.AssertExpectations(GinkgoT())
			})
		})

		Context("PutModels", func() {
			var models *data_receiver.Models
			var modelStatusResult *data_receiver.ModelStatusResult

			BeforeEach(func() {
				models = &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}

				out.On("PutModels", mock.Anything, &data_receiver.Models{
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}).Return(nil, nil)

				modelStatusResult, err = p.PutModels(context.TODO(), models)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for model", func() {
				Ω(modelStatusResult.Succeeded).Should(BeNumerically("==", 1))
				Ω(modelStatusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(modelStatusResult.FailedModels)).Should(Equal(0))
			})

			It("sends model to output", func() {
				out.AssertExpectations(GinkgoT())
			})
		})
	})

	Context("with invalid action types", func() {
		BeforeEach(func() {
			config := processor.Config{
				Output: out,
				MetricRules: []processor.MetricRuleConfig{{
					Actions: []processor.MetricActionConfig{{Type: "invalid"}},
				}},
				TaggedMetricRules: []processor.TaggedMetricRuleConfig{{
					Actions: []processor.TaggedMetricActionConfig{{Type: "invalid"}},
				}},
				ModelRules: []processor.ModelRuleConfig{{
					Actions: []processor.ModelActionConfig{{Type: "invalid"}},
				}},
			}

			p, err = processor.New(config)
		})

		Context("PutMetrics", func() {
			var metrics *data_receiver.Metrics
			var statusResult *data_receiver.StatusResult

			BeforeEach(func() {
				metrics = &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics:          []*data_receiver.Metric{{Metric: "m1"}},
					TaggedMetrics:    []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}

				out.On("PutMetrics", mock.Anything, mock.Anything).Return(nil, nil)
				statusResult, err = p.PutMetrics(context.TODO(), metrics)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("fails for metric and tagged metric", func() {
				Ω(statusResult.Succeeded).Should(BeNumerically("==", 0))
				Ω(statusResult.Failed).Should(BeNumerically("==", 2))

				Ω(len(statusResult.FailedMetrics)).Should(BeNumerically("==", 1))
				Ω(statusResult.FailedMetrics[0].Metric.Metric).Should(Equal("m1"))
				Ω(statusResult.FailedMetrics[0].Error).Should(Equal("unknown metric action type: default (invalid)"))

				Ω(len(statusResult.FailedTaggedMetrics)).Should(BeNumerically("==", 1))
				Ω(statusResult.FailedTaggedMetrics[0].Metric.Metric).Should(Equal("tm1"))
				Ω(statusResult.FailedTaggedMetrics[0].Error).Should(Equal("unknown tagged metric action type: default (invalid)"))
			})

			It("doesn't send metrics to output", func() {
				out.AssertNotCalled(GinkgoT(), "PutMetrics")
			})
		})

		Context("PutModels", func() {
			var models *data_receiver.Models
			var modelStatusResult *data_receiver.ModelStatusResult

			BeforeEach(func() {
				models = &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}

				out.On("PutModels", mock.Anything, mock.Anything).Return(nil, nil)
				modelStatusResult, err = p.PutModels(context.TODO(), models)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("fails for model", func() {
				Ω(modelStatusResult.Succeeded).Should(BeNumerically("==", 0))
				Ω(modelStatusResult.Failed).Should(BeNumerically("==", 1))

				Ω(len(modelStatusResult.FailedModels)).Should(BeNumerically("==", 1))
				Ω(modelStatusResult.FailedModels[0].Model.Dimensions["source"]).Should(Equal("bob"))
				Ω(modelStatusResult.FailedModels[0].Error).Should(Equal("unknown model action type: default (invalid)"))
			})

			It("doesn't send model to output", func() {
				out.AssertNotCalled(GinkgoT(), "PutModels")
			})
		})
	})

	Context("with drop action types", func() {
		BeforeEach(func() {
			config := processor.Config{
				Output: out,
				MetricRules: []processor.MetricRuleConfig{{
					Actions: []processor.MetricActionConfig{{Type: "drop"}},
				}},
				TaggedMetricRules: []processor.TaggedMetricRuleConfig{{
					Actions: []processor.TaggedMetricActionConfig{{Type: "drop"}},
				}},
				ModelRules: []processor.ModelRuleConfig{{
					Actions: []processor.ModelActionConfig{{Type: "drop"}},
				}},
			}

			p, err = processor.New(config)
		})

		Context("PutMetrics", func() {
			var metrics *data_receiver.Metrics
			var statusResult *data_receiver.StatusResult

			BeforeEach(func() {
				metrics = &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics:          []*data_receiver.Metric{{Metric: "m1"}},
					TaggedMetrics:    []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}

				out.On("PutMetrics", mock.Anything, mock.Anything).Return(nil, nil)
				statusResult, err = p.PutMetrics(context.TODO(), metrics)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for metric and tagged metric", func() {
				Ω(statusResult.Succeeded).Should(BeNumerically("==", 2))
				Ω(statusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(statusResult.FailedMetrics)).Should(Equal(0))
				Ω(len(statusResult.FailedTaggedMetrics)).Should(Equal(0))
			})

			It("doesn't send metrics to output", func() {
				out.AssertNotCalled(GinkgoT(), "PutMetrics")
			})
		})

		Context("PutModels", func() {
			var models *data_receiver.Models
			var modelStatusResult *data_receiver.ModelStatusResult

			BeforeEach(func() {
				models = &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}

				out.On("PutModels", mock.Anything, mock.Anything).Return(nil, nil)
				modelStatusResult, err = p.PutModels(context.TODO(), models)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for model", func() {
				Ω(modelStatusResult.Succeeded).Should(BeNumerically("==", 1))
				Ω(modelStatusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(modelStatusResult.FailedModels)).Should(Equal(0))
			})

			It("doesn't send model to output", func() {
				out.AssertNotCalled(GinkgoT(), "PutModels")
			})
		})
	})

	Context("with noop action types", func() {
		BeforeEach(func() {
			config := processor.Config{
				Output: out,
				MetricRules: []processor.MetricRuleConfig{{
					Actions: []processor.MetricActionConfig{{Type: "noop"}},
				}},
				TaggedMetricRules: []processor.TaggedMetricRuleConfig{{
					Actions: []processor.TaggedMetricActionConfig{{Type: "noop"}},
				}},
				ModelRules: []processor.ModelRuleConfig{{
					Actions: []processor.ModelActionConfig{{Type: "noop"}},
				}},
			}

			p, err = processor.New(config)
		})

		Context("PutMetrics", func() {
			var metrics *data_receiver.Metrics
			var statusResult *data_receiver.StatusResult

			BeforeEach(func() {
				metrics = &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics:          []*data_receiver.Metric{{Metric: "m1"}},
					TaggedMetrics:    []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}

				out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					Metrics: []*data_receiver.Metric{{Metric: "m1"}},
				}).Return(nil, nil)

				out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					TaggedMetrics: []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}).Return(nil, nil)

				statusResult, err = p.PutMetrics(context.TODO(), metrics)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for metric and tagged metric", func() {
				Ω(statusResult.Succeeded).Should(BeNumerically("==", 2))
				Ω(statusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(statusResult.FailedMetrics)).Should(Equal(0))
				Ω(len(statusResult.FailedTaggedMetrics)).Should(Equal(0))
			})

			It("sends metrics to output", func() {
				out.AssertExpectations(GinkgoT())
			})
		})

		Context("PutModels", func() {
			var models *data_receiver.Models
			var modelStatusResult *data_receiver.ModelStatusResult

			BeforeEach(func() {
				models = &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}

				out.On("PutModels", mock.Anything, &data_receiver.Models{
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}).Return(nil, nil)

				modelStatusResult, err = p.PutModels(context.TODO(), models)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for model", func() {
				Ω(modelStatusResult.Succeeded).Should(BeNumerically("==", 1))
				Ω(modelStatusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(modelStatusResult.FailedModels)).Should(Equal(0))
			})

			It("sends model to output", func() {
				out.AssertExpectations(GinkgoT())
			})
		})
	})

	Context("with send action types", func() {
		BeforeEach(func() {
			config := processor.Config{
				Output: out,
				MetricRules: []processor.MetricRuleConfig{{
					Actions: []processor.MetricActionConfig{
						{Type: "send"},
						{Type: "drop"},
					},
				}},
				TaggedMetricRules: []processor.TaggedMetricRuleConfig{{
					Actions: []processor.TaggedMetricActionConfig{
						{Type: "send"},
						{Type: "drop"},
					},
				}},
				ModelRules: []processor.ModelRuleConfig{{
					Actions: []processor.ModelActionConfig{
						{Type: "send"},
						{Type: "drop"},
					},
				}},
			}

			p, err = processor.New(config)
		})

		Context("PutMetrics", func() {
			var metrics *data_receiver.Metrics
			var statusResult *data_receiver.StatusResult

			BeforeEach(func() {
				metrics = &data_receiver.Metrics{
					DetailedResponse: true,
					Metrics:          []*data_receiver.Metric{{Metric: "m1"}},
					TaggedMetrics:    []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}

				out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					Metrics: []*data_receiver.Metric{{Metric: "m1"}},
				}).Return(nil, nil)

				out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
					TaggedMetrics: []*data_receiver.TaggedMetric{{Metric: "tm1"}},
				}).Return(nil, nil)

				statusResult, err = p.PutMetrics(context.TODO(), metrics)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for metric and tagged metric", func() {
				Ω(statusResult.Succeeded).Should(BeNumerically("==", 2))
				Ω(statusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(statusResult.FailedMetrics)).Should(Equal(0))
				Ω(len(statusResult.FailedTaggedMetrics)).Should(Equal(0))
			})

			It("sends metrics to output", func() {
				out.AssertExpectations(GinkgoT())
			})
		})

		Context("PutModels", func() {
			var models *data_receiver.Models
			var modelStatusResult *data_receiver.ModelStatusResult

			BeforeEach(func() {
				models = &data_receiver.Models{
					DetailedResponse: true,
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}

				out.On("PutModels", mock.Anything, &data_receiver.Models{
					Models: []*data_receiver.Model{{
						Dimensions: map[string]string{"source": "bob"},
					}},
				}).Return(nil, nil)

				modelStatusResult, err = p.PutModels(context.TODO(), models)
			})

			It("doesn't result in an error", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("succeeds for model", func() {
				Ω(modelStatusResult.Succeeded).Should(BeNumerically("==", 1))
				Ω(modelStatusResult.Failed).Should(BeNumerically("==", 0))
				Ω(len(modelStatusResult.FailedModels)).Should(Equal(0))
			})

			It("sends model to output", func() {
				out.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("Rules", func() {
		Context("MetricRuleConfig", func() {
			var r processor.MetricRuleConfig

			It("is named 'default' by default", func() {
				r = processor.MetricRuleConfig{}
				Ω(r.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				r = processor.MetricRuleConfig{Name: "test"}
				Ω(r.GetName()).Should(Equal("test"))
			})

			Context("Apply", func() {
				It("applies action with implicit match", func() {
					r = processor.MetricRuleConfig{
						Actions: []processor.MetricActionConfig{{Type: "drop"}}}

					err = r.Apply(context.TODO(), nil, &data_receiver.Metric{Metric: "test"})
					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})

				It("applies action with explicit match", func() {
					r = processor.MetricRuleConfig{
						Matches: []processor.MetricMatchConfig{{Metric: "test"}},
						Actions: []processor.MetricActionConfig{{Type: "drop"}}}

					err = r.Apply(context.TODO(), nil, &data_receiver.Metric{Metric: "test"})
					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})

				It("applies action with nested match", func() {
					r = processor.MetricRuleConfig{
						Matches: []processor.MetricMatchConfig{
							{DimensionKeys: []string{"source"}},
						},
						MetricRules: []processor.MetricRuleConfig{
							{
								Matches: []processor.MetricMatchConfig{
									{Metric: "test"},
								},
								Actions: []processor.MetricActionConfig{
									{Type: "drop"},
								},
							},
						},
					}

					err = r.Apply(context.TODO(), nil, &data_receiver.Metric{
						Metric:     "test",
						Dimensions: map[string]string{"source": "test"}})

					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})
			})
		})

		Context("TaggedMetricRuleConfig", func() {
			var r processor.TaggedMetricRuleConfig

			It("is named 'default' by default", func() {
				r = processor.TaggedMetricRuleConfig{}
				Ω(r.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				r = processor.TaggedMetricRuleConfig{Name: "test"}
				Ω(r.GetName()).Should(Equal("test"))
			})

			Context("Apply", func() {
				It("applies action with implicit match", func() {
					r = processor.TaggedMetricRuleConfig{
						Actions: []processor.TaggedMetricActionConfig{{Type: "drop"}}}

					err = r.Apply(context.TODO(), nil, &data_receiver.TaggedMetric{Metric: "test"})
					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})

				It("applies action with explicit match", func() {
					r = processor.TaggedMetricRuleConfig{
						Matches: []processor.TaggedMetricMatchConfig{{Metric: "test"}},
						Actions: []processor.TaggedMetricActionConfig{{Type: "drop"}}}

					err = r.Apply(context.TODO(), nil, &data_receiver.TaggedMetric{Metric: "test"})
					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})

				It("applies action with nested match", func() {
					r = processor.TaggedMetricRuleConfig{
						Matches: []processor.TaggedMetricMatchConfig{
							{TagKeys: []string{"source"}},
						},
						TaggedMetricRules: []processor.TaggedMetricRuleConfig{
							{
								Matches: []processor.TaggedMetricMatchConfig{
									{Metric: "test"},
								},
								Actions: []processor.TaggedMetricActionConfig{
									{Type: "drop"},
								},
							},
						},
					}

					err = r.Apply(context.TODO(), nil, &data_receiver.TaggedMetric{
						Metric: "test",
						Tags:   map[string]string{"source": "test"}})

					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})
			})
		})
		Context("ModelRuleConfig", func() {
			var r processor.ModelRuleConfig

			It("is named 'default' by default", func() {
				r = processor.ModelRuleConfig{}
				Ω(r.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				r = processor.ModelRuleConfig{Name: "test"}
				Ω(r.GetName()).Should(Equal("test"))
			})

			Context("Apply", func() {
				It("applies action with implicit match", func() {
					r = processor.ModelRuleConfig{
						Actions: []processor.ModelActionConfig{{Type: "drop"}}}

					err = r.Apply(context.TODO(), nil, &data_receiver.Model{
						Dimensions: map[string]string{"source": "test"}})

					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})

				It("applies action with explicit match", func() {
					r = processor.ModelRuleConfig{
						Matches: []processor.ModelMatchConfig{{
							DimensionKeys: []string{"source"}}},
						Actions: []processor.ModelActionConfig{{Type: "drop"}}}

					err = r.Apply(context.TODO(), nil, &data_receiver.Model{
						Dimensions: map[string]string{"source": "test"}})

					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})

				It("applies action with nested match", func() {
					r = processor.ModelRuleConfig{
						Matches: []processor.ModelMatchConfig{
							{DimensionKeys: []string{"source"}},
						},
						ModelRules: []processor.ModelRuleConfig{
							{
								Matches: []processor.ModelMatchConfig{
									{DimensionKeys: []string{"app"}},
								},
								Actions: []processor.ModelActionConfig{
									{Type: "drop"},
								},
							},
						},
					}

					err = r.Apply(context.TODO(), nil, &data_receiver.Model{
						Dimensions: map[string]string{
							"source": "test",
							"app":    "test"}})

					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})
			})
		})
	})

	Describe("Matches", func() {
		Context("MetricMatchConfig", func() {
			It("is named 'default' by default", func() {
				m := processor.MetricMatchConfig{}
				Ω(m.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				m := processor.MetricMatchConfig{Name: "test"}
				Ω(m.GetName()).Should(Equal("test"))
			})

			DescribeTable("MatchesMetric (match)",
				func(match processor.MetricMatchConfig, metric data_receiver.Metric) {
					Ω(match.MatchesMetric(&metric)).Should(BeTrue())
				},
				Entry("all criteria",
					processor.MetricMatchConfig{
						Metric:         "test",
						DimensionKeys:  []string{"test"},
						Dimensions:     map[string]string{"test": "value"},
						MetadataKeys:   []string{"test"},
						MetadataFields: map[string]string{"test": "value"},
					},
					data_receiver.Metric{
						Metric:         "test",
						Dimensions:     map[string]string{"test": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"}),
					},
				),
				Entry("no criteria",
					processor.MetricMatchConfig{},
					data_receiver.Metric{},
				),
				Entry("exact metric name",
					processor.MetricMatchConfig{Metric: "test"},
					data_receiver.Metric{Metric: "test"},
				),
				Entry("exact dimensionKeys",
					processor.MetricMatchConfig{DimensionKeys: []string{"test"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("partial dimensionKeys",
					processor.MetricMatchConfig{DimensionKeys: []string{"test"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value", "testX": "valueX"}},
				),
				Entry("exact dimensions",
					processor.MetricMatchConfig{Dimensions: map[string]string{"test": "value"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("partial dimensions",
					processor.MetricMatchConfig{Dimensions: map[string]string{"test": "value"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value", "testX": "valueX"}},
				),
				Entry("exact metadataKeys",
					processor.MetricMatchConfig{MetadataKeys: []string{"test"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("partial metadataKeys",
					processor.MetricMatchConfig{MetadataKeys: []string{"test"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value", "testX": "valueX"})},
				),
				Entry("exact metadataFields",
					processor.MetricMatchConfig{MetadataFields: map[string]string{"test": "value"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("partial metadataFields",
					processor.MetricMatchConfig{MetadataFields: map[string]string{"test": "value"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value", "testX": "valueX"})},
				),
			)

			DescribeTable("MatchesMetric (no match)",
				func(match processor.MetricMatchConfig, metric data_receiver.Metric) {
					Ω(match.MatchesMetric(&metric)).Should(BeFalse())
				},
				Entry("some criteria",
					processor.MetricMatchConfig{
						Metric:         "test",
						DimensionKeys:  []string{"test"},
						Dimensions:     map[string]string{"test": "value"},
						MetadataKeys:   []string{"test"},
						MetadataFields: map[string]string{"test": "valueX"},
					},
					data_receiver.Metric{
						Metric:         "test",
						Dimensions:     map[string]string{"test": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"}),
					},
				),
				Entry("metric name that doesn't match",
					processor.MetricMatchConfig{Metric: "testX"},
					data_receiver.Metric{Metric: "test"},
				),
				Entry("dimensionKeys with empty metric dimensions",
					processor.MetricMatchConfig{DimensionKeys: []string{"test"}},
					data_receiver.Metric{},
				),
				Entry("dimensionKeys that don't match",
					processor.MetricMatchConfig{DimensionKeys: []string{"testX"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensionKeys that partially match",
					processor.MetricMatchConfig{DimensionKeys: []string{"test", "testX"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensions with empty metric dimensions",
					processor.MetricMatchConfig{Dimensions: map[string]string{"test": "value"}},
					data_receiver.Metric{},
				),
				Entry("dimensions that don't match by key",
					processor.MetricMatchConfig{Dimensions: map[string]string{"testX": "value"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensions that don't match by value",
					processor.MetricMatchConfig{Dimensions: map[string]string{"test": "valueX"}},
					data_receiver.Metric{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensions that partially match",
					processor.MetricMatchConfig{Dimensions: map[string]string{"test1": "value1", "test2": "valueX"}},
					data_receiver.Metric{Dimensions: map[string]string{"test1": "value1", "test2": "value2"}},
				),
				Entry("metadataKeys with empty metric metadataFields",
					processor.MetricMatchConfig{MetadataKeys: []string{"test"}},
					data_receiver.Metric{},
				),
				Entry("metadataKeys that don't match",
					processor.MetricMatchConfig{MetadataKeys: []string{"testX"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataKeys that partially match",
					processor.MetricMatchConfig{MetadataKeys: []string{"test", "testX"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataFields with empty metric metadataFields",
					processor.MetricMatchConfig{MetadataFields: map[string]string{"test": "value"}},
					data_receiver.Metric{},
				),
				Entry("metadataFields that don't match by key",
					processor.MetricMatchConfig{MetadataFields: map[string]string{"testX": "value"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataFields that don't match by value",
					processor.MetricMatchConfig{MetadataFields: map[string]string{"test": "valueX"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataFields that partially match",
					processor.MetricMatchConfig{MetadataFields: map[string]string{"test1": "value1", "test2": "valueX"}},
					data_receiver.Metric{MetadataFields: metadata.FromStringMap(map[string]string{"test1": "value1", "test2": "value2"})},
				),
			)
		})

		Context("TaggedMetricMatchConfig", func() {
			It("is named 'default' by default", func() {
				m := processor.TaggedMetricMatchConfig{}
				Ω(m.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				m := processor.TaggedMetricMatchConfig{Name: "test"}
				Ω(m.GetName()).Should(Equal("test"))
			})

			DescribeTable("MatchesTaggedMetric (match)",
				func(match processor.TaggedMetricMatchConfig, taggedMetric data_receiver.TaggedMetric) {
					Ω(match.MatchesTaggedMetric(&taggedMetric)).Should(BeTrue())
				},
				Entry("all criteria",
					processor.TaggedMetricMatchConfig{
						Metric:  "test",
						TagKeys: []string{"test"},
						Tags:    map[string]string{"test": "value"},
					},
					data_receiver.TaggedMetric{
						Metric: "test",
						Tags:   map[string]string{"test": "value"},
					},
				),
				Entry("no criteria",
					processor.TaggedMetricMatchConfig{},
					data_receiver.TaggedMetric{},
				),
				Entry("exact metric name",
					processor.TaggedMetricMatchConfig{Metric: "test"},
					data_receiver.TaggedMetric{Metric: "test"},
				),
				Entry("exact tagKeys",
					processor.TaggedMetricMatchConfig{TagKeys: []string{"test"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value"}},
				),
				Entry("partial tagKeys",
					processor.TaggedMetricMatchConfig{TagKeys: []string{"test"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value", "testX": "valueX"}},
				),
				Entry("exact tags",
					processor.TaggedMetricMatchConfig{Tags: map[string]string{"test": "value"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value"}},
				),
				Entry("partial tags",
					processor.TaggedMetricMatchConfig{Tags: map[string]string{"test": "value"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value", "testX": "valueX"}},
				),
			)

			DescribeTable("MatchesTaggedMetric (no match)",
				func(match processor.TaggedMetricMatchConfig, taggedMetric data_receiver.TaggedMetric) {
					Ω(match.MatchesTaggedMetric(&taggedMetric)).Should(BeFalse())
				},
				Entry("some criteria",
					processor.TaggedMetricMatchConfig{
						Metric:  "test",
						TagKeys: []string{"test"},
						Tags:    map[string]string{"test": "valueX"},
					},
					data_receiver.TaggedMetric{
						Metric: "test",
						Tags:   map[string]string{"test": "value"},
					},
				),
				Entry("metric name that doesn't match",
					processor.TaggedMetricMatchConfig{Metric: "testX"},
					data_receiver.TaggedMetric{Metric: "test"},
				),
				Entry("tagKeys with empty metric tags",
					processor.TaggedMetricMatchConfig{TagKeys: []string{"test"}},
					data_receiver.TaggedMetric{},
				),
				Entry("tagKeys that don't match",
					processor.TaggedMetricMatchConfig{TagKeys: []string{"testX"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value"}},
				),
				Entry("tagKeys that partially match",
					processor.TaggedMetricMatchConfig{TagKeys: []string{"test", "testX"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value"}},
				),
				Entry("tags with empty metric tags",
					processor.TaggedMetricMatchConfig{Tags: map[string]string{"test": "value"}},
					data_receiver.TaggedMetric{},
				),
				Entry("tags that don't match by key",
					processor.TaggedMetricMatchConfig{Tags: map[string]string{"testX": "value"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value"}},
				),
				Entry("tags that don't match by value",
					processor.TaggedMetricMatchConfig{Tags: map[string]string{"test": "valueX"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test": "value"}},
				),
				Entry("tags that partially match",
					processor.TaggedMetricMatchConfig{Tags: map[string]string{"test1": "value1", "test2": "valueX"}},
					data_receiver.TaggedMetric{Tags: map[string]string{"test1": "value1", "test2": "value2"}},
				),
			)
		})

		Context("ModelMatchConfig", func() {
			It("is named 'default' by default", func() {
				m := processor.ModelMatchConfig{}
				Ω(m.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				m := processor.ModelMatchConfig{Name: "test"}
				Ω(m.GetName()).Should(Equal("test"))
			})

			DescribeTable("MatchesModel (match)",
				func(match processor.ModelMatchConfig, model data_receiver.Model) {
					Ω(match.MatchesModel(&model)).Should(BeTrue())
				},
				Entry("all criteria",
					processor.ModelMatchConfig{
						DimensionKeys:  []string{"test"},
						Dimensions:     map[string]string{"test": "value"},
						MetadataKeys:   []string{"test"},
						MetadataFields: map[string]string{"test": "value"},
					},
					data_receiver.Model{
						Dimensions:     map[string]string{"test": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"}),
					},
				),
				Entry("no criteria",
					processor.ModelMatchConfig{},
					data_receiver.Model{},
				),
				Entry("exact dimensionKeys",
					processor.ModelMatchConfig{DimensionKeys: []string{"test"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("partial dimensionKeys",
					processor.ModelMatchConfig{DimensionKeys: []string{"test"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value", "testX": "valueX"}},
				),
				Entry("exact dimensions",
					processor.ModelMatchConfig{Dimensions: map[string]string{"test": "value"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("partial dimensions",
					processor.ModelMatchConfig{Dimensions: map[string]string{"test": "value"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value", "testX": "valueX"}},
				),
				Entry("exact metadataKeys",
					processor.ModelMatchConfig{MetadataKeys: []string{"test"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("partial metadataKeys",
					processor.ModelMatchConfig{MetadataKeys: []string{"test"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value", "testX": "valueX"})},
				),
				Entry("exact metadataFields",
					processor.ModelMatchConfig{MetadataFields: map[string]string{"test": "value"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("partial metadataFields",
					processor.ModelMatchConfig{MetadataFields: map[string]string{"test": "value"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value", "testX": "valueX"})},
				),
			)

			DescribeTable("MatchesModel (no match)",
				func(match processor.ModelMatchConfig, model data_receiver.Model) {
					Ω(match.MatchesModel(&model)).Should(BeFalse())
				},
				Entry("some criteria",
					processor.ModelMatchConfig{
						DimensionKeys:  []string{"test"},
						Dimensions:     map[string]string{"test": "value"},
						MetadataKeys:   []string{"test"},
						MetadataFields: map[string]string{"test": "valueX"},
					},
					data_receiver.Model{
						Dimensions:     map[string]string{"test": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"}),
					},
				),
				Entry("dimensionKeys with empty metric dimensions",
					processor.ModelMatchConfig{DimensionKeys: []string{"test"}},
					data_receiver.Model{},
				),
				Entry("dimensionKeys that don't match",
					processor.ModelMatchConfig{DimensionKeys: []string{"testX"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensionKeys that partially match",
					processor.ModelMatchConfig{DimensionKeys: []string{"test", "testX"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensions with empty metric dimensions",
					processor.ModelMatchConfig{Dimensions: map[string]string{"test": "value"}},
					data_receiver.Model{},
				),
				Entry("dimensions that don't match by key",
					processor.ModelMatchConfig{Dimensions: map[string]string{"testX": "value"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensions that don't match by value",
					processor.ModelMatchConfig{Dimensions: map[string]string{"test": "valueX"}},
					data_receiver.Model{Dimensions: map[string]string{"test": "value"}},
				),
				Entry("dimensions that partially match",
					processor.ModelMatchConfig{Dimensions: map[string]string{"test1": "value1", "test2": "valueX"}},
					data_receiver.Model{Dimensions: map[string]string{"test1": "value1", "test2": "value2"}},
				),
				Entry("metadataKeys with empty metric metadataFields",
					processor.ModelMatchConfig{MetadataKeys: []string{"test"}},
					data_receiver.Model{},
				),
				Entry("metadataKeys that don't match",
					processor.ModelMatchConfig{MetadataKeys: []string{"testX"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataKeys that partially match",
					processor.ModelMatchConfig{MetadataKeys: []string{"test", "testX"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataFields with empty metric metadataFields",
					processor.ModelMatchConfig{MetadataFields: map[string]string{"test": "value"}},
					data_receiver.Model{},
				),
				Entry("metadataFields that don't match by key",
					processor.ModelMatchConfig{MetadataFields: map[string]string{"testX": "value"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataFields that don't match by value",
					processor.ModelMatchConfig{MetadataFields: map[string]string{"test": "valueX"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"})},
				),
				Entry("metadataFields that partially match",
					processor.ModelMatchConfig{MetadataFields: map[string]string{"test1": "value1", "test2": "valueX"}},
					data_receiver.Model{MetadataFields: metadata.FromStringMap(map[string]string{"test1": "value1", "test2": "value2"})},
				),
			)
		})
	})

	Describe("Actions", func() {
		Context("MetricActionConfig", func() {
			var action processor.MetricActionConfig
			var metric *data_receiver.Metric

			It("is named 'default' by default", func() {
				action = processor.MetricActionConfig{}
				Ω(action.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				action = processor.MetricActionConfig{Name: "test"}
				Ω(action.GetName()).Should(Equal("test"))
			})

			Context("Apply (noop)", func() {
				BeforeEach(func() {
					action = processor.MetricActionConfig{Type: processor.ActionTypeNoop}
					metric = &data_receiver.Metric{}
					err = action.Apply(ctx, p, metric)
				})

				It("doesn't result in an error", func() {
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("shouldn't have modified the metric", func() {
					Ω(metric).Should(Equal(&data_receiver.Metric{}))
				})
			})

			Context("Apply (log)", func() {
				BeforeEach(func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeLog,
						Name: "test"}

					metric = &data_receiver.Metric{
						Metric:     "test",
						Dimensions: map[string]string{"test": "value"},
						MetadataFields: metadata.MustFromMap(map[string]interface{}{
							"string":        "value",
							"listOfStrings": []string{"value"},
						}),
					}

					err = action.Apply(ctx, p, metric)
				})

				It("doesn't result in an error", func() {
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("logs the metric", func() {
					Ω(logOutput).Should(gbytes.Say(`Metric{Metric: "test", Dimensions: map\[test:value\], MetadataFields: map\[listOfStrings:\[value\] string:value\]} fields=map\[action:test processor:default\]`))
				})
			})

			Context("Apply (drop)", func() {
				BeforeEach(func() {
					action = processor.MetricActionConfig{Type: processor.ActionTypeDrop}
					err = action.Apply(ctx, p, &data_receiver.Metric{})
				})

				It("returns the drop signal", func() {
					Ω(err).Should(Equal(&processor.SignalDrop{}))
					Ω(err.Error()).Should(Equal("drop"))
				})
			})

			Context("Apply (send)", func() {
				BeforeEach(func() {
					action = processor.MetricActionConfig{Type: processor.ActionTypeSend}
					err = action.Apply(ctx, p, &data_receiver.Metric{})
				})

				It("returns the send signal", func() {
					Ω(err).Should(MatchError(&processor.SignalSend{}))
					Ω(err.Error()).Should(Equal("send"))
				})
			})

			Context("Apply (append-to-metadata-field)", func() {
				It("fails with invalid options", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: map[string]interface{}{
							"key": 0}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(MatchRegexp(`expected type 'string', got unconvertible type 'int'`))
				})

				It("fails without a key", func() {
					action = processor.MetricActionConfig{
						Type:    processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(MatchError(`missing "key"`))
				})

				It("fails with valueTemplate parse error", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":           "test",
							"valueTemplate": "{{{invalid"}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(MatchError(`line 1: unmatched open tag`))
				})

				It("fails with valueTemplate render error", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":           "test",
							"valueTemplate": "{{{dimensionX}}}/{{{metadataX}}}"}}

					metric = &data_receiver.Metric{
						Dimensions:     map[string]string{"dimension": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"metadata": "value"}),
					}

					err = action.Apply(ctx, p, metric)
					Ω(err).Should(MatchError(`Missing variable "dimensionX"`))
				})

				It("fails without value or valueTemplate", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key": "test"}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(MatchError(`missing "value" or "valueTemplate"`))
				})

				It("fails with value and valueTemplate", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":           "test",
							"value":         "value",
							"valueTemplate": "{{{value}}}"}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(MatchError(`setting "value" and "valueTemplate" is ambiguous`))
				})

				It("fails when metadata field isn't a list", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":   "test",
							"value": "value"}}

					err = action.Apply(ctx, p, &data_receiver.Metric{
						MetadataFields: metadata.FromStringMap(
							map[string]string{"test": "value"})})

					Ω(err).Should(MatchError(`unable to append "test" field: field is not a list`))
				})

				It("succeeds when metadata field was empty", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":   "test",
							"value": "value"}}

					metric = &data_receiver.Metric{}

					err = action.Apply(ctx, p, metric)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(metric.GetMetadataFields()).Should(Equal(
						metadata.MustFromMap(map[string]interface{}{
							"test": []string{"value"}})))
				})

				It("succeeds when metadata field was a list", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":   "test",
							"value": "value2"}}

					metric = &data_receiver.Metric{
						MetadataFields: metadata.MustFromMap(map[string]interface{}{
							"test": []string{"value1"}})}

					err = action.Apply(ctx, p, metric)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(metric.GetMetadataFields()).Should(Equal(
						metadata.MustFromMap(map[string]interface{}{
							"test": []string{"value1", "value2"}})))
				})

				It("succeeds with valueTemplate", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":           "test",
							"valueTemplate": "{{{dimension1}}}/{{{metadata1}}}"}}

					metric = &data_receiver.Metric{
						Dimensions:     map[string]string{"dimension1": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"metadata1": "value"})}

					err = action.Apply(ctx, p, metric)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(metric.GetMetadataFields()).Should(Equal(
						metadata.MustFromMap(map[string]interface{}{
							"metadata1": "value",
							"test":      []string{"value/value"}})))
				})
			})

			Context("Apply (create-model)", func() {
				It("fails with invalid options", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeCreateModel,
						Options: map[string]interface{}{
							"nameTemplate": 0}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(MatchRegexp(`expected type 'string', got unconvertible type 'int'`))
				})

				It("fails with nameTemplate parse error", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeCreateModel,
						Options: processor.ActionOptions{
							"nameTemplate": "{{{invalid"}}

					err = action.Apply(ctx, p, &data_receiver.Metric{})
					Ω(err).Should(MatchError(`line 1: unmatched open tag`))
				})

				It("fails with nameTemplate render error", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeCreateModel,
						Options: processor.ActionOptions{
							"nameTemplate": "{{{dimensionX}}}/{{{metadataX}}}"}}

					metric = &data_receiver.Metric{
						Dimensions:     map[string]string{"dimension": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"metadata": "value"}),
					}

					err = action.Apply(ctx, p, metric)
					Ω(err).Should(MatchError(`Missing variable "dimensionX"`))
				})

				It("succeeds with defaults", func() {
					out.On("PutModels", mock.Anything, mock.Anything).Return(nil, nil)
					action = processor.MetricActionConfig{Type: processor.ActionTypeCreateModel}
					metric = &data_receiver.Metric{}

					err = action.Apply(ctx, p, metric)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(metric).Should(Equal(&data_receiver.Metric{}))
				})

				It("creates model with defaults", func() {
					action = processor.MetricActionConfig{Type: processor.ActionTypeCreateModel}
					metric = &data_receiver.Metric{
						Dimensions:     map[string]string{"d1": "dim1"},
						MetadataFields: metadata.FromStringMap(map[string]string{"m1": "met1"}),
					}

					out.On("PutModels", mock.Anything, &data_receiver.Models{
						Models: []*data_receiver.Model{{
							Dimensions:     map[string]string{"d1": "dim1"},
							MetadataFields: metadata.FromStringMap(map[string]string{"m1": "met1"}),
						}},
					}).Return(nil, nil)

					_ = action.Apply(ctx, p, metric)
					out.AssertExpectations(GinkgoT())
				})

				It("creates model with nameTemplate", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeCreateModel,
						Options: map[string]interface{}{
							"nameTemplate": "{{{string}}}/{{{number}}}/{{{bool}}}",
						},
					}

					metric = &data_receiver.Metric{
						Dimensions: map[string]string{"d1": "dim1"},
						MetadataFields: metadata.MustFromMap(map[string]interface{}{
							"string": "value",
							"number": 123.4,
							"bool":   true,
							"other":  nil,
						}),
					}

					out.On("PutModels", mock.Anything, &data_receiver.Models{
						Models: []*data_receiver.Model{{
							Dimensions: map[string]string{"d1": "dim1"},
							MetadataFields: metadata.MustFromMap(map[string]interface{}{
								"name":   "value/123.4/true",
								"string": "value",
								"number": 123.4,
								"bool":   true,
								"other":  nil,
							}),
						}},
					}).Return(nil, nil)

					err = action.Apply(ctx, p, metric)
					Ω(err).ShouldNot(HaveOccurred())
					out.AssertExpectations(GinkgoT())
				})

				It("creates model with dropMetadataKeys", func() {
					action = processor.MetricActionConfig{
						Type: processor.ActionTypeCreateModel,
						Options: map[string]interface{}{
							"dropMetadataKeys": []string{"units"},
						},
					}

					metric = &data_receiver.Metric{
						Dimensions: map[string]string{"d1": "dim1"},
						MetadataFields: metadata.FromStringMap(map[string]string{
							"m1":    "met1",
							"units": "barrels",
						}),
					}

					out.On("PutModels", mock.Anything, &data_receiver.Models{
						Models: []*data_receiver.Model{{
							Dimensions: map[string]string{"d1": "dim1"},
							MetadataFields: metadata.FromStringMap(map[string]string{
								"m1": "met1",
							}),
						}},
					}).Return(nil, nil)

					_ = action.Apply(ctx, p, metric)
					out.AssertExpectations(GinkgoT())
				})
			})
		})

		// TODO: Test TaggedMetricActionConfig.

		Context("TaggedMetricActionConfig", func() {
			var action processor.TaggedMetricActionConfig
			var taggedMetric *data_receiver.TaggedMetric

			It("is named 'default' by default", func() {
				action = processor.TaggedMetricActionConfig{}
				Ω(action.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				action = processor.TaggedMetricActionConfig{Name: "test"}
				Ω(action.GetName()).Should(Equal("test"))
			})

			Context("Apply (noop)", func() {
				BeforeEach(func() {
					action = processor.TaggedMetricActionConfig{Type: processor.ActionTypeNoop}
					taggedMetric = &data_receiver.TaggedMetric{}
					err = action.Apply(ctx, p, taggedMetric)
				})

				It("doesn't result in an error", func() {
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("shouldn't have modified the tagged metric", func() {
					Ω(taggedMetric).Should(Equal(&data_receiver.TaggedMetric{}))
				})
			})

			Context("Apply (log)", func() {
				BeforeEach(func() {
					action = processor.TaggedMetricActionConfig{
						Type: processor.ActionTypeLog,
						Name: "test"}

					taggedMetric = &data_receiver.TaggedMetric{
						Metric: "test",
						Tags:   map[string]string{"test": "value"},
					}

					err = action.Apply(ctx, p, taggedMetric)
				})

				It("doesn't result in an error", func() {
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("logs the metric", func() {
					Ω(logOutput).Should(gbytes.Say(`TaggedMetric{Metric: "test", Tags: map\[test:value\]} fields=map\[action:test processor:default\]`))
				})
			})

			Context("Apply (drop)", func() {
				BeforeEach(func() {
					action = processor.TaggedMetricActionConfig{Type: processor.ActionTypeDrop}
					err = action.Apply(ctx, p, &data_receiver.TaggedMetric{})
				})

				It("returns the drop signal", func() {
					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})
			})

			Context("Apply (send)", func() {
				BeforeEach(func() {
					action = processor.TaggedMetricActionConfig{Type: processor.ActionTypeSend}
					err = action.Apply(ctx, p, &data_receiver.TaggedMetric{})
				})

				It("returns the send signal", func() {
					Ω(err).Should(MatchError(&processor.SignalSend{}))
				})
			})

			Context("Apply (copy-to-metric)", func() {
				It("fails with invalid options", func() {
					action = processor.TaggedMetricActionConfig{
						Type: processor.ActionTypeCopyToMetric,
						Options: map[string]interface{}{
							"metadataKeys": "test"}}

					err = action.Apply(ctx, p, &data_receiver.TaggedMetric{})
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(MatchRegexp(`source data must be an array or slice, got string`))
				})

				It("succeeds with defaults", func() {
					out.On("PutMetrics", mock.Anything, mock.Anything).Return(nil, nil)
					action = processor.TaggedMetricActionConfig{Type: processor.ActionTypeCopyToMetric}
					taggedMetric = &data_receiver.TaggedMetric{}

					err = action.Apply(ctx, p, taggedMetric)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(taggedMetric).Should(Equal(&data_receiver.TaggedMetric{}))
				})

				It("creates metric with defaults", func() {
					action = processor.TaggedMetricActionConfig{Type: processor.ActionTypeCopyToMetric}
					taggedMetric = &data_receiver.TaggedMetric{
						Metric:    "test",
						Timestamp: 77,
						Value:     123.4,
						Tags:      map[string]string{"test": "value"},
					}

					out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
						Metrics: []*data_receiver.Metric{{
							Metric:         "test",
							Timestamp:      77,
							Value:          123.4,
							Dimensions:     map[string]string{"test": "value"},
							MetadataFields: metadata.FromStringMap(nil),
						}},
					}).Return(nil, nil)

					_ = action.Apply(ctx, p, taggedMetric)
					out.AssertExpectations(GinkgoT())
				})

				It("creates metric with metadataKeys", func() {
					action = processor.TaggedMetricActionConfig{
						Type: processor.ActionTypeCopyToMetric,
						Options: map[string]interface{}{
							"metadataKeys": []string{"m1", "m2"},
						},
					}

					taggedMetric = &data_receiver.TaggedMetric{
						Metric:    "test",
						Timestamp: 77,
						Value:     123.4,
						Tags: map[string]string{
							"test": "value",
							"m1":   "metadata1",
							"m2":   "metadata2",
						},
					}

					out.On("PutMetrics", mock.Anything, &data_receiver.Metrics{
						Metrics: []*data_receiver.Metric{{
							Metric:     "test",
							Timestamp:  77,
							Value:      123.4,
							Dimensions: map[string]string{"test": "value"},
							MetadataFields: metadata.FromStringMap(map[string]string{
								"m1": "metadata1",
								"m2": "metadata2",
							}),
						}},
					}).Return(nil, nil)

					_ = action.Apply(ctx, p, taggedMetric)
					out.AssertExpectations(GinkgoT())
				})
			})
		})

		Context("ModelActionConfig", func() {
			var action processor.ModelActionConfig
			var model *data_receiver.Model

			It("is named 'default' by default", func() {
				action = processor.ModelActionConfig{}
				Ω(action.GetName()).Should(Equal("default"))
			})

			It("can have a custom name", func() {
				action = processor.ModelActionConfig{Name: "test"}
				Ω(action.GetName()).Should(Equal("test"))
			})

			Context("Apply (noop)", func() {
				BeforeEach(func() {
					action = processor.ModelActionConfig{Type: processor.ActionTypeNoop}
					model = &data_receiver.Model{}
					err = action.Apply(ctx, p, model)
				})

				It("doesn't result in an error", func() {
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("shouldn't have modified the model", func() {
					Ω(model).Should(Equal(&data_receiver.Model{}))
				})
			})

			Context("Apply (log)", func() {
				BeforeEach(func() {
					action = processor.ModelActionConfig{
						Type: processor.ActionTypeLog,
						Name: "test"}

					model = &data_receiver.Model{
						Dimensions:     map[string]string{"test": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"test": "value"}),
					}

					err = action.Apply(ctx, p, model)
				})

				It("doesn't result in an error", func() {
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("logs the metric", func() {
					Ω(logOutput).Should(gbytes.Say(`Model{Dimensions: map\[test:value\], MetadataFields: map\[test:value\]} fields=map\[action:test processor:default\]`))
				})
			})

			Context("Apply (drop)", func() {
				BeforeEach(func() {
					action = processor.ModelActionConfig{Type: processor.ActionTypeDrop}
					err = action.Apply(ctx, p, &data_receiver.Model{})
				})

				It("returns the drop signal", func() {
					Ω(err).Should(Equal(&processor.SignalDrop{}))
				})
			})

			Context("Apply (send)", func() {
				BeforeEach(func() {
					action = processor.ModelActionConfig{Type: processor.ActionTypeSend}
					err = action.Apply(ctx, p, &data_receiver.Model{})
				})

				It("returns the send signal", func() {
					Ω(err).Should(MatchError(&processor.SignalSend{}))
				})
			})

			Context("Apply (append-to-metadata-field)", func() {
				It("fails without a key", func() {
					action = processor.ModelActionConfig{
						Type:    processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{}}

					err = action.Apply(ctx, p, &data_receiver.Model{})
					Ω(err).Should(MatchError(`missing "key"`))
				})

				It("fails with valueTemplate render error", func() {
					action = processor.ModelActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":           "test",
							"valueTemplate": "{{{dimensionX}}}/{{{metadataX}}}"}}

					model = &data_receiver.Model{
						Dimensions:     map[string]string{"dimension": "value"},
						MetadataFields: metadata.FromStringMap(map[string]string{"metadata": "value"}),
					}

					err = action.Apply(ctx, p, model)
					Ω(err).Should(MatchError(`Missing variable "dimensionX"`))
				})

				It("succeeds when metadata field was empty", func() {
					action = processor.ModelActionConfig{
						Type: processor.ActionTypeAppendToMetadataField,
						Options: processor.ActionOptions{
							"key":   "test",
							"value": "value"}}

					model = &data_receiver.Model{}

					err = action.Apply(ctx, p, model)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(model.GetMetadataFields()).Should(Equal(
						metadata.MustFromMap(map[string]interface{}{
							"test": []string{"value"}})))
				})

				// Other cases handled under MetricActionConfig.
			})
		})
	})
})
