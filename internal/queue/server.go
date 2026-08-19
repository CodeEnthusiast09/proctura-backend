package queue

import (
	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/hibiken/asynq"
)

// Queue names — single source of truth shared by client (enqueue) and server (consume).
const (
	QueueCritical = "critical" // grading — exam-blocking
	QueueDefault  = "default"  // email
)

// NewServer builds an asynq server bound to the given Redis config.
// The caller registers handlers via NewServeMux and passes the mux to Run.
func NewServer(cfg config.RedisConfig) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				QueueCritical: 6,
				QueueDefault:  3,
			},
		},
	)
}

// NewScheduler builds an asynq scheduler for tasks that run on a fixed
// interval rather than in response to a request. asynq holds the schedule in
// Redis, so running several workers still produces one run per interval
// instead of one per worker.
func NewScheduler(cfg config.RedisConfig) *asynq.Scheduler {
	return asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		},
		nil,
	)
}

// NewServeMux returns a fresh asynq.ServeMux for handler registration.
func NewServeMux() *asynq.ServeMux {
	return asynq.NewServeMux()
}
