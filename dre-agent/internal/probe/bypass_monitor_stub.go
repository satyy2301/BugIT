//go:build !linux

package probe

import "context"

// MonitorBypass is a no-op on non-Linux platforms.
func MonitorBypass(ctx context.Context, loader *Loader) {}
