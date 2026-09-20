package forward

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bugit/dre-engine/api/grpcapi"
	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/pkg/grpctls"
	"github.com/bugit/dre-engine/pkg/vectorclock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	addr   string
	nodeID string
	conn   *grpc.ClientConn
	mu     sync.Mutex
	vec    *vectorclock.Engine
}

func New(addr, nodeID string) *Client {
	return &Client{addr: addr, nodeID: nodeID, vec: vectorclock.New()}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return nil
	}
	opts := []grpc.DialOption{grpc.WithBlock()}
	if grpctls.ClientEnabled() {
		creds, err := grpctls.ClientCredentials()
		if err != nil {
			return err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.DialContext(ctx, c.addr, opts...)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Client) injectVector(evt ioevent.IOEvent) ioevent.IOEvent {
	if evt.IsWrite != 1 || evt.PayloadLen == 0 {
		return evt
	}
	pl := evt.Payload[:evt.PayloadLen]
	updated := vectorclock.MaybeInjectHTTP(pl, c.nodeID, c.vec)
	if len(updated) == len(pl) {
		return evt
	}
	evt.PayloadLen = uint32(len(updated))
	copy(evt.Payload[:], updated)
	return evt
}

func (c *Client) SendEvent(ctx context.Context, evt ioevent.IOEvent) error {
	if c.conn == nil {
		if err := c.Connect(ctx); err != nil {
			return err
		}
	}
	client := grpcapi.NewEventIngestClient(c.conn)
	stream, err := client.StreamEvents(ctx)
	if err != nil {
		return err
	}
	evt = c.injectVector(evt)
	msg := grpcapi.IOEventFromNative(evt, c.nodeID)
	if err := stream.Send(msg); err != nil {
		return err
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}
	_, err = stream.Recv()
	return err
}

func (c *Client) RunForwarder(ctx context.Context, events <-chan ioevent.IOEvent) {
	if err := c.Connect(ctx); err != nil {
		log.Printf("forward connect: %v", err)
		return
	}
	client := grpcapi.NewEventIngestClient(c.conn)
	for {
		stream, err := client.StreamEvents(ctx)
		if err != nil {
			log.Printf("forward stream: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if c.runStream(ctx, stream, events) {
			return
		}
	}
}

func (c *Client) runStream(ctx context.Context, stream grpcapi.EventIngest_StreamEventsClient, events <-chan ioevent.IOEvent) bool {
	for {
		select {
		case <-ctx.Done():
			_ = stream.CloseSend()
			return true
		case evt, ok := <-events:
			if !ok {
				_ = stream.CloseSend()
				return true
			}
			evt = c.injectVector(evt)
			if err := stream.Send(grpcapi.IOEventFromNative(evt, c.nodeID)); err != nil {
				log.Printf("forward send: %v", err)
				_ = stream.CloseSend()
				return false
			}
		}
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func MockForwarder(ctx context.Context, addr, nodeID string, events <-chan ioevent.IOEvent) {
	log.Printf("mock forwarder: node=%s collector=%s", nodeID, addr)
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			log.Printf("mock event: pid_tgid=%d ts=%d fd=%d", evt.PidTgid, evt.TimestampNs, evt.Fd)
		}
	}
}

var ErrNotConnected = fmt.Errorf("grpc client not connected")
