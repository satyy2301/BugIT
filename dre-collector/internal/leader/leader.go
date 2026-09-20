package leader

import (
	"context"
	"log"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// RunElection blocks until ctx cancelled; onLead runs when this pod becomes leader.
func RunElection(ctx context.Context, onLead func(context.Context)) error {
	if os.Getenv("DRE_LEADER_ELECT") != "1" {
		onLead(ctx)
		return nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("leader elect: in-cluster config failed, running standalone: %v", err)
		onLead(ctx)
		return nil
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	id := os.Getenv("HOSTNAME")
	if id == "" {
		id = "dre-collector-local"
	}
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "dre-collector-leader",
			Namespace: envOr("DRE_NAMESPACE", "dre-engine"),
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{Identity: id},
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) { onLead(ctx) },
			OnStoppedLeading: func() { log.Println("leader lost") },
		},
	})
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
