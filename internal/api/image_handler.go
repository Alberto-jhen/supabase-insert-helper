package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
	"github.com/Alberto-jhen/supabase-insert-helper/internal/workers"
)

// ImageIngestHandler returns an http.HandlerFunc that parses and validates the
// incoming JSON for POST /ingest/images and submits each URL as a job to the worker pool.
// It returns 202 Accepted immediately so the HTTP server is not blocked by processing.
func ImageIngestHandler(pool *workers.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			WriteError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		var payload models.ImagePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON payload")
			return
		}

		if err := validateImagePayload(&payload); err != nil {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		for i, url := range payload.ImageURLs {
			job := models.ImageJob{
				URL:        url,
				BucketName: payload.BucketName,
				Index:      i,
			}
			if err := pool.Submit(r.Context(), job); err != nil {
				WriteError(w, http.StatusServiceUnavailable, "server is shutting down")
				return
			}
		}

		WriteJSON(w, http.StatusAccepted, map[string]interface{}{
			"status": "accepted",
			"bucket": payload.BucketName,
			"count":  len(payload.ImageURLs),
		})
	}
}

// validateImagePayload checks that the payload meets the minimum business rules.
func validateImagePayload(p *models.ImagePayload) error {
	if p.BucketName == "" {
		return errors.New("bucket_name is required")
	}
	if len(p.ImageURLs) == 0 {
		return errors.New("image_urls must contain at least one URL")
	}
	for i, url := range p.ImageURLs {
		if url == "" {
			return errors.New("image_urls contains an empty URL at index " + strconv.Itoa(i))
		}
	}
	return nil
}
