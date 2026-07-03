// Package config loads runtime configuration from environment variables with
// sensible defaults, so every service is configured the same way.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full runtime configuration shared across services. Each service
// reads the subset it needs.
type Config struct {
	Env         string
	LogLevel    string
	ServiceName string

	HTTP      HTTPConfig
	GRPC      GRPCConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Scheduler SchedulerConfig
	Telemetry TelemetryConfig
	Auth      AuthConfig
}

// HTTPConfig configures the REST API gateway.
type HTTPConfig struct {
	Addr            string
	MetricsAddr     string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	RateLimitRPS    int
	RateLimitBurst  int
}

// GRPCConfig configures gRPC servers and clients.
type GRPCConfig struct {
	// SchedulerAddr is the address agents/clients dial to reach the scheduler.
	SchedulerAddr string
	// SchedulerListen is the bind address the scheduler's gRPC server listens on.
	SchedulerListen string
	AgentAddr       string
	MetricsAddr     string
}

// PostgresConfig configures the PostgreSQL connection pool.
type PostgresConfig struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
}

// RedisConfig configures the Redis client used for the queue and coordination.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// SchedulerConfig tunes scheduling behaviour.
type SchedulerConfig struct {
	// Algorithm selects the placement strategy: first-fit, best-fit, priority,
	// or fair-share.
	Algorithm string
	// Interval is how often the scheduler drains the queue.
	Interval time.Duration
	// AgingFactor adds this many priority points per minute a job waits, to
	// prevent starvation.
	AgingFactor float64
	// StarvationThreshold is the wait time after which a job may preempt lower
	// priority running jobs.
	StarvationThreshold time.Duration
	// EnablePreemption allows high priority jobs to evict lower priority ones.
	EnablePreemption bool
	// HeartbeatTTL is how long a node may go without a heartbeat before it is
	// marked down.
	HeartbeatTTL time.Duration
	// LeaderTTL is the lease duration for scheduler leader election.
	LeaderTTL time.Duration
	// MaxRetries is the default retry budget for failed jobs.
	MaxRetries int
}

// TelemetryConfig configures tracing and metrics export.
type TelemetryConfig struct {
	OTLPEndpoint  string
	TracingEnable bool
	SampleRatio   float64
}

// AuthConfig configures API authentication.
type AuthConfig struct {
	JWTSecret string
	Enabled   bool
}

// Load builds a Config from the environment. serviceName identifies the calling
// service and is used for telemetry.
func Load(serviceName string) *Config {
	return &Config{
		Env:         getEnv("APP_ENV", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		ServiceName: serviceName,
		HTTP: HTTPConfig{
			Addr:            getEnv("HTTP_ADDR", ":8080"),
			MetricsAddr:     getEnv("METRICS_ADDR", ":9090"),
			ReadTimeout:     getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 20*time.Second),
			RateLimitRPS:    getInt("RATE_LIMIT_RPS", 100),
			RateLimitBurst:  getInt("RATE_LIMIT_BURST", 200),
		},
		GRPC: GRPCConfig{
			// Accept both the native names and the deployment (compose/k8s) names.
			SchedulerAddr:   firstEnv([]string{"SCHEDULER_GRPC_ADDR", "SCHEDULER_ADDR"}, "localhost:50051"),
			SchedulerListen: firstEnv([]string{"SCHEDULER_GRPC_LISTEN", "GRPC_ADDR"}, ":50051"),
			AgentAddr:       getEnv("AGENT_GRPC_ADDR", "localhost:50061"),
			MetricsAddr:     getEnv("METRICS_GRPC_ADDR", "localhost:50071"),
		},
		Postgres: PostgresConfig{
			DSN:         firstEnv([]string{"POSTGRES_DSN", "DATABASE_URL"}, "postgres://gpuscheduler:gpuscheduler@localhost:5432/gpuscheduler?sslmode=disable"),
			MaxConns:    int32(getInt("POSTGRES_MAX_CONNS", 20)),
			MinConns:    int32(getInt("POSTGRES_MIN_CONNS", 2)),
			MaxConnLife: getDuration("POSTGRES_MAX_CONN_LIFE", time.Hour),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		Scheduler: SchedulerConfig{
			Algorithm:           getEnv("SCHEDULER_ALGORITHM", "best-fit"),
			Interval:            getDuration("SCHEDULER_INTERVAL", 2*time.Second),
			AgingFactor:         getFloat("SCHEDULER_AGING_FACTOR", 1.0),
			StarvationThreshold: getDuration("SCHEDULER_STARVATION_THRESHOLD", 5*time.Minute),
			EnablePreemption:    getBool("SCHEDULER_ENABLE_PREEMPTION", true),
			HeartbeatTTL:        getDuration("HEARTBEAT_TTL", 30*time.Second),
			LeaderTTL:           getDuration("LEADER_TTL", 15*time.Second),
			MaxRetries:          getInt("MAX_RETRIES", 3),
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint: normalizeOTLP(firstEnv([]string{"OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT"}, "")),
			// Tracing turns on automatically when an OTLP endpoint is configured,
			// unless explicitly overridden.
			TracingEnable: getBool("TRACING_ENABLE", firstEnv([]string{"OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT"}, "") != ""),
			SampleRatio:   getFloat("TRACE_SAMPLE_RATIO", 1.0),
		},
		Auth: AuthConfig{
			JWTSecret: getEnv("JWT_SECRET", "dev-insecure-secret-change-me"),
			Enabled:   getBool("AUTH_ENABLED", false),
		},
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// firstEnv returns the value of the first set (non-empty) key, else def. This
// lets a service accept multiple env-var spellings (native and deployment).
func firstEnv(keys []string, def string) string {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			return v
		}
	}
	return def
}

// normalizeOTLP strips a scheme from an OTLP endpoint since the gRPC exporter
// wants a bare host:port (compose sets http://otel-collector:4317).
func normalizeOTLP(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return endpoint
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// Validate performs basic sanity checks on the loaded configuration.
func (c *Config) Validate() error {
	if c.Postgres.DSN == "" {
		return fmt.Errorf("postgres DSN is required")
	}
	if c.Auth.Enabled && c.Auth.JWTSecret == "dev-insecure-secret-change-me" {
		return fmt.Errorf("JWT_SECRET must be set when auth is enabled")
	}
	return nil
}
