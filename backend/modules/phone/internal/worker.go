package phone

import (
	"context"
	"time"
)

// workerInterval 是异步任务收敛的轮询间隔。
const workerInterval = 5 * time.Second

// taskWorker 周期性轮询中台，把云手机从过渡态（CREATING/STARTING）收敛到稳定态。
// 只有在中台已配置时才启动（见 module.OnStart）。
type taskWorker struct {
	svc  *serviceImpl
	quit chan struct{}
}

func newTaskWorker(svc *serviceImpl) *taskWorker {
	return &taskWorker{svc: svc, quit: make(chan struct{})}
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
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				w.svc.runDueTasks(ctx)
				cancel()
			}
		}
	}()
}

func (w *taskWorker) stop() {
	close(w.quit)
}
