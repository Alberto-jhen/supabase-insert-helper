package models

import "net/http"

// SupabaseClient holds the Supabase configuration and the HTTP engine (singleton).
type SupabaseClient struct {
	URL        string
	Key        string
	HTTPClient *http.Client
}

// ImagePayload represents the JSON body expected by POST /ingest/images.
type ImagePayload struct {
	BucketName string   `json:"bucket_name"`
	ImageURLs  []string `json:"image_urls"`
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
	Index      int
}

// ImageResult describes the outcome of processing an ImageJob.
type ImageResult struct {
	URL   string `json:"url"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}
