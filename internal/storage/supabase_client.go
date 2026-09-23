package storage

import (
	"net/http"
	"sync"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
)

var (
	once     sync.Once
	instance *models.SupabaseClient
)

// GetClient returns the unique, thread-safe Supabase client instance.
// It initializes the instance on the first call using the provided credentials.
func GetClient(url, key string) *models.SupabaseClient {
	once.Do(func() {
		instance = &models.SupabaseClient{
			URL:        url,
			Key:        key,
			HTTPClient: &http.Client{},
		}
	})
	return instance
}
