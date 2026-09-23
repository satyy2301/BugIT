package debugbridge

import "testing"

func TestParseStackLocationWindowsPath(t *testing.T) {
	file, line, col := parseStackLocation(`C:\repo\server.js:42:5`)
	if file != `C:\repo\server.js` {
		t.Fatalf("file %q", file)
	}
	if line != 42 || col != 5 {
		t.Fatalf("line=%d col=%d", line, col)
	}
}

func TestParseStackLocationUnixPath(t *testing.T) {
	file, line, col := parseStackLocation(`/app/backend/server.js:10:3`)
	if file != `/app/backend/server.js` {
		t.Fatalf("file %q", file)
	}
	if line != 10 || col != 3 {
		t.Fatalf("line=%d col=%d", line, col)
	}
}

func TestParseNodeStackWindows(t *testing.T) {
	stack := "Error\n    at handler (C:\\repo\\server.js:42:5)\n    at processTicksAndRejections (node:internal/process/task_queues:95:5)"
	ref := parseNodeStack(stack)
	if ref == nil {
		t.Fatal("expected source ref")
	}
	if ref.File != `C:\repo\server.js` {
		t.Fatalf("file %q", ref.File)
	}
	if ref.Line != 42 || ref.Column != 5 {
		t.Fatalf("line=%d col=%d", ref.Line, ref.Column)
	}
}
