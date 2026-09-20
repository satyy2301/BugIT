package storage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObjectKey(t *testing.T) {
	if ObjectKey("prod", "abc") != "prod/incident-abc.dre" {
		t.Fatal(ObjectKey("prod", "abc"))
	}
	if ObjectKey("", "abc") != "incident-abc.dre" {
		t.Fatal(ObjectKey("", "abc"))
	}
}

func TestConfigFromEnvEmpty(t *testing.T) {
	cfg := ConfigFromEnv()
	u, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Fatal("expected nil uploader without bucket env")
	}
}

// MinIO-style S3 mock via httptest is skipped; integration test covers real upload.

func TestTrimIncidentKey(t *testing.T) {
	if trimIncidentKey("incident-foo.dre") != "foo" {
		t.Fatal()
	}
}

// Ensure httptest import used in future integration helpers.
var _ = httptest.NewServer(http.NotFoundHandler())
