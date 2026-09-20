package forward

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bugit/dre-engine/api/grpcapi"
	"github.com/bugit/dre-engine/api/ioevent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	addr   string
	nodeID string
	conn   *grpc.ClientConn
	mu     sync.Mutex
}

func New(addr, nodeID string) *Client {
	return &Client{addr: addr, nodeID: nodeID}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return nil
	}
	opts := append(grpcapi.DialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	conn, err := grpc.DialContext(ctx, c.addr, opts...)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
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
	msg := grpcapi.IOEventFromNative(evt, c.nodeID)
	if err := stream.Send(msg); err != nil {
		return err
	}
	_, err = stream.CloseAndRecv()
	return err
}

func (c *Client) RunForwarder(ctx context.Context, events <-chan ioevent.IOEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.SendEvent(sendCtx, evt)
			cancel()
			if err != nil {
				log.Printf("forward error: %v", err)
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
