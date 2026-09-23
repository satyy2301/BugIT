package captureattach

import "testing"

func TestPrepareBackendInspectorInvalidPID(t *testing.T) {
	_, err := PrepareBackendInspector("", 0, TargetPickOptions{CaptureRoot: "/app/backend"})
	if err == nil {
		t.Fatal("expected error without inspector targets")
	}
}

func TestEvalRemoteBool(t *testing.T) {
	if !evalRemoteBool(map[string]interface{}{"value": true}) {
		t.Fatal("expected true")
	}
	if !evalRemoteBool(map[string]interface{}{"result": map[string]interface{}{"value": true}}) {
		t.Fatal("expected nested true")
	}
}
