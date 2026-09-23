package workers

import (
	"context"
	"log/slog"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
	"github.com/Alberto-jhen/supabase-insert-helper/internal/storage"
)

// NewStorageProcessor returns a Processor that downloads each assigned image
// and uploads it to Supabase Storage using the provided singleton client.
func NewStorageProcessor(client *models.SupabaseClient) Processor {
	return func(ctx context.Context, job models.ImageJob) models.ImageResult {
		path, err := storage.UploadImage(ctx, client, job)
		if err != nil {
			slog.Error("image upload failed",
				"url", job.URL,
				"bucket", job.BucketName,
				"index", job.Index,
				"error", err,
			)
			return models.ImageResult{
				URL:   job.URL,
				Error: err.Error(),
			}
		}

		slog.Info("image uploaded",
			"url", job.URL,
			"bucket", job.BucketName,
			"path", path,
		)
		return models.ImageResult{
			URL:  job.URL,
			Path: path,
		}
	}
}

// StartResultCollector launches a background goroutine that reads results from
// the provided channel and logs them with slog. It exits when the channel is closed.
func StartResultCollector(ctx context.Context, results <-chan models.ImageResult) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case result, ok := <-results:
				if !ok {
					return
				}
				if result.Error != "" {
					slog.Error("image job failed",
						"url", result.URL,
						"error", result.Error,
					)
				} else {
					slog.Info("image job completed",
						"url", result.URL,
						"path", result.Path,
					)
				}
			}
		}
	}()
}
