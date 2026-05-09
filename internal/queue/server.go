package queue

import (
	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/hibiken/asynq"
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
				"critical": 6, // grading — exam-blocking
				"default":  3, // email
			},
		},
	)
}

// NewServeMux returns a fresh asynq.ServeMux for handler registration.
func NewServeMux() *asynq.ServeMux {
	return asynq.NewServeMux()
}
