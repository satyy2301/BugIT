//go:build linux

package bpf

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// Collection holds loaded eBPF objects and attached tracepoint links.
type Collection struct {
	objs  bpfObjects
	links []link.Link
}

// LoadCollection loads BPF objects and attaches syscall/sched tracepoints.
func LoadCollection() (*Collection, error) {
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("load bpf objects: %w", err)
	}

	c := &Collection{objs: objs}
	if err := c.attach(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Collection) attach() error {
	progs := []struct {
		group string
		name  string
		prog  *ebpf.Program
	}{
		{"syscalls", "sys_enter_read", c.objs.DreTraceRead},
		{"syscalls", "sys_enter_write", c.objs.DreTraceWrite},
		{"syscalls", "sys_enter_recvfrom", c.objs.DreTraceRecvfrom},
		{"syscalls", "sys_enter_sendto", c.objs.DreTraceSendto},
		{"sched", "sched_switch", c.objs.DreTraceSchedSwitch},
	}

	for _, tp := range progs {
		if tp.prog == nil {
			return fmt.Errorf("missing program for %s/%s", tp.group, tp.name)
		}
		l, err := link.Tracepoint(tp.group, tp.name, tp.prog, nil)
		if err != nil {
			return fmt.Errorf("attach %s/%s: %w", tp.group, tp.name, err)
		}
		c.links = append(c.links, l)
	}
	return nil
}

// EventsMap returns the ring buffer map.
func (c *Collection) EventsMap() *ebpf.Map {
	return c.objs.DreEvents
}

// SetBypass toggles kernel-side probe bypass (1 = bypassed).
func (c *Collection) SetBypass(enabled bool) error {
	if c.objs.DreBypass == nil {
		return fmt.Errorf("bypass map not loaded")
	}
	key := uint32(0)
	val := uint32(0)
	if enabled {
		val = 1
	}
	return c.objs.DreBypass.Put(key, val)
}

// Close detaches tracepoints and releases BPF resources.
func (c *Collection) Close() {
	for _, l := range c.links {
		l.Close()
	}
	c.links = nil
	_ = c.objs.Close()
}
