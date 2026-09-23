package captureattach

import (
	"context"
)

// ServerTap captures inbound HTTP via diagnostics_channel injection on a shared AttachTap session.
type ServerTap struct {
	attach *AttachTap
}

// StartServerTap starts hybrid attach capture (same as StartAttachTap).
func StartServerTap(ctx context.Context, opts AttachOptions) (*ServerTap, error) {
	attach, err := StartAttachTap(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &ServerTap{attach: attach}, nil
}

func (s *ServerTap) InboundCount() int64 {
	if s.attach == nil {
		return 0
	}
	return s.attach.InboundCount()
}

func (s *ServerTap) Close() {
	if s.attach != nil {
		s.attach.Close()
	}
}
