package scheduler

import (
	"context"
	"sync/atomic"

	"go-espn-api/internal/ingest"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// jobSummary tallies the outcome of a job's fan-out, mirroring the
// created/updated/errors dicts the Python refresh_all_* tasks log.
type jobSummary struct {
	created int64
	updated int64
	errors  int64
}

// fanOut runs every unit through a bounded worker pool (errgroup + SetLimit),
// capping concurrent ingests at s.concurrency so concurrent DB writes stay
// within the Postgres pool. Per-unit isolation is total: a returned error or a
// panic is logged and counted but never aborts the other units (the group's
// error channel is deliberately never used to short-circuit). A per-job summary
// is logged at the end.
func (s *Scheduler) fanOut(
	ctx context.Context,
	jobName string,
	units []workUnit,
	do func(ctx context.Context, u workUnit) (ingest.IngestionResult, error),
) jobSummary {
	var sum jobSummary

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for _, u := range units {
		u := u

		g.Go(func() error {
			// Panic isolation: a panic in one unit must not abort the job.
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&sum.errors, 1)
					log.Error().
						Str("job", jobName).
						Str("sport", u.sport).
						Str("league", u.league).
						Str("date", u.date).
						Interface("panic", r).
						Msg("ingest unit panicked")
				}
			}()

			if gctx.Err() != nil {
				return nil
			}

			res, err := do(gctx, u)
			if err != nil {
				atomic.AddInt64(&sum.errors, 1)
				log.Error().
					Err(err).
					Str("job", jobName).
					Str("sport", u.sport).
					Str("league", u.league).
					Str("date", u.date).
					Msg("ingest unit failed")

				return nil
			}

			atomic.AddInt64(&sum.created, int64(res.Created))
			atomic.AddInt64(&sum.updated, int64(res.Updated))

			// Always return nil: errors are counted, not propagated, so one
			// failing unit never cancels the group (Celery per-task isolation).
			return nil
		})
	}

	_ = g.Wait()

	log.Info().
		Str("job", jobName).
		Int("units", len(units)).
		Int64("created", sum.created).
		Int64("updated", sum.updated).
		Int64("errors", sum.errors).
		Msg("scheduler job completed")

	return sum
}
