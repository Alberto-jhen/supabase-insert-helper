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
	"regexp"
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

// maxNameLength caps the readable file-name part of an object key.
const maxNameLength = 100

// unsafeNameChars matches runs of characters that are not kept in the file-name part of an object key.
var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// validExt accepts short alphanumeric extensions such as ".jpg" or ".png".
var validExt = regexp.MustCompile(`^\.[a-z0-9]{1,10}$`)

// UploadImage downloads the image referenced by job.URL and uploads it to the
// Supabase Storage bucket specified in job.BucketName. It returns the object path
// inside the bucket or an error if any step fails.
func UploadImage(ctx context.Context, client *models.SupabaseClient, job models.ImageJob) (string, error) {
	data, contentType, err := downloadImage(ctx, client, job.URL)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	objectPath := buildObjectPath(job, contentType)

	if err := uploadToBucket(ctx, client, job.BucketName, objectPath, data, contentType); err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return objectPath, nil
}

// defaultFolder is the folder used when a request does not specify one.
const defaultFolder = "uploads"

// buildObjectPath returns the Storage key for a job, in the form
// "{folder}/{index}_{hash}_{name}{extension}". The folder defaults to defaultFolder.
// The name is job.Name when usable, otherwise the file name from the URL. The hash
// keeps keys unique per (URL, index), so two URLs that share a file name never
// overwrite each other, even though uploads use x-upsert. The name part is omitted
// when neither source provides one.
func buildObjectPath(job models.ImageJob, contentType string) string {
	stem, urlExt := imageNameFromURL(job.URL)
	if custom := sanitizeStem(stripImageExt(job.Name)); custom != "" {
		stem = custom
	}

	ext := extensionByType[contentType]
	if ext == "" {
		ext = urlExt
	}
	if ext == "" {
		ext = ".bin"
	}

	folder := job.Folder
	if folder == "" {
		folder = defaultFolder
	}

	hash := sha256.Sum256([]byte(job.URL + strconv.Itoa(job.Index)))
	shortHash := hex.EncodeToString(hash[:8])

	if stem == "" {
		return fmt.Sprintf("%s/%d_%s%s", folder, job.Index, shortHash, ext)
	}
	return fmt.Sprintf("%s/%d_%s_%s%s", folder, job.Index, shortHash, stem, ext)
}

// knownImageExts lists extensions removed from user-provided names, because the real
// extension is always taken from the file's content type.
var knownImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true}

// sanitizeStem keeps only [A-Za-z0-9._-] in a file name, replaces other runs with "_",
// trims surrounding underscores, and caps the length at maxNameLength.
func sanitizeStem(raw string) string {
	stem := strings.Trim(unsafeNameChars.ReplaceAllString(raw, "_"), "_")
	if len(stem) > maxNameLength {
		stem = stem[:maxNameLength]
	}
	return stem
}

// stripImageExt removes a trailing known image extension, such as ".PNG", from a name.
func stripImageExt(name string) string {
	ext := path.Ext(name)
	if knownImageExts[strings.ToLower(ext)] {
		return name[:len(name)-len(ext)]
	}
	return name
}

// imageNameFromURL extracts the file name from the last segment of the URL path.
// It returns the name without its extension, sanitized to [A-Za-z0-9._-] and capped
// in length, plus a lowercase extension. Query strings and fragments are ignored.
// Both values are empty when the URL has no usable file name.
func imageNameFromURL(rawURL string) (stem, ext string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}

	base := path.Base(u.Path)
	if base == "." || base == "/" {
		return "", ""
	}

	rawExt := path.Ext(base)
	stem = sanitizeStem(strings.TrimSuffix(base, rawExt))

	ext = strings.ToLower(rawExt)
	if !validExt.MatchString(ext) {
		ext = ""
	}

	return stem, ext
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
