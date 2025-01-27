package uploader

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
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

	log.Info().
		Dur("interval", w.interval).
		Dur("sync_interval", w.syncInterval).
		Msg("started cleanup worker")
}

func (w *CleanupWorker) Stop() {
	w.cleanupTicker.Stop()
	w.syncTicker.Stop()
	close(w.done)
	log.Info().Msg("cleanup worker stopped")
}

func (w *CleanupWorker) performInitialCleanup(ctx context.Context) {
	log.Info().Msg("performing initial cleanup")

	if err := w.service.CleanupExpiredFiles(ctx); err != nil {
		log.Error().
			Err(err).
			Msg("error during initial expired files cleanup")
	}

	if err := w.service.SyncStorageWithDatabase(ctx); err != nil {
		log.Error().
			Err(err).
			Msg("error during initial storage sync")
	}
}

func (w *CleanupWorker) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("context cancelled, cleanup worker shutting down")
			return
		case <-w.done:
			return
		case <-w.cleanupTicker.C:
			if err := w.service.CleanupExpiredFiles(ctx); err != nil {
				log.Error().
					Err(err).
					Msg("error cleaning up expired files")
			}
		case <-w.syncTicker.C:
			if err := w.service.SyncStorageWithDatabase(ctx); err != nil {
				log.Error().
					Err(err).
					Msg("error syncing storage with database")
			}
		}
	}
}

// StartExpiredFilesWorker is kept for backward compatibility
func StartExpiredFilesWorker(ctx context.Context, service *service, interval time.Duration) {
	worker := NewCleanupWorker(service, interval)
	worker.Start(ctx)
}
