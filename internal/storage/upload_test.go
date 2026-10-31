package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
)

func TestImageNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantStem string
		wantExt  string
	}{
		{"simple name", "https://example.com/fotos/camisa-roja.jpg", "camisa-roja", ".jpg"},
		{"query and fragment ignored", "https://example.com/a/IMG 001.JPG?size=large#top", "IMG_001", ".jpg"},
		{"no extension", "https://picsum.photos/id/101/200/300", "300", ""},
		{"empty path", "https://example.com", "", ""},
		{"root path", "https://example.com/", "", ""},
		{"encoded and non ascii", "https://example.com/%E2%9C%93%20ok.png", "ok", ".png"},
		{"multiple dots", "https://example.com/archivo.tar.gz", "archivo.tar", ".gz"},
		{"long name is capped", "https://example.com/" + strings.Repeat("a", 150) + ".jpg", strings.Repeat("a", maxNameLength), ".jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stem, ext := imageNameFromURL(tt.url)
			if stem != tt.wantStem || ext != tt.wantExt {
				t.Errorf("imageNameFromURL(%q) = (%q, %q), want (%q, %q)", tt.url, stem, ext, tt.wantStem, tt.wantExt)
			}
		})
	}
}

func TestBuildObjectPath(t *testing.T) {
	// shortHash mirrors the hash computed in buildObjectPath.
	shortHash := func(rawURL string, index int) string {
		sum := sha256.Sum256([]byte(rawURL + fmt.Sprint(index)))
		return hex.EncodeToString(sum[:8])
	}

	tests := []struct {
		name        string
		job         models.ImageJob
		contentType string
		want        string
	}{
		{
			name:        "name from URL, extension from content type",
			job:         models.ImageJob{URL: "https://example.com/fotos/camisa-roja.png", BucketName: "prueba", Index: 0},
			contentType: "image/jpeg",
			want:        "uploads/0_" + shortHash("https://example.com/fotos/camisa-roja.png", 0) + "_camisa-roja.jpg",
		},
		{
			name:        "no file name in URL",
			job:         models.ImageJob{URL: "https://example.com/", BucketName: "prueba", Index: 3},
			contentType: "image/png",
			want:        "uploads/3_" + shortHash("https://example.com/", 3) + ".png",
		},
		{
			name:        "unknown content type falls back to URL extension",
			job:         models.ImageJob{URL: "https://example.com/a.webp", BucketName: "prueba", Index: 1},
			contentType: "application/octet-stream",
			want:        "uploads/1_" + shortHash("https://example.com/a.webp", 1) + "_a.webp",
		},
		{
			name:        "unknown content type and no URL extension falls back to .bin",
			job:         models.ImageJob{URL: "https://example.com/", BucketName: "prueba", Index: 0},
			contentType: "application/octet-stream",
			want:        "uploads/0_" + shortHash("https://example.com/", 0) + ".bin",
		},
		{
			name:        "numeric name without extension",
			job:         models.ImageJob{URL: "https://picsum.photos/id/101/200/300", BucketName: "prueba", Index: 2},
			contentType: "image/jpeg",
			want:        "uploads/2_" + shortHash("https://picsum.photos/id/101/200/300", 2) + "_300.jpg",
		},
		{
			name:        "custom folder replaces the default uploads folder",
			job:         models.ImageJob{URL: "https://example.com/fotos/camisa-roja.png", BucketName: "prueba", Folder: "directorio", Index: 0},
			contentType: "image/jpeg",
			want:        "directorio/0_" + shortHash("https://example.com/fotos/camisa-roja.png", 0) + "_camisa-roja.jpg",
		},
		{
			name:        "nested folder",
			job:         models.ImageJob{URL: "https://example.com/", BucketName: "prueba", Folder: "productos/2026", Index: 1},
			contentType: "image/png",
			want:        "productos/2026/1_" + shortHash("https://example.com/", 1) + ".png",
		},
		{
			name:        "custom name replaces the name from the URL",
			job:         models.ImageJob{URL: "https://example.com/fotos/camisa-roja.png", BucketName: "prueba", Name: "zapatilla", Index: 0},
			contentType: "image/jpeg",
			want:        "uploads/0_" + shortHash("https://example.com/fotos/camisa-roja.png", 0) + "_zapatilla.jpg",
		},
		{
			name:        "custom name with image extension is cleaned",
			job:         models.ImageJob{URL: "https://example.com/a.png", BucketName: "prueba", Name: "Zapatilla Roja.PNG", Index: 2},
			contentType: "image/jpeg",
			want:        "uploads/2_" + shortHash("https://example.com/a.png", 2) + "_Zapatilla_Roja.jpg",
		},
		{
			name:        "unusable custom name falls back to the name from the URL",
			job:         models.ImageJob{URL: "https://example.com/fotos/camisa-roja.jpg", BucketName: "prueba", Name: "¿¿¿", Index: 0},
			contentType: "image/jpeg",
			want:        "uploads/0_" + shortHash("https://example.com/fotos/camisa-roja.jpg", 0) + "_camisa-roja.jpg",
		},
		{
			name:        "custom name inside a folder",
			job:         models.ImageJob{URL: "https://example.com/", BucketName: "prueba", Folder: "directorio", Name: "camisa-roja", Index: 0},
			contentType: "image/jpeg",
			want:        "directorio/0_" + shortHash("https://example.com/", 0) + "_camisa-roja.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildObjectPath(tt.job, tt.contentType)
			if got != tt.want {
				t.Errorf("buildObjectPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
