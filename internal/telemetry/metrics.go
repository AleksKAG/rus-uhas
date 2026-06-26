package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics содержит все метрики системы РУС-УХАС
type Metrics struct {
	// Generator metrics
	GeneratorPowerWatts      prometheus.Gauge
	GeneratorFrequencyHz     prometheus.Gauge
	GeneratorTipTempC        prometheus.Gauge
	GeneratorImpedanceOhms   prometheus.Gauge
	GeneratorAspirationBar   prometheus.Gauge
	GeneratorIrrigationMlMin prometheus.Gauge
	GeneratorIsFiring        prometheus.Gauge

	// Operation metrics
	OperationDuration        prometheus.Histogram
	OperationTissueRemoved   prometheus.Counter
	OperationBloodLoss       prometheus.Counter
	OperationCyclesTotal     prometheus.Counter
	OperationErrorsTotal     prometheus.Counter

	// Protocol metrics
	ProtocolSwitches         prometheus.Counter
	ProtocolSafetyStops      prometheus.Counter

	// AI metrics
	AIClassificationDuration prometheus.Histogram
	AIClassificationsTotal   prometheus.Counter

	// System metrics
	ControlLoopDuration      prometheus.Histogram
	SystemUptime             prometheus.Gauge
}

// NewMetrics создает и регистрирует все метрики
func NewMetrics() *Metrics {
	return &Metrics{
		// Generator metrics
		GeneratorPowerWatts: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "power_watts",
			Help:      "Current generator power in watts",
		}),
		GeneratorFrequencyHz: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "frequency_hz",
			Help:      "Current generator frequency in Hz",
		}),
		GeneratorTipTempC: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "tip_temp_celsius",
			Help:      "Current tip temperature in Celsius",
		}),
		GeneratorImpedanceOhms: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "impedance_ohms",
			Help:      "Current tissue impedance in Ohms",
		}),
		GeneratorAspirationBar: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "aspiration_bar",
			Help:      "Current aspiration vacuum in bar",
		}),
		GeneratorIrrigationMlMin: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "irrigation_ml_min",
			Help:      "Current irrigation flow in ml/min",
		}),
		GeneratorIsFiring: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "generator",
			Name:      "is_firing",
			Help:      "Whether generator is currently firing (1) or not (0)",
		}),

		// Operation metrics
		OperationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "rus_uhas",
			Subsystem: "operation",
			Name:      "duration_seconds",
			Help:      "Duration of surgical operations",
			Buckets:   prometheus.ExponentialBuckets(60, 2, 10), // 1min to ~17hours
		}),
		OperationTissueRemoved: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "operation",
			Name:      "tissue_removed_ml",
			Help:      "Total volume of tissue removed in ml",
		}),
		OperationBloodLoss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "operation",
			Name:      "blood_loss_ml",
			Help:      "Estimated blood loss in ml",
		}),
		OperationCyclesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "operation",
			Name:      "control_cycles_total",
			Help:      "Total number of control loop cycles",
		}),
		OperationErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "operation",
			Name:      "errors_total",
			Help:      "Total number of errors by type",
		}, []string{"error_type"}),

		// Protocol metrics
		ProtocolSwitches: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "protocol",
			Name:      "switches_total",
			Help:      "Total number of protocol switches",
		}, []string{"protocol"}),
		ProtocolSafetyStops: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "protocol",
			Name:      "safety_stops_total",
			Help:      "Total number of safety-triggered stops",
		}),

		// AI metrics
		AIClassificationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "rus_uhas",
			Subsystem: "ai",
			Name:      "classification_duration_seconds",
			Help:      "Duration of tissue classification",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		}),
		AIClassificationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "rus_uhas",
			Subsystem: "ai",
			Name:      "classifications_total",
			Help:      "Total number of tissue classifications",
		}, []string{"tissue_type"}),

		// System metrics
		ControlLoopDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "rus_uhas",
			Subsystem: "system",
			Name:      "control_loop_duration_seconds",
			Help:      "Duration of control loop iterations",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		SystemUptime: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "rus_uhas",
			Subsystem: "system",
			Name:      "uptime_seconds",
			Help:      "System uptime in seconds",
		}),
	}
}

// UpdateFromState обновляет метрики из состояния генератора
func (m *Metrics) UpdateFromState(power, freq, temp, impedance, aspiration, irrigation float64, isFiring bool) {
	m.GeneratorPowerWatts.Set(power)
	m.GeneratorFrequencyHz.Set(freq)
	m.GeneratorTipTempC.Set(temp)
	m.GeneratorImpedanceOhms.Set(impedance)
	m.GeneratorAspirationBar.Set(aspiration)
	m.GeneratorIrrigationMlMin.Set(irrigation)
	
	if isFiring {
		m.GeneratorIsFiring.Set(1)
	} else {
		m.GeneratorIsFiring.Set(0)
	}
}
