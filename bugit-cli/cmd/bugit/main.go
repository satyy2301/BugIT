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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bugit/dre-engine/dre-collector/pkg/localcapture"
	"github.com/bugit/dre-engine/pkg/captureattach"
	"github.com/bugit/dre-engine/pkg/discover"
	"github.com/bugit/dre-engine/pkg/project"
)

const version = "1.1.1"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "capture":
		runCapture(os.Args[2:])
	case "record":
		runRecord(os.Args[2:])
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
	case "status":
		runStatus(os.Args[2:])
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
	root := fs.String("root", ".", "workspace folder")
	auto := fs.Bool("auto", false, "auto-detect dev command and ports")
	_ = fs.Parse(args)

	rest := fs.Args()
	if *auto {
		rest = nil
	} else if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	if !*auto && len(rest) == 0 {
		fatal(fmt.Errorf("usage: bugit capture [--auto] OR bugit capture -- <command...>"))
	}
	if *mode == "cluster" {
		fatal(fmt.Errorf("cluster mode: use dre-agent + dre-cli trigger (see docs/runbooks/)"))
	}

	rootPath, _ := filepath.Abs(*root)
	_, _, _ = project.SyncWorkspaceConfig(rootPath)
	target := project.ResolveCaptureTarget(rootPath)

	fmt.Printf("BugIT capture in %s\n", target.Root)
	fmt.Printf("Use your app at %s\n", localcapture.PublicURL(rootPath))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var path string
	var err error
	if *auto {
		path, err = localcapture.RunCaptureAuto(ctx, rootPath, *saveOnExit, *detail)
	} else {
		path, err = localcapture.RunCapture(ctx, localcapture.CaptureOptions{
			Root:       rootPath,
			Command:    rest,
			SaveOnExit: *saveOnExit,
			Mode:       *mode,
			Detail:     *detail,
		})
	}
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

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	root := fs.String("root", ".", "workspace folder")
	_ = fs.Parse(args)
	target := project.ResolveCaptureTarget(*root)
	fmt.Printf("capture_root=%s\n", target.Root)
	fmt.Printf("dev_command=%s\n", project.DetectDevCommand(target.Root))
	fmt.Printf("public_url=%s\n", localcapture.PublicURL(*root))
}

func runReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	openSourceAt := fs.Int("open-source-at", -1, "open source for event index after load")
	drePath := fs.String("dre", "", "snapshot path")
	_ = fs.Parse(args)

	rootPath, _ := filepath.Abs(*root)
	captureRoot := project.FindCaptureRoot(rootPath)
	layout := project.LayoutFor(captureRoot)
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

func runRecord(args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	saveOnExit := fs.Bool("save-on-exit", true, "save snapshot when recording stops")
	detail := fs.String("detail", "manual capture", "snapshot detail on exit")
	root := fs.String("root", ".", "workspace folder")
	_ = fs.Parse(args)

	rootPath, _ := filepath.Abs(*root)
	_, _, err := project.SyncWorkspaceConfig(rootPath)
	if err != nil {
		fatal(err)
	}
	disc := discover.Discover(rootPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var path string
	if discover.IsListening("127.0.0.1", disc.BackendPort) {
		fmt.Printf("BugIT record (attach) — backend listening on :%d\n", disc.BackendPort)
		path, err = localcapture.RunAttachRecord(ctx, rootPath, disc, *saveOnExit, *detail)
	} else {
		fmt.Printf("No listener on :%d — spawn fallback (starting dev server)\n", disc.BackendPort)
		fmt.Printf("Use your app at %s\n", localcapture.PublicURL(rootPath))
		path, err = localcapture.RunCaptureAuto(ctx, rootPath, *saveOnExit, *detail)
	}
	if path != "" {
		fmt.Printf("Snapshot saved: %s\n", path)
	}
	if err != nil && path == "" {
		fatal(err)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "record finished with: %v\n", err)
	}
}

func runSnapshot(args []string) {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	root := fs.String("root", ".", "project root")
	detail := fs.String("detail", "manual snapshot", "trigger detail")
	_ = fs.Parse(args)

	rootPath, _ := filepath.Abs(*root)
	path, err := localcapture.TriggerSnapshotHTTP(rootPath, *detail)
	if err == nil && path != "" {
		fmt.Println(path)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d, _, err := localcapture.StartDaemon(ctx, project.FindCaptureRoot(rootPath), localcapture.DaemonOptions{})
	if err != nil {
		fatal(err)
	}
	defer d.Stop()
	path, err = d.TriggerSnapshotPublic(*detail)
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

	rootPath := project.FindCaptureRoot(*root)
	layout := project.LayoutFor(rootPath)
	disc := discover.Discover(*root)
	fmt.Println("BugIT doctor", version)
	fmt.Println("OS:", runtime.GOOS, runtime.GOARCH)
	fmt.Println("Capture root:", rootPath)
	fmt.Println("BugIT dir:", layout.BugitDir)
	fmt.Println("Public URL:", localcapture.PublicURL(*root))
	fmt.Println("Backend port:", disc.BackendPort)
	if discover.IsListening("127.0.0.1", disc.BackendPort) {
		fmt.Println("OK backend listening on :" + fmt.Sprint(disc.BackendPort))
		if disc.BackendPID > 0 {
			fmt.Println("Backend PID:", disc.BackendPID)
		}
	} else {
		fmt.Println("WARN backend not listening on :" + fmt.Sprint(disc.BackendPort))
	}

	nodeVer := runCmdOutput("node", "-v")
	if nodeVer == "" {
		fmt.Println("WARN node not found — attach capture requires Node >=18")
	} else {
		fmt.Println("Node:", nodeVer)
		if nodeMajor(nodeVer) >= 18 {
			fmt.Println("OK inbound attach capture supported (diagnostics_channel)")
		} else {
			fmt.Println("WARN Node >=18 required for inbound attach capture")
		}
	}

	inspectPort := disc.InspectPort
	if inspectPort <= 0 {
		inspectPort = 9229
	}
	if captureattach.InspectAvailable(inspectPort) {
		fmt.Printf("OK Node inspector on :%d\n", inspectPort)
	} else {
		fmt.Printf("WARN Node inspector not on :%d — BugIT can enable it on attach\n", inspectPort)
	}

	replayBin := resolveReplayBin()
	if _, err := exec.LookPath(replayBin); err != nil {
		if _, statErr := os.Stat(replayBin); statErr != nil {
			fmt.Println("WARN dre-replay not found:", replayBin)
		} else {
			fmt.Println("OK dre-replay:", replayBin)
		}
	} else {
		fmt.Println("OK dre-replay:", replayBin)
	}
}

func runCmdOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func nodeMajor(version string) int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.SplitN(version, ".", 2)
	if len(parts) == 0 {
		return 0
	}
	major, _ := strconv.Atoi(parts[0])
	return major
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
  bugit record [--root PATH]
  bugit capture [--auto] [--root PATH]
  bugit capture [--root PATH] -- <command...>
  bugit replay [--dre PATH] [--root PATH]
  bugit open [--dre PATH] [--root PATH]
  bugit snapshot [--detail TEXT] [--root PATH]
  bugit snapshots list [--root PATH]
  bugit status [--root PATH]
  bugit doctor [--root PATH]
  bugit version`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
