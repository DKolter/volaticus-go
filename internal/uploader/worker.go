package uploader

import (
	"context"
	"log"
	"time"
)

type CleanupWorker struct {
	service       *service
	interval      time.Duration
	syncInterval  time.Duration
	done          chan struct{}
	cleanupTicker *time.Ticker
	syncTicker    *time.Ticker
}

func NewCleanupWorker(service *service, interval time.Duration) *CleanupWorker {
	return &CleanupWorker{
		service:      service,
		interval:     interval,
		syncInterval: time.Hour * 6, // Sync every 6 hours
		done:         make(chan struct{}),
	}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	// Perform initial cleanup
	w.performInitialCleanup(ctx)

	// Start tickers
	w.cleanupTicker = time.NewTicker(w.interval)
	w.syncTicker = time.NewTicker(w.syncInterval)

	go w.run(ctx)

	log.Printf("Started cleanup worker with interval: %v, sync interval: %v", w.interval, w.syncInterval)
}

func (w *CleanupWorker) Stop() {
	w.cleanupTicker.Stop()
	w.syncTicker.Stop()
	close(w.done)
	log.Printf("Cleanup worker stopped")
}

func (w *CleanupWorker) performInitialCleanup(ctx context.Context) {
	log.Printf("Performing initial cleanup...")

	if err := w.service.CleanupExpiredFiles(ctx); err != nil {
		log.Printf("Error during initial expired files cleanup: %v", err)
	}

	if err := w.service.SyncStorageWithDatabase(ctx); err != nil {
		log.Printf("Error during initial storage sync: %v", err)
	}
}

func (w *CleanupWorker) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, cleanup worker shutting down")
			return
		case <-w.done:
			return
		case <-w.cleanupTicker.C:
			if err := w.service.CleanupExpiredFiles(ctx); err != nil {
				log.Printf("Error cleaning up expired files: %v", err)
			}
		case <-w.syncTicker.C:
			if err := w.service.SyncStorageWithDatabase(ctx); err != nil {
				log.Printf("Error syncing storage with database: %v", err)
			}
		}
	}
}

// StartExpiredFilesWorker is kept for backward compatibility
func StartExpiredFilesWorker(ctx context.Context, service *service, interval time.Duration) {
	worker := NewCleanupWorker(service, interval)
	worker.Start(ctx)
}
