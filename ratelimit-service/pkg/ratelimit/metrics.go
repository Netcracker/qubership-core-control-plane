package ratelimit

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    totalRequests    *prometheus.CounterVec
    rejectedRequests *prometheus.CounterVec
    currentLoad      *prometheus.GaugeVec
    limitHit         *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        totalRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "ratelimit_requests_total",
                Help: "Total number of rate limit checks",
            },
            []string{"key", "result"},
        ),
        rejectedRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "ratelimit_rejected_total",
                Help: "Total number of rejected requests",
            },
            []string{"key"},
        ),
        currentLoad: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "ratelimit_current_load",
                Help: "Current request count for rate limit keys",
            },
            []string{"key"},
        ),
        limitHit: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "ratelimit_limit_hit_total",
                Help: "Total number of times limit was hit",
            },
            []string{"key", "limit"},
        ),
    }
    
    if reg != nil {
        reg.MustRegister(m.totalRequests, m.rejectedRequests, m.currentLoad, m.limitHit)
    }
    
    return m
}

func (m *Metrics) RecordRateLimit(key string, allowed bool, current int, limit int) {
    result := "allowed"
    if !allowed {
        result = "rejected"
        m.rejectedRequests.WithLabelValues(key).Inc()
        m.limitHit.WithLabelValues(key, string(rune(limit))).Inc()
    }
    m.totalRequests.WithLabelValues(key, result).Inc()
    m.currentLoad.WithLabelValues(key).Set(float64(current))
}