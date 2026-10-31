package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

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

		for i, image := range payload.Images {
			job := models.ImageJob{
				URL:        image.URL,
				BucketName: payload.BucketName,
				Folder:     payload.Folder,
				Name:       image.Name,
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
			"count":  len(payload.Images),
		})
	}
}

// folderPattern accepts relative folder paths made of safe segments, such as "photos" or "products/2026".
var folderPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)

// validateImagePayload checks that the payload meets the minimum business rules.
func validateImagePayload(p *models.ImagePayload) error {
	if p.BucketName == "" {
		return errors.New("bucket_name is required")
	}
	if strings.Contains(p.BucketName, "/") {
		return errors.New("bucket_name must not contain '/'; use folder for subfolders")
	}
	if p.Folder != "" && !isValidFolder(p.Folder) {
		return errors.New("folder must use letters, digits, '.', '_' and '-' separated by '/', without '.' or '..' segments")
	}
	if len(p.Images) == 0 {
		return errors.New("images must contain at least one item")
	}
	for i, image := range p.Images {
		if image.URL == "" {
			return errors.New("images contains an empty url at index " + strconv.Itoa(i))
		}
	}
	return nil
}

// isValidFolder reports whether folder is a safe relative path inside a bucket.
func isValidFolder(folder string) bool {
	if !folderPattern.MatchString(folder) {
		return false
	}
	for _, segment := range strings.Split(folder, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
