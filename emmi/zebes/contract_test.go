package zebes

import (
	"context"
	"testing"
	"time"

	"github.com/vlumi/tourian/emmi/contracttest"
)

func TestContract(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CommandTimeout = 300 * time.Millisecond
	cfg.HeartbeatTimeout = 600 * time.Millisecond
	contracttest.Run(t, contracttest.Harness{
		Start:            func(ctx context.Context, url string) { _ = Run(ctx, url, cfg) },
		CommandTimeout:   cfg.CommandTimeout,
		HeartbeatTimeout: cfg.HeartbeatTimeout,
	})
}
