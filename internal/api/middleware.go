package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// requestLogger logs each request with method, path, status, and latency.
func (s *Server) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.log.Info("http request",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000.0,
			"client_ip", c.ClientIP(),
		)
	}
}

// metricsMiddleware records Prometheus request counters and latency histograms.
func (s *Server) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		if s.metrics != nil {
			s.metrics.HTTPRequests.WithLabelValues(c.Request.Method, route, http.StatusText(c.Writer.Status())).Inc()
			s.metrics.HTTPLatency.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
		}
	}
}

// tracing starts an OpenTelemetry span per request and propagates context.
func (s *Server) tracing() gin.HandlerFunc {
	tracer := otel.Tracer("api-gateway")
	prop := otel.GetTextMapPropagator()
	return func(c *gin.Context) {
		ctx := prop.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		ctx, span := tracer.Start(ctx, c.Request.Method+" "+route,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", route),
			),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		span.SetAttributes(attribute.Int("http.status_code", c.Writer.Status()))
	}
}

// rateLimit enforces the per-IP token bucket, returning 429 when exhausted.
func (s *Server) rateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.limiter != nil && !s.limiter.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// cors permits the frontend dev origin to call the API.
func (s *Server) cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// auth validates a bearer JWT when authentication is enabled. The verified
// subject (user ID) is stored on the context for handlers to use.
func (s *Server) auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.Auth.Enabled {
			c.Next()
			return
		}
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(s.cfg.Auth.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if sub, ok := claims["sub"].(string); ok {
				c.Set("user_id", sub)
			}
		}
		c.Next()
	}
}
