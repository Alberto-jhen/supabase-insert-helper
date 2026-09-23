package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
)

// maxDownloadSize limits the amount of memory a single image can consume.
const maxDownloadSize = 50 * 1024 * 1024 // 50 MB

// extensionByType maps detected image MIME types to file extensions.
var extensionByType = map[string]string{
	"image/jpeg":    ".jpg",
	"image/png":     ".png",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/svg+xml": ".svg",
}

// UploadImage downloads the image referenced by job.URL and uploads it to the
// Supabase Storage bucket specified in job.BucketName. It returns the object path
// inside the bucket or an error if any step fails.
func UploadImage(ctx context.Context, client *models.SupabaseClient, job models.ImageJob) (string, error) {
	data, contentType, err := downloadImage(ctx, client, job.URL)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	ext := extensionByType[contentType]
	if ext == "" {
		ext = path.Ext(job.URL)
		if ext == "" {
			ext = ".bin"
		}
	}

	hash := sha256.Sum256([]byte(job.URL + strconv.Itoa(job.Index)))
	objectPath := fmt.Sprintf("uploads/%d_%s%s", job.Index, hex.EncodeToString(hash[:8]), ext)

	if err := uploadToBucket(ctx, client, job.BucketName, objectPath, data, contentType); err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return objectPath, nil
}

// downloadImage performs an HTTP GET on url using the shared Supabase HTTP client,
// limits the response size, and returns the raw bytes plus a cleaned MIME type.
func downloadImage(ctx context.Context, client *models.SupabaseClient, url string) ([]byte, string, error) {
	downloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}
	if int64(len(data)) > maxDownloadSize {
		return nil, "", fmt.Errorf("image exceeds %d bytes", maxDownloadSize)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])

	return data, contentType, nil
}

// uploadToBucket sends the raw bytes to Supabase Storage using the REST API.
func uploadToBucket(ctx context.Context, client *models.SupabaseClient, bucket, objectPath string, data []byte, contentType string) error {
	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	endpoint, err := url.JoinPath(strings.TrimSuffix(client.URL, "/"), "storage/v1/object", bucket, objectPath)
	if err != nil {
		return fmt.Errorf("build upload URL: %w", err)
	}

	req, err := http.NewRequestWithContext(uploadCtx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+client.Key)
	req.Header.Set("apikey", client.Key)
	req.Header.Set("Content-Type", contentType)
	// Allow re-uploading the same object path without failing.
	req.Header.Set("x-upsert", "true")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
