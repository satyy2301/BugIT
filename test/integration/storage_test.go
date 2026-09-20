package integration

import (
	"os"
	"testing"

	"github.com/bugit/dre-engine/dre-collector/internal/storage"
)

func TestStorageConfigNoBucket(t *testing.T) {
	_ = os.Unsetenv("DRE_S3_BUCKET")
	_ = os.Unsetenv("DRE_GCS_BUCKET")
	u, err := storage.NewFromConfig(storage.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Fatal("expected nil without bucket")
	}
}
