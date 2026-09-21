package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/bugit/dre-engine/dre-collector/pkg/localcapture"
	"github.com/bugit/dre-engine/pkg/project"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "capture":
		runCapture(os.Args[2:])
	case "replay":
		runReplay(os.Args[2:])
	case "open":
		runOpen(os.Args[2:])
	case "snapshot":
		runSnapshot(os.Args[2:])
	case "snapshots":
		runSnapshots(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "version":
		fmt.Println("bugit", version)
	default:
		usage()
		os.Exit(1)
	}
}

func runCapture(args []string) {
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	saveOnExit := fs.Bool("save-on-exit", true, "save snapshot when command exits")
	mode := fs.String("mode", "local", "capture mode: local or cluster")
	detail := fs.String("detail", "manual capture", "snapshot detail on exit")
	root := fs.String("root", ".", "project root")
	_ = fs.Parse(args)

	rest := fs.Args()
	if len(rest) == 0 {
		fatal(fmt.Errorf("usage: bugit capture [--] <command...>"))
	}
	if rest[0] == "--" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		fatal(fmt.Errorf("command required after --"))
	}
	if *mode == "cluster" {
		fatal(fmt.Errorf("cluster mode: use dre-agent + dre-cli trigger (see docs/runbooks/)"))
	}

	rootPath, _ := filepath.Abs(*root)
	layout := project.LayoutFor(project.FindRoot(rootPath))
	cfg, _ := project.LoadConfig(layout.ConfigPath)

	fmt.Printf("BugIT capture starting in %s\n", rootPath)
	fmt.Printf("Record inbound HTTP via proxy: http://%s\n", cfg.RecordProxy)
	fmt.Printf("App listens on PORT=%d (internal)\n", cfg.AppPort+10000)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	path, err := localcapture.RunCapture(ctx, localcapture.CaptureOptions{
		Root:       rootPath,
		Command:    rest,
		SaveOnExit: *saveOnExit,
		Mode:       *mode,
		Detail:     *detail,
	})
	if path != "" {
		fmt.Printf("Snapshot saved: %s\n", path)
	}
	if err != nil && path == "" {
		fatal(err)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "capture finished with: %v\n", err)
	}
}

func runReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	openSourceAt := fs.Int("open-source-at", -1, "open source for event index after load")
	drePath := fs.String("dre", "", "snapshot path")
	_ = fs.Parse(args)

	rootPath, _ := filepath.Abs(*root)
	layout := project.LayoutFor(project.FindRoot(rootPath))
	snap := *drePath
	if snap == "" {
		var err error
		snap, err = localcapture.LatestSnapshot(rootPath)
		if err != nil {
			fatal(err)
		}
	}

	cfgPath := layout.ReplayPath
	if _, err := os.Stat(cfgPath); err != nil {
		cfgPath = ""
	}

	replayBin := resolveReplayBin()
	key := project.DefaultConfig().SnapshotKey
	if cfg, err := project.LoadConfig(layout.ConfigPath); err == nil && cfg.SnapshotKey != "" {
		key = cfg.SnapshotKey
	}

	argv := []string{"run", "--dre", snap, "--key", key}
	if cfgPath != "" {
		argv = append(argv, "--config", cfgPath)
	}
	cmd := exec.Command(replayBin, argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	fmt.Printf("Replaying %s\n", snap)
	if err := cmd.Run(); err != nil {
		fatal(err)
	}
	if *openSourceAt >= 0 {
		fmt.Printf("Event %d: use VS Code → DRE: Open Source at Current Event\n", *openSourceAt)
	}
}

func runOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	drePath := fs.String("dre", "", "snapshot path")
	_ = fs.Parse(args)

	rootPath, _ := filepath.Abs(*root)
	snap := *drePath
	if snap == "" {
		var err error
		snap, err = localcapture.LatestSnapshot(rootPath)
		if err != nil {
			fatal(err)
		}
	}

	if err := openVSCode(rootPath, snap); err != nil {
		fmt.Printf("VS Code not available (%v). Run: bugit replay --dre %s\n", err, snap)
		runReplay([]string{"--dre", snap, "--root", rootPath})
	}
}

func runSnapshot(args []string) {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	detail := fs.String("detail", "manual snapshot", "trigger detail")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d, _, err := localcapture.StartDaemon(ctx, project.FindRoot(*root))
	if err != nil {
		fatal(err)
	}
	defer d.Stop()
	path, err := d.TriggerSnapshotPublic(*detail)
	if err != nil {
		fatal(err)
	}
	fmt.Println(path)
}

func runSnapshots(args []string) {
	if len(args) > 0 && args[0] != "list" {
		fatal(fmt.Errorf("usage: bugit snapshots list"))
	}
	if len(args) > 0 {
		args = args[1:]
	}
	fs := flag.NewFlagSet("snapshots", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	_ = fs.Parse(args)

	snaps, err := localcapture.ListSnapshots(*root)
	if err != nil {
		fatal(err)
	}
	for _, s := range snaps {
		fmt.Println(s)
	}
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	_ = fs.Parse(args)

	rootPath := project.FindRoot(*root)
	layout := project.LayoutFor(rootPath)
	fmt.Println("BugIT doctor", version)
	fmt.Println("OS:", runtime.GOOS, runtime.GOARCH)
	fmt.Println("Project root:", rootPath)
	fmt.Println("BugIT dir:", layout.BugitDir)

	replayBin := resolveReplayBin()
	if _, err := exec.LookPath(replayBin); err != nil {
		if _, statErr := os.Stat(replayBin); statErr != nil {
			fmt.Println("WARN dre-replay not found:", replayBin)
			fmt.Println("  Fix: make build  OR  npm install -g @bugit/cli")
		} else {
			fmt.Println("OK dre-replay:", replayBin)
		}
	} else {
		fmt.Println("OK dre-replay:", replayBin)
	}

	if _, err := exec.LookPath("code"); err == nil {
		fmt.Println("OK VS Code CLI available")
	} else {
		fmt.Println("WARN VS Code CLI (code) not on PATH")
	}
}

func openVSCode(root, snap string) error {
	code, err := exec.LookPath("code")
	if err != nil {
		return err
	}
	cmd := exec.Command(code, root, "--command", "bugit.loadSnapshot")
	cmd.Env = append(os.Environ(), "BUGIT_OPEN_SNAPSHOT="+snap)
	return cmd.Start()
}

func resolveReplayBin() string {
	if v := os.Getenv("DRE_REPLAY_BIN"); v != "" {
		return v
	}
	if v := os.Getenv("BUGIT_REPLAY_BIN"); v != "" {
		return v
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	candidates := []string{
		filepath.Join("bin", "dre-replay"+ext),
		filepath.Join(".bugit", "bin", "dre-replay"+ext),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "dre-replay" + ext
}

func usage() {
	fmt.Println(`bugit — plug-and-play local bug capture and replay

Commands:
  bugit capture [--save-on-exit] [--root PATH] -- <command...>
  bugit replay [--dre PATH] [--root PATH] [--open-source-at N]
  bugit open [--dre PATH] [--root PATH]
  bugit snapshot [--detail TEXT] [--root PATH]
  bugit snapshots list [--root PATH]
  bugit doctor [--root PATH]
  bugit version`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
