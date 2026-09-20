package server_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bugit/dre-engine/api/grpcapi"
	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-collector/internal/server"
	"github.com/bugit/dre-engine/pkg/drearchive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	collector := server.New(dir, "test-cluster", "ci-test-key", nil)

	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	grpcapi.RegisterEventIngestServer(grpcSrv, collector)
	grpcapi.RegisterCollectorAdminServer(grpcSrv, collector)
	go grpcSrv.Serve(grpcLis)
	defer grpcSrv.Stop()

	conn, err := grpc.Dial(grpcLis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := grpcapi.NewEventIngestClient(conn)
	stream, err := client.StreamEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var evt ioevent.IOEvent
	evt.PidTgid = 42
	evt.TimestampNs = uint64(time.Now().UnixNano())
	evt.Fd = 7
	evt.PayloadLen = 13
	evt.IsWrite = 1
	copy(evt.Payload[:], []byte("HTTP/1.1 500"))
	if err := stream.Send(grpcapi.IOEventFromNative(evt, "node-a")); err != nil {
		t.Fatal(err)
	}
	_ = stream.CloseSend()

	admin := grpcapi.NewCollectorAdminClient(conn)
	resp, err := admin.TriggerSnapshot(context.Background(), &grpcapi.TriggerSnapshotRequest{
		Reason: "manual",
		Detail: "integration test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SnapshotId == "" {
		t.Fatal("expected snapshot id")
	}

	drePath := filepath.Join(dir, "incident-"+resp.SnapshotId+".dre")
	if _, err := os.Stat(drePath); err != nil {
		list, err := admin.ListSnapshots(context.Background(), &grpcapi.Empty{})
		if err != nil || len(list.Snapshots) == 0 {
			t.Fatalf("snapshot file missing: %v", err)
		}
		drePath = list.Snapshots[len(list.Snapshots)-1].Path
	}

	snap, err := drearchive.OpenFile(drePath, "ci-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Manifest.EventCount == 0 {
		t.Fatal("expected events in snapshot")
	}
	if snap.Manifest.Trigger.Type != manifest.TriggerManual &&
		snap.Manifest.Trigger.Type != manifest.TriggerHTTP5xx {
		t.Fatalf("unexpected trigger: %s", snap.Manifest.Trigger.Type)
	}

	fixtureDir := filepath.Join("..", "..", "..", "test", "fixtures", "minimal.dre")
	_ = os.MkdirAll(fixtureDir, 0o755)
	data, _ := os.ReadFile(drePath)
	_ = os.WriteFile(filepath.Join(fixtureDir, "sample.dre"), data, 0o644)
}
