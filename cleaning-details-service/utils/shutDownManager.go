package utils

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type ShutdownManager struct {
	cancelFunc    context.CancelFunc
	shutdownTasks []func(context.Context) error
	mu            sync.Mutex
}

func NewShutdownManager(ctx context.Context) (context.Context, *ShutdownManager) {
	ctx, cancel := context.WithCancel(ctx)
	manager := &ShutdownManager{
		cancelFunc: cancel,
	}
	return ctx, manager
}

func (sm *ShutdownManager) Register(task func(context.Context) error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.shutdownTasks = append(sm.shutdownTasks, task)
}

func (sm *ShutdownManager) StartListening() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("[SHUTDOWN] Received signal: %v", sig)
		sm.cancelFunc()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		sm.mu.Lock()
		defer sm.mu.Unlock()
		for _, task := range sm.shutdownTasks {
			if err := task(ctx); err != nil {
				log.Printf("[SHUTDOWN] Error during shutdown: %v", err)
			}
		}

		log.Println("[SHUTDOWN] Graceful shutdown complete")
		os.Exit(0)
	}()
}
