package models

import "net/http"

// SupabaseClient holds the Supabase configuration and the HTTP engine (singleton).
type SupabaseClient struct {
	URL        string
	Key        string
	HTTPClient *http.Client
}

// ImagePayload represents the JSON body expected by POST /ingest/images.
// Folder is optional; when empty, images are stored in the default "uploads" folder.
type ImagePayload struct {
	BucketName string      `json:"bucket_name"`
	Folder     string      `json:"folder"`
	Images     []ImageItem `json:"images"`
}

// ImageItem is one image to ingest: its public URL and an optional file name.
// When Name is empty, the file name is taken from the URL.
type ImageItem struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// RecordPayload represents the JSON body expected by POST /ingest/records.
type RecordPayload struct {
	TableName string                   `json:"table_name"`
	Records   []map[string]interface{} `json:"records"`
}

// ImageJob describes a single unit of work to download and upload one image.
type ImageJob struct {
	URL        string
	BucketName string
	Folder     string
	Name       string
	Index      int
}

// ImageResult describes the outcome of processing an ImageJob.
type ImageResult struct {
	URL   string `json:"url"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}
