package agent

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	pb "github.com/ayushpramanik/gpu-cluster-scheduler/internal/grpc/clusterpb"
)

// execute simulates running a GPU workload: it reports the job running, streams
// synthetic training-style log lines, waits a randomised duration, and reports a
// terminal status (occasionally failing to exercise retry/recovery paths).
func (a *Agent) execute(ctx context.Context, job *pb.Job) {
	defer a.done(job.GetJobId())

	a.report(ctx, job.GetJobId(), "running", "", 0)
	a.log.Info("executing job", "job", job.GetJobId(), "name", job.GetName())

	duration := a.jobDuration()
	steps := 5
	stepDelay := duration / time.Duration(steps)

	logStream, err := a.client.StreamLogs(ctx)
	sendLog := func(line string) {
		if err != nil || logStream == nil {
			return
		}
		_ = logStream.Send(&pb.LogLine{
			JobId:           job.GetJobId(),
			NodeId:          a.cfg.NodeID,
			TimestampUnixMs: time.Now().UnixMilli(),
			Line:            line,
		})
	}

	sendLog(fmt.Sprintf("pulling image %s", job.GetImage()))
	sendLog(fmt.Sprintf("allocated %d GPU(s) on %s", job.GetRequest().GetGpu(), a.cfg.Hostname))

	for step := 1; step <= steps; step++ {
		select {
		case <-ctx.Done():
			if logStream != nil {
				_, _ = logStream.CloseAndRecv()
			}
			return
		case <-time.After(stepDelay):
		}
		loss := 2.5 / float64(step)
		sendLog(fmt.Sprintf("step %d/%d loss=%.4f throughput=%d samples/s",
			step, steps, loss, 1000+a.rng.intn(500)))
	}

	if logStream != nil {
		_, _ = logStream.CloseAndRecv()
	}

	// ~10% simulated failure rate to exercise retries and recovery.
	if a.rng.float() < 0.10 {
		a.report(ctx, job.GetJobId(), "failed", "process exited non-zero", 1)
		a.log.Warn("job failed", "job", job.GetJobId())
		return
	}
	a.report(ctx, job.GetJobId(), "completed", "ok", 0)
	a.log.Info("job completed", "job", job.GetJobId())
}

func (a *Agent) report(ctx context.Context, jobID, status, msg string, exit int32) {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := a.client.UpdateJobStatus(rCtx, &pb.JobStatusUpdate{
		JobId:    jobID,
		NodeId:   a.cfg.NodeID,
		Status:   status,
		Message:  msg,
		ExitCode: exit,
	})
	if err != nil {
		a.log.Warn("report job status failed", "job", jobID, "status", status, "error", err)
	}
}

func (a *Agent) jobDuration() time.Duration {
	mean := a.cfg.MeanJobSeconds
	if mean < 2 {
		mean = 2
	}
	// Uniform in [mean/2, 3*mean/2].
	secs := mean/2 + a.rng.intn(mean)
	return time.Duration(secs) * time.Second
}

// rng is a tiny wrapper over math/rand seeded per node, so simulated behaviour
// is reproducible per node but varied across the cluster.
type rng struct{ r *rand.Rand }

func newRNG(seed int64) *rng { return &rng{r: rand.New(rand.NewSource(seed))} }

func (g *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return g.r.Intn(n)
}

func (g *rng) float() float64 { return g.r.Float64() }

func hashString(s string) int64 {
	var h int64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= int64(s[i])
		h *= 1099511628211
	}
	if h < 0 {
		h = -h
	}
	return h
}
