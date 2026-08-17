package worker

import (
	"context"
	"os"
	"strconv"
	"time"
)

// retentionAge is how long a finalized job's source file is kept in R2 before it
// becomes eligible for deletion.
const retentionAge = 7 * 24 * time.Hour

// retentionLoop runs R2 source-file cleanup at the configured interval, starting
// immediately. The interval defaults to 6 hours and can be overridden via the
// RETENTION_INTERVAL environment variable (in seconds).
func (w *Worker) retentionLoop(ctx context.Context) {
	interval := retentionInterval()
	t := time.NewTicker(interval)
	defer t.Stop()

	w.log.Info("retention started", "interval", interval.String(), "age", retentionAge.String())
	w.runRetention(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.runRetention(ctx)
		}
	}
}

// runRetention deletes the R2 source files of finalized jobs older than the
// retention age. Cleanup is idempotent: a missing object counts as success. Only
// job IDs are logged, never file keys.
func (w *Worker) runRetention(ctx context.Context) {
	jobs, err := w.jobs.ListForRetention(ctx, time.Now().Add(-retentionAge))
	if err != nil {
		w.log.Error("worker: list retention jobs", "error", err.Error())
		return
	}
	for _, j := range jobs {
		if err := w.deps.storage.Delete(ctx, j.FileKey); err != nil {
			w.log.Error("worker: retention delete failed", "job_id", j.ID, "error", err.Error())
			continue
		}
		w.log.Info("worker: retention deleted", "job_id", j.ID)
	}
}

// defaultRetentionInterval is the fallback cleanup cadence (6 hours).
const defaultRetentionInterval = 6 * time.Hour

// retentionInterval parses RETENTION_INTERVAL seconds with a fallback.
func retentionInterval() time.Duration {
	raw := os.Getenv("RETENTION_INTERVAL")
	if raw == "" {
		return defaultRetentionInterval
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || secs <= 0 {
		return defaultRetentionInterval
	}
	return time.Duration(secs) * time.Second
}
