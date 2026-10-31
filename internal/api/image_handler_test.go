package api

import (
	"testing"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
)

func TestIsValidFolder(t *testing.T) {
	tests := []struct {
		folder string
		want   bool
	}{
		{"directorio", true},
		{"products/2026", true},
		{"my-photos_v1.0", true},
		{"", false},
		{"/directorio", false},
		{"directorio/", false},
		{"../secret", false},
		{"a/../b", false},
		{"./a", false},
		{"a//b", false},
		{"con espacios", false},
	}

	for _, tt := range tests {
		if got := isValidFolder(tt.folder); got != tt.want {
			t.Errorf("isValidFolder(%q) = %v, want %v", tt.folder, got, tt.want)
		}
	}
}

func TestValidateImagePayload(t *testing.T) {
	validImages := []models.ImageItem{{URL: "https://example.com/a.jpg"}}

	tests := []struct {
		name    string
		payload models.ImagePayload
		wantErr bool
	}{
		{"valid without folder or name", models.ImagePayload{BucketName: "prueba", Images: validImages}, false},
		{"valid with folder and name", models.ImagePayload{BucketName: "prueba", Folder: "directorio", Images: []models.ImageItem{{URL: "https://example.com/a.jpg", Name: "camisa-roja"}}}, false},
		{"missing bucket", models.ImagePayload{Images: validImages}, true},
		{"bucket with slash", models.ImagePayload{BucketName: "prueba/directorio", Images: validImages}, true},
		{"folder with parent segment", models.ImagePayload{BucketName: "prueba", Folder: "../secret", Images: validImages}, true},
		{"no images", models.ImagePayload{BucketName: "prueba"}, true},
		{"empty images list", models.ImagePayload{BucketName: "prueba", Images: []models.ImageItem{}}, true},
		{"image with empty url", models.ImagePayload{BucketName: "prueba", Images: []models.ImageItem{{Name: "camisa"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := tt.payload
			err := validateImagePayload(&payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImagePayload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
