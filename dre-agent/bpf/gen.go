//go:build ignore

package main

// Regenerate eBPF bindings (Linux/WSL2 only):
//
//   make bpf
//
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.16.0 -cc clang -cflags "-O2 -g -Wall" -target amd64,arm64 -go-package bpf -output-dir . bpf dre_probes.bpf.c -- -I../../api -I.
