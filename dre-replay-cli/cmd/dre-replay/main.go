package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bugit/dre-engine/dre-replay-cli/internal/archive"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/delve"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/orchestrator"
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
	cfgPath := fs.String("config", "", "replay.yaml path")
	proxyAddr := fs.String("proxy", "", "replay proxy address override for ide output")
	delveAddr := fs.String("delve", "127.0.0.1:2345", "delve listen address for ide output")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	if *proxyAddr != "" {
		cfg.ProxyAddr = *proxyAddr
	}

	snap, err := archive.Open(*path, *key)
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if *format == "ide" {
		_ = enc.Encode(map[string]interface{}{
			"manifest":       snap.Manifest,
			"vector_graph":   snap.VectorGraph,
			"event_count":    len(snap.Events),
			"vector_nodes":   len(snap.VectorGraph.Nodes),
			"events":         summary.SummarizeEvents(snap.Events),
			"flow":           summary.FlowDescription(snap.Events),
			"clock_timeline": snap.ClockTimeline,
			"replay": map[string]string{
				"proxy_addr":  cfg.ProxyAddr,
				"debug_addr":  cfg.DebugAddr,
				"delve_addr":  *delveAddr,
				"status":      "ready",
				"config_path": *cfgPath,
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
	binary := fs.String("binary", "", "optional binary to launch under time-freeze and Delve")
	delveAddr := fs.String("delve", "127.0.0.1:2345", "delve listen address")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}

	snap, err := archive.Open(*path, *key)
	if err != nil {
		fatal(err)
	}

	session := orchestrator.New(snap, cfg, orchestrator.Options{
		ProxyAddrOverride: *proxyAddr,
		Binary:            *binary,
		DelveAddr:         *delveAddr,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := session.Start(ctx); err != nil {
		fatal(err)
	}
	defer session.Stop()

	if addrs := session.ProxyAddrs(); len(addrs) > 0 {
		log.Printf("proxy listeners: %s", strings.Join(addrs, ", "))
	}
	if cfg.TimeFreeze.Enabled {
		log.Printf("time-freeze enabled (shim=%s frozen_ns=%d)", cfg.TimeFreeze.ShimPath, session.FrozenTimestampNs())
	}

	<-ctx.Done()
}

func runDebug(args []string) {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	binary := fs.String("binary", "", "Go binary to debug")
	delveAddr := fs.String("delve", "127.0.0.1:2345", "delve listen address")
	_ = fs.Parse(args)
	if *binary == "" {
		fatal(fmt.Errorf("--binary required"))
	}
	d := delve.New()
	if err := d.StartHeadless(*binary, *delveAddr); err != nil {
		fatal(err)
	}
	fmt.Printf("Delve headless on %s\n", *delveAddr)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	d.Stop()
}

func usage() {
	fmt.Println(`dre-replay commands:
  dre-replay load --dre incident.dre [--key KEY] [--format ide] [--config replay.yaml]
  dre-replay run --dre incident.dre [--config replay.yaml] [--proxy ADDR] [--binary ./app]
  dre-replay debug --binary ./app [--delve 127.0.0.1:2345]`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
