package phone

import (
	"context"
	"time"

	"manager-backend/framework"

	"gorm.io/gorm"
)

const workerInterval = 5 * time.Second

type taskWorker struct {
	svc  *serviceImpl
	db   *gorm.DB
	quit chan struct{}
}

func newTaskWorker(svc *serviceImpl, db *gorm.DB) *taskWorker {
	return &taskWorker{svc: svc, db: db, quit: make(chan struct{})}
}

func (w *taskWorker) start() {
	go func() {
		ticker := time.NewTicker(workerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-w.quit:
				return
			case <-ticker.C:
				// HA：每 tick 仅一个实例执行；lease=interval，依赖 runDueTasks 幂等容忍偶发重叠。
				_, _ = framework.TryRunLocked(w.db, "phone:task-worker", workerInterval, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					w.svc.runDueTasks(ctx)
					return nil
				})
			}
		}
	}()
}

func (w *taskWorker) stop() { close(w.quit) }
