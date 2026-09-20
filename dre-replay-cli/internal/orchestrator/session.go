package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/archive"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/debugger"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/delve"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/proxy"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/timefreeze"
)

// Session coordinates proxy, debugger, time-freeze, and optional Delve.
type Session struct {
	Snap   *archive.Snapshot
	Config config.ReplayConfig

	proxyOverride string
	binary        string
	delveAddr     string

	mu     sync.Mutex
	cursor int

	proxySrv *proxy.MultiServer
	dbg      *debugger.Server
	timeEng  *timefreeze.Engine
	dlv      *delve.Bridge
	target   *exec.Cmd
	stopped  bool
}

type Options struct {
	ProxyAddrOverride string
	Binary            string
	DelveAddr         string
}

func New(snap *archive.Snapshot, cfg config.ReplayConfig, opts Options) *Session {
	if opts.DelveAddr == "" {
		opts.DelveAddr = "127.0.0.1:2345"
	}
	return &Session{
		Snap:          snap,
		Config:        cfg,
		proxyOverride: opts.ProxyAddrOverride,
		binary:        opts.Binary,
		delveAddr:     opts.DelveAddr,
	}
}

func (s *Session) Cursor() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor
}

func (s *Session) Seek(index int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 {
		index = 0
	}
	if index > len(s.Snap.Events) {
		index = len(s.Snap.Events)
	}
	s.cursor = index
	s.onCursorChange()
	return s.cursor
}

func (s *Session) Step(delta int) int {
	return s.Seek(s.Cursor() + delta)
}

func (s *Session) Events() []ioevent.IOEvent {
	return s.Snap.Events
}

func (s *Session) onCursorChange() {
	if s.proxySrv != nil {
		s.proxySrv.SetCursor(s.cursor)
	}
	if s.timeEng != nil {
		s.timeEng.SyncToCursor(s.cursor, s.Snap.Events)
	}
}

func (s *Session) Start(ctx context.Context) error {
	defaultAddr := s.Config.ProxyAddr
	if s.proxyOverride != "" {
		defaultAddr = s.proxyOverride
	}

	s.proxySrv = proxy.NewMulti(defaultAddr, s.Config.Services, s.Snap.Events, func() int { return s.Cursor() })
	if err := s.proxySrv.Start(); err != nil {
		return fmt.Errorf("proxy: %w", err)
	}

	s.timeEng = timefreeze.NewEngine(s.Snap.ClockTimeline, s.Config.TimeFreeze)
	if s.Config.TimeFreeze.Enabled {
		s.timeEng.SyncToCursor(0, s.Snap.Events)
	}

	s.dbg = debugger.New(s.Config.DebugAddr, s)
	s.dbg.SetTimeEngine(s.timeEng)
	if err := s.dbg.Start(); err != nil {
		return fmt.Errorf("debugger: %w", err)
	}

	if s.binary != "" {
		if err := s.startTarget(); err != nil {
			log.Printf("target launch: %v", err)
		}
		if s.dlv == nil {
			s.dlv = delve.New()
		}
		if err := s.dlv.StartHeadless(s.binary, s.delveAddr); err != nil {
			log.Printf("delve: %v", err)
		} else {
			log.Printf("delve headless on %s", s.delveAddr)
		}
	}

	log.Printf("replay session ready: manifest=%s events=%d proxy=%s debug=%s",
		s.Snap.Manifest.ID, len(s.Snap.Events), defaultAddr, s.DebugAddr())

	if ctx != nil {
		go func() {
			<-ctx.Done()
			s.Stop()
		}()
	}
	return nil
}

func (s *Session) startTarget() error {
	cmd := exec.Command(s.binary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if s.Config.TimeFreeze.Enabled && s.Config.TimeFreeze.ShimPath != "" {
		cmd.Env = append(cmd.Env, "LD_PRELOAD="+s.Config.TimeFreeze.ShimPath)
		if ts := s.timeEng.FrozenTimestampNs(); ts > 0 {
			cmd.Env = append(cmd.Env, fmt.Sprintf("DRE_FROZEN_TIME_NS=%d", ts))
		}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.target = cmd
	return nil
}

func (s *Session) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	if s.dbg != nil {
		s.dbg.Stop()
	}
	if s.dlv != nil {
		s.dlv.Stop()
	}
	if s.target != nil && s.target.Process != nil {
		_ = s.target.Process.Kill()
	}
	if s.proxySrv != nil {
		s.proxySrv.Stop()
	}
}

func (s *Session) DebugAddr() string {
	if s.dbg != nil {
		return s.dbg.Addr()
	}
	return s.Config.DebugAddr
}

func (s *Session) ProxyAddrs() []string {
	if s.proxySrv == nil {
		return nil
	}
	return s.proxySrv.Addrs()
}

func (s *Session) DelveAddr() string {
	return s.delveAddr
}

func (s *Session) FrozenTimestampNs() uint64 {
	if s.timeEng == nil {
		return 0
	}
	return s.timeEng.FrozenTimestampNs()
}
