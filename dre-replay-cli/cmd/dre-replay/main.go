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
	"github.com/bugit/dre-engine/dre-replay-cli/internal/debugger"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/proxy"
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
	_ = fs.Parse(args)

	snap, err := archive.Open(*path, *key)
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if *format == "ide" {
		_ = enc.Encode(map[string]interface{}{
			"manifest":      snap.Manifest,
			"vector_graph":  snap.VectorGraph,
			"event_count":   len(snap.Events),
			"vector_nodes":  len(snap.VectorGraph.Nodes),
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
	proxyAddr := fs.String("proxy", "127.0.0.1:18080", "proxy listen address")
	_ = fs.Parse(args)

	snap, err := archive.Open(*path, *key)
	if err != nil {
		fatal(err)
	}
	p := proxy.New(*proxyAddr, snap.Events)
	if err := p.Start(); err != nil {
		fatal(err)
	}
	dbg := debugger.New("/tmp/dre-debug.sock", snap.Events)
	if err := dbg.Start(); err != nil {
		log.Printf("debugger api: %v", err)
	}

	log.Printf("replay ready: manifest=%s events=%d", snap.Manifest.ID, len(snap.Events))
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func usage() {
	fmt.Println(`dre-replay commands:
  dre-replay load --dre incident.dre [--key KEY]
  dre-replay run --dre incident.dre [--proxy 127.0.0.1:18080] [--key KEY]`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
