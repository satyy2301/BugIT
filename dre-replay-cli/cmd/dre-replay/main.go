package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bugit/dre-engine/dre-replay-cli/internal/archive"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/debugger"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/delve"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/proxy"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/summary"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "load":
		runLoad(os.Args[2:])
	case "run":
		runServe(os.Args[2:])
	case "debug":
		runDebug(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func runLoad(args []string) {
	fs := flag.NewFlagSet("load", flag.ExitOnError)
	path := fs.String("dre", "", "path to .dre snapshot")
	key := fs.String("key", "dev-insecure-key-change-me", "AES key")
	format := fs.String("format", "manifest", "output format: manifest or ide")
	proxyAddr := fs.String("proxy", "127.0.0.1:18080", "replay proxy address for ide output")
	_ = fs.Parse(args)

	snap, err := archive.Open(*path, *key)
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if *format == "ide" {
		_ = enc.Encode(map[string]interface{}{
			"manifest":     snap.Manifest,
			"vector_graph": snap.VectorGraph,
			"event_count":  len(snap.Events),
			"vector_nodes": len(snap.VectorGraph.Nodes),
			"events":       summary.SummarizeEvents(snap.Events),
			"flow":         summary.FlowDescription(snap.Events),
			"clock_timeline": snap.ClockTimeline,
			"replay": map[string]string{
				"proxy_addr": *proxyAddr,
				"debug_addr": "127.0.0.1:19090",
				"status":     "ready",
			},
		})
		return
	}
	_ = enc.Encode(snap.Manifest)
	fmt.Fprintf(os.Stderr, "events=%d vector_nodes=%d\n",
		len(snap.Events), len(snap.VectorGraph.Nodes))
}

func runServe(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	path := fs.String("dre", "", "path to .dre snapshot")
	key := fs.String("key", "dev-insecure-key-change-me", "AES key")
	cfgPath := fs.String("config", "", "replay.yaml path")
	proxyAddr := fs.String("proxy", "", "proxy listen address override")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	addr := cfg.ProxyAddr
	if *proxyAddr != "" {
		addr = *proxyAddr
	}

	snap, err := archive.Open(*path, *key)
	if err != nil {
		fatal(err)
	}
	p := proxy.New(addr, snap.Events)
	if err := p.Start(); err != nil {
		fatal(err)
	}
	dbg := debugger.New("/tmp/dre-debug.sock", snap.Events)
	if err := dbg.Start(); err != nil {
		log.Printf("debugger api: %v", err)
	}
	if cfg.TimeFreeze.Enabled && len(snap.ClockTimeline.Entries) > 0 {
		ts := snap.ClockTimeline.Entries[0].TimestampNs
		os.Setenv("DRE_FROZEN_TIME_NS", fmt.Sprintf("%d", ts))
		log.Printf("time-freeze env DRE_FROZEN_TIME_NS=%d (use LD_PRELOAD=%s)", ts, cfg.TimeFreeze.ShimPath)
	}

	log.Printf("replay ready: manifest=%s events=%d", snap.Manifest.ID, len(snap.Events))
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func runDebug(args []string) {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	binary := fs.String("binary", "", "Go binary to debug")
	_ = fs.Parse(args)
	if *binary == "" {
		fatal(fmt.Errorf("--binary required"))
	}
	d := delve.New()
	if err := d.StartHeadless(*binary); err != nil {
		fatal(err)
	}
	fmt.Println("Delve headless on 127.0.0.1:2345")
}

func usage() {
	fmt.Println(`dre-replay commands:
  dre-replay load --dre incident.dre [--key KEY] [--format ide]
  dre-replay run --dre incident.dre [--config replay.yaml] [--proxy ADDR]
  dre-replay debug --binary ./app`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
