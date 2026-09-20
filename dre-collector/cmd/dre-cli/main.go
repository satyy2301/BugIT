package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "trigger":
		runTrigger(os.Args[2:])
	case "list-snapshots":
		runList(os.Args[2:])
	case "download":
		runDownload(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func runTrigger(args []string) {
	fs := flag.NewFlagSet("trigger", flag.ExitOnError)
	collector := fs.String("collector", envOr("DRE_COLLECTOR_HTTP", "http://localhost:8080"), "collector HTTP base URL")
	detail := fs.String("detail", "manual trigger", "trigger detail")
	_ = fs.Parse(args)

	resp, err := http.Post(*collector+"/v1/trigger", "application/json",
		strings.NewReader(fmt.Sprintf(`{"detail":"%s"}`, *detail)))
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func runList(args []string) {
	fs := flag.NewFlagSet("list-snapshots", flag.ExitOnError)
	collector := fs.String("collector", envOr("DRE_COLLECTOR_HTTP", "http://localhost:8080"), "collector HTTP base URL")
	_ = fs.Parse(args)

	resp, err := http.Get(*collector + "/v1/snapshots")
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	var out interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func runDownload(args []string) {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	collector := fs.String("collector", envOr("DRE_COLLECTOR_HTTP", "http://localhost:8080"), "collector HTTP base URL")
	id := fs.String("id", "", "snapshot id (latest if empty)")
	out := fs.String("out", "", "output .dre path")
	_ = fs.Parse(args)

	resp, err := http.Get(*collector + "/v1/snapshots")
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	var parsed struct {
		Snapshots []struct {
			ID          string `json:"id"`
			Path        string `json:"path"`
			DownloadURL string `json:"download_url"`
		} `json:"snapshots"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		fatal(err)
	}
	if len(parsed.Snapshots) == 0 {
		fatal(fmt.Errorf("no snapshots"))
	}

	targetID := *id
	if targetID == "" {
		targetID = parsed.Snapshots[len(parsed.Snapshots)-1].ID
	}

	var snap *struct {
		ID          string `json:"id"`
		Path        string `json:"path"`
		DownloadURL string `json:"download_url"`
	}
	for i := range parsed.Snapshots {
		if parsed.Snapshots[i].ID == targetID {
			snap = &parsed.Snapshots[i]
			break
		}
	}
	if snap == nil {
		fatal(fmt.Errorf("snapshot %q not found", targetID))
	}

	downloadURL := snap.DownloadURL
	if downloadURL == "" {
		downloadURL = strings.TrimRight(*collector, "/") + "/v1/snapshots/" + snap.ID + "/download"
	}

	dresp, err := http.Get(downloadURL)
	if err != nil {
		fatal(err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != http.StatusOK {
		fatal(fmt.Errorf("download HTTP %d", dresp.StatusCode))
	}
	data, err := io.ReadAll(dresp.Body)
	if err != nil {
		fatal(err)
	}
	dest := *out
	if dest == "" {
		dest = fmt.Sprintf("incident-%s.dre", snap.ID)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		fatal(err)
	}
	fmt.Println(dest)
}

func usage() {
	fmt.Println(`dre-cli commands:
  dre-cli trigger [--collector URL] [--detail TEXT]
  dre-cli list-snapshots [--collector URL]
  dre-cli download [--id ID] [--out PATH] [--collector URL]`)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
