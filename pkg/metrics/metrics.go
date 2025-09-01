package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Total AWS API errors when managing Pod Identity Associations, labeled by operation
	PodIdentityAssociationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pia_operator_pod_identity_association_errors_total",
			Help: "Total number of errors when managing Pod Identity Associations via AWS API, labeled by operation (create, update, delete)",
		},
		[]string{"operation"},
	)

	// Number of Pod Identity Associations managed by the operator
	PodIdentityAssociationsManaged = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pia_operator_pod_identity_associations_managed",
			Help: "Number of Pod Identity Associations managed by the operator",
		},
	)
)

// RegisterMetrics registers all custom metrics with the given Prometheus registry
func RegisterMetrics(registry prometheus.Registerer) {
	registry.MustRegister(PodIdentityAssociationErrors)
	registry.MustRegister(PodIdentityAssociationsManaged)
}

// IncAssociationError increments the error counter for a given operation
func IncAssociationError(operation string) {
	PodIdentityAssociationErrors.WithLabelValues(operation).Inc()
}

// SetAssociationsManaged sets the gauge for the number of managed associations
func SetAssociationsManaged(count int) {
	PodIdentityAssociationsManaged.Set(float64(count))
}
