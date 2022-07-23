package kude

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gopkg.in/yaml.v3"
	"io"
	"log"
)

type Execution interface {
	GetPipeline() Pipeline
	GetLogger() *log.Logger
	ExecuteToWriter(ctx context.Context, w io.Writer) error
	ExecuteToChannel(ctx context.Context, target chan *yaml.Node) error
}

var (
	executionsDurationHistogramMetric = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "kude_pipeline_executions_duration_seconds",
		Help:    "Histogram of Kude pipeline executions",
		Buckets: prometheus.LinearBuckets(1000, 5000, 20),
	})
	resGenDurationHistogramMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kude_pipeline_resource_generation_duration_seconds",
		Help:    "Histogram of Kude pipeline resource generation",
		Buckets: prometheus.LinearBuckets(100, 1000, 20),
	}, []string{"path"})
	resGenCounterMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kude_pipeline_resource_generation_count",
		Help: "Count of Kude pipeline resource generation",
	}, []string{"path"})
	stepDurationHistogramMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kude_pipeline_step_duration_seconds",
		Help:    "Histogram of Kude pipeline steps",
		Buckets: prometheus.LinearBuckets(100, 1000, 20),
	}, []string{"id", "name"})
	stepGaugeMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kude_pipeline_step_running_total",
		Help: "Gauge of Kude pipeline steps currently running",
	}, []string{"id", "name"})
	stepInputResourcesCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kude_pipeline_step_input_resources_total",
		Help: "Counter of input resources by step",
	}, []string{"id", "name"})
	stepOutputResourcesCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kude_pipeline_step_output_resources_total",
		Help: "Counter of output resources by step",
	}, []string{"id", "name"})
	collectedResourcesCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kude_pipeline_collected_resources_total",
		Help: "Counter of collected resources",
	})
	resolvedResourcesCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kude_pipeline_resolved_resources_total",
		Help: "Counter of resolved resources",
	})
)
