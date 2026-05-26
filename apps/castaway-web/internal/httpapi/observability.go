package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "castaway_web_http_requests_total",
			Help: "Total Castaway web HTTP requests.",
		},
		[]string{"method", "route", "status_class"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "castaway_web_http_request_duration_seconds",
			Help:    "Castaway web HTTP request duration in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route"},
	)
	httpInFlightRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "castaway_web_http_in_flight_requests",
			Help: "Current number of in-flight Castaway web HTTP requests.",
		},
	)
)

var (
	requestLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	dbMetricsOnce sync.Once
)

func (s *Server) registerMetrics() {
	dbMetricsOnce.Do(func() {
		promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "castaway_web_db_pool_acquired_connections",
				Help: "Number of acquired connections in the Castaway web database pool.",
			},
			func() float64 { return float64(s.pool.Stat().AcquiredConns()) },
		)
		promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "castaway_web_db_pool_idle_connections",
				Help: "Number of idle connections in the Castaway web database pool.",
			},
			func() float64 { return float64(s.pool.Stat().IdleConns()) },
		)
		promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "castaway_web_db_pool_max_connections",
				Help: "Maximum configured connections in the Castaway web database pool.",
			},
			func() float64 { return float64(s.pool.Stat().MaxConns()) },
		)
		promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "castaway_web_db_pool_acquire_count_total",
				Help: "Total number of successful connection acquires from the Castaway web database pool.",
			},
			func() float64 { return float64(s.pool.Stat().AcquireCount()) },
		)
		promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "castaway_web_db_pool_acquire_duration_seconds_total",
				Help: "Total duration spent acquiring Castaway web database pool connections in seconds.",
			},
			func() float64 { return s.pool.Stat().AcquireDuration().Seconds() },
		)
		promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "castaway_web_db_up",
				Help: "Whether Castaway web can ping its database (1 = up, 0 = down).",
			},
			func() float64 {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := s.pool.Ping(ctx); err != nil {
					return 0
				}
				return 1
			},
		)
	})
}

func metricsHandler() gin.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		httpInFlightRequests.Inc()
		defer httpInFlightRequests.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		statusClass := strconv.Itoa(status/100) + "xx"
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(c.Request.Method, route, statusClass).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, route).Observe(duration)
	}
}

func structuredRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		if route == "/healthz" && status < http.StatusBadRequest {
			return
		}

		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.String("service", "castaway-web"),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", status),
			slog.String("status_class", strconv.Itoa(status/100)+"xx"),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		}
		if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error_class", "handler_error"))
		}

		requestLogger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}
