package stats

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	// ReceivedCompactMetrics is the number of compact metrics received.
	ReceivedCompactMetrics = stats.Int64("zenoss.io/received_compact_metrics", "Number of compact metrics received.", stats.UnitDimensionless)

	// SentCompactMetrics is the number of compact metrics successfully sent.
	SentCompactMetrics = stats.Int64("zenoss.io/sent_compact_metrics", "Number of compact metrics successfully sent.", stats.UnitDimensionless)

	// FailedCompactMetrics is the number of compact metrics unsuccessfully sent.
	FailedCompactMetrics = stats.Int64("zenoss.io/failed_compact_metrics", "Number of compact metrics unsuccessfully sent.", stats.UnitDimensionless)

	// ReceivedTaggedMetrics is the number of tagged metrics received.
	ReceivedTaggedMetrics = stats.Int64("zenoss.io/received_tagged_metrics", "Number of tagged metrics received.", stats.UnitDimensionless)

	// SentTaggedMetrics is the number of tagged metrics successfully sent.
	SentTaggedMetrics = stats.Int64("zenoss.io/sent_tagged_metrics", "Number of tagged metrics successfully sent.", stats.UnitDimensionless)

	// FailedTaggedMetrics is the number of tagged metrics unsuccessfully sent.
	FailedTaggedMetrics = stats.Int64("zenoss.io/failed_tagged_metrics", "Number of tagged metrics unsuccessfully sent.", stats.UnitDimensionless)

	// ReceivedMetrics is the number of metrics received.
	ReceivedMetrics = stats.Int64("zenoss.io/received_metrics", "Number of metrics received.", stats.UnitDimensionless)

	// SentMetrics is the number of metrics successfully sent.
	SentMetrics = stats.Int64("zenoss.io/sent_metrics", "Number of metrics successfully sent.", stats.UnitDimensionless)

	// FailedMetrics is the number of metrics unsuccessfully sent.
	FailedMetrics = stats.Int64("zenoss.io/failed_metrics", "Number of metrics unsuccessfully sent.", stats.UnitDimensionless)

	// ReceivedModels is the number of models received.
	ReceivedModels = stats.Int64("zenoss.io/received_models", "Number of models received.", stats.UnitDimensionless)

	// SentModels is the number of models successfully sent.
	SentModels = stats.Int64("zenoss.io/sent_models", "Number of models successfully sent.", stats.UnitDimensionless)

	// FailedModels is the number of models unsuccessfully sent.
	FailedModels = stats.Int64("zenoss.io/failed_models", "Number of models unsuccessfully sent.", stats.UnitDimensionless)

	// ReceivedEvents is the number of Events received.
	ReceivedEvents = stats.Int64("zenoss.io/received_events", "Number of events received.", stats.UnitDimensionless)

	// SentEvents is the number of Events successfully sent.
	SentEvents = stats.Int64("zenoss.io/sent_events", "Number of events successfully sent.", stats.UnitDimensionless)

	// FailedEvents is the number of Events unsuccessfully sent.
	FailedEvents = stats.Int64("zenoss.io/failed_events", "Number of events unsuccessfully sent.", stats.UnitDimensionless)
)

var (
	// KeyModuleType is used to tag metrics with the type of module that processed them.
	KeyModuleType = tag.MustNewKey("module_type")

	// KeyModuleName is used to tag metrics with the name of the module that processed them.
	KeyModuleName = tag.MustNewKey("module_name")
)

var (
	// DefaultCountDistribution is a simple powers-of-two distribution.
	DefaultCountDistribution = view.Distribution(0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536)
)

