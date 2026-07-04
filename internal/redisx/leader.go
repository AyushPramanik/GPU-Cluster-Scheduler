package redisx

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// releaseScript atomically releases the lock only if we still own it, so a
// slow instance whose lease already expired cannot delete a new leader's lock.
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

// renewScript extends the lease only if we still own it.
var renewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("pexpire", KEYS[1], ARGV[2])
else
	return 0
end`)

// LeaderElector implements simple, correct single-leader election on top of
// Redis using a fenced lease (SET NX PX + owner-checked renew/release). Only the
// current leader runs the scheduling loop; standby replicas keep trying to
// acquire the lease so failover is automatic when the leader dies.
type LeaderElector struct {
	client   *redis.Client
	key      string
	identity string
	ttl      time.Duration
	log      *slog.Logger

	onElected func(ctx context.Context)
}

// NewLeaderElector builds an elector for the given lock key and lease TTL.
func NewLeaderElector(client *redis.Client, key string, ttl time.Duration, log *slog.Logger) *LeaderElector {
	return &LeaderElector{
		client:   client,
		key:      key,
		identity: uuid.NewString(),
		ttl:      ttl,
		log:      log,
	}
}

// Identity returns this instance's unique election identity.
func (e *LeaderElector) Identity() string { return e.identity }

// Run blocks until ctx is cancelled, continuously campaigning for leadership.
// While this instance holds the lease it invokes lead(leaderCtx); leaderCtx is
// cancelled the moment leadership is lost, so the callback must stop promptly.
func (e *LeaderElector) Run(ctx context.Context, lead func(leaderCtx context.Context)) {
	acquireInterval := e.ttl / 3
	if acquireInterval < time.Second {
		acquireInterval = time.Second
	}
	ticker := time.NewTicker(acquireInterval)
	defer ticker.Stop()

	for {
		if e.tryAcquire(ctx) {
			e.log.Info("acquired leadership", "identity", e.identity, "key", e.key)
			leaderCtx, cancel := context.WithCancel(ctx)
			go e.keepAlive(leaderCtx, cancel)
			lead(leaderCtx)
			cancel()
			e.release(context.Background())
			e.log.Info("lost leadership", "identity", e.identity)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *LeaderElector) tryAcquire(ctx context.Context) bool {
	ok, err := e.client.SetNX(ctx, e.key, e.identity, e.ttl).Result()
	if err != nil {
		e.log.Warn("leader acquire failed", "error", err)
		return false
	}
	return ok
}

// keepAlive renews the lease periodically; if renewal fails (lease lost), it
// cancels the leader context so the callback shuts down.
func (e *LeaderElector) keepAlive(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(e.ttl / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res, err := renewScript.Run(ctx, e.client, []string{e.key}, e.identity, e.ttl.Milliseconds()).Int()
			if err != nil || res == 0 {
				e.log.Warn("lease renewal failed, stepping down", "error", err)
				cancel()
				return
			}
		}
	}
}

func (e *LeaderElector) release(ctx context.Context) {
	if err := releaseScript.Run(ctx, e.client, []string{e.key}, e.identity).Err(); err != nil {
		e.log.Warn("leader release failed", "error", err)
	}
}