var (
	// ReceivedCompactMetricsView views the sum of compact metrics received.
	ReceivedCompactMetricsView = &view.View{
		Name:        "zenoss.io/received_compact_metrics",
		Description: "Sum of compact metrics received.",
		Measure:     ReceivedCompactMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// ReceivedCompactMetricsPerRequestView views the distribution of compact metrics received per request.
	ReceivedCompactMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/received_compact_metrics_per_request",
		Description: "Distribution of compact metrics received per request.",
		Measure:     ReceivedCompactMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// SentCompactMetricsView views the sum of compact metrics successfully sent.
	SentCompactMetricsView = &view.View{
		Name:        "zenoss.io/sent_compact_metrics",
		Description: "Sum of compact metrics successfully sent.",
		Measure:     SentCompactMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// SentCompactMetricsPerRequestView views the distribution of compact metrics successfully sent per request.
	SentCompactMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/sent_compact_metrics_per_request",
		Description: "Distribution of compact metrics successfully sent per request.",
		Measure:     SentCompactMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// FailedCompactMetricsView views the sum of compact metrics unsuccessfully sent.
	FailedCompactMetricsView = &view.View{
		Name:        "zenoss.io/failed_compact_metrics",
		Description: "Sum of compact metrics unsuccessfully sent.",
		Measure:     FailedCompactMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// FailedCompactMetricsPerRequestView views the distribution of compact metrics unsuccessfully sent per request.
	FailedCompactMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/failed_compact_metrics_per_request",
		Description: "Distribution of compact metrics unsuccessfully sent per request.",
		Measure:     FailedCompactMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// ReceivedTaggedMetricsView views the sum of tagged metrics received.
	ReceivedTaggedMetricsView = &view.View{
		Name:        "zenoss.io/received_tagged_metrics",
		Description: "Sum of tagged metrics received.",
		Measure:     ReceivedTaggedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// ReceivedTaggedMetricsPerRequestView views the distribution of tagged metrics received per request.
	ReceivedTaggedMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/received_tagged_metrics_per_request",
		Description: "Distribution of tagged metrics received per request.",
		Measure:     ReceivedTaggedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// SentTaggedMetricsView views the sum of tagged metrics successfully sent.
	SentTaggedMetricsView = &view.View{
		Name:        "zenoss.io/sent_tagged_metrics",
		Description: "Sum of tagged metrics successfully sent.",
		Measure:     SentTaggedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// SentTaggedMetricsPerRequestView views the distribution of tagged metrics successfully sent per request.
	SentTaggedMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/sent_tagged_metrics_per_request",
		Description: "Distribution of tagged metrics successfully sent per request.",
		Measure:     SentTaggedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// FailedTaggedMetricsView views the sum of tagged metrics unsuccessfully sent.
	FailedTaggedMetricsView = &view.View{
		Name:        "zenoss.io/failed_tagged_metrics",
		Description: "Sum of tagged metrics unsuccessfully sent.",
		Measure:     FailedTaggedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// FailedTaggedMetricsPerRequestView views the distribution of tagged metrics unsuccessfully sent per request.
	FailedTaggedMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/failed_tagged_metrics_per_request",
		Description: "Distribution of tagged metrics unsuccessfully sent per request.",
		Measure:     FailedTaggedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// ReceivedMetricsView views the sum of tagged metrics received.
	ReceivedMetricsView = &view.View{
		Name:        "zenoss.io/received_metrics",
		Description: "Sum of tagged metrics received.",
		Measure:     ReceivedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// ReceivedMetricsPerRequestView views the distribution of metrics received per request.
	ReceivedMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/received_metrics_per_request",
		Description: "Distribution of metrics received per request.",
		Measure:     ReceivedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// SentMetricsView views the sum of metrics successfully sent.
	SentMetricsView = &view.View{
		Name:        "zenoss.io/sent_metrics",
		Description: "Sum of metrics successfully sent.",
		Measure:     SentMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// SentMetricsPerRequestView views the distribution of metrics successfully sent per request.
	SentMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/sent_metrics_per_request",
		Description: "Distribution of metrics successfully sent per request.",
		Measure:     SentMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// FailedMetricsView views the sum of metrics unsuccessfully sent.
	FailedMetricsView = &view.View{
		Name:        "zenoss.io/failed_metrics",
		Description: "Sum of metrics unsuccessfully sent.",
		Measure:     FailedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// FailedMetricsPerRequestView views the distribution of metrics unsuccessfully sent per request.
	FailedMetricsPerRequestView = &view.View{
		Name:        "zenoss.io/failed_metrics_per_request",
		Description: "Distribution of metrics unsuccessfully sent per request.",
		Measure:     FailedMetrics,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// ReceivedModelsView views the sum of models received.
	ReceivedModelsView = &view.View{
		Name:        "zenoss.io/received_models",
		Description: "Sum of models received.",
		Measure:     ReceivedModels,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// ReceivedModelsPerRequestView views the distribution of models received per request.
	ReceivedModelsPerRequestView = &view.View{
		Name:        "zenoss.io/received_models_per_request",
		Description: "Distribution of models received per request.",
		Measure:     ReceivedModels,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// SentModelsView views the sum of models successfully sent.
	SentModelsView = &view.View{
		Name:        "zenoss.io/sent_models",
		Description: "Sum of models successfully sent.",
		Measure:     SentModels,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// SentModelsPerRequestView views the distribution of models successfully sent per request.
	SentModelsPerRequestView = &view.View{
		Name:        "zenoss.io/sent_models_per_request",
		Description: "Distribution of models successfully sent per request.",
		Measure:     SentModels,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}

	// FailedModelsView views the sum of models unsuccessfully sent.
	FailedModelsView = &view.View{
		Name:        "zenoss.io/failed_models",
		Description: "Sum of models unsuccessfully sent.",
		Measure:     FailedModels,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: view.Sum(),
	}

	// FailedModelsPerRequestView views the distribution of models unsuccessfully sent per request.
	FailedModelsPerRequestView = &view.View{
		Name:        "zenoss.io/failed_models_per_request",
		Description: "Distribution of models unsuccessfully sent per request.",
		Measure:     FailedModels,
		TagKeys:     []tag.Key{KeyModuleType, KeyModuleName},
		Aggregation: DefaultCountDistribution,
	}
)

// DefaultViews are the default views provided by this package.
var DefaultViews = []*view.View{
	ReceivedCompactMetricsView,
	ReceivedCompactMetricsPerRequestView,
	SentCompactMetricsView,
	SentCompactMetricsPerRequestView,
	FailedCompactMetricsView,
	FailedCompactMetricsPerRequestView,
	ReceivedTaggedMetricsView,
	ReceivedTaggedMetricsPerRequestView,
	SentTaggedMetricsView,
	SentTaggedMetricsPerRequestView,
	FailedTaggedMetricsView,
	FailedTaggedMetricsPerRequestView,
	ReceivedMetricsView,
	ReceivedMetricsPerRequestView,
	SentMetricsView,
	SentMetricsPerRequestView,
	FailedMetricsView,
	FailedMetricsPerRequestView,
	ReceivedModelsView,
	ReceivedModelsPerRequestView,
	SentModelsView,
	SentModelsPerRequestView,
	FailedModelsView,
	FailedModelsPerRequestView,
}
