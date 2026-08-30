package engine

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	cfg := config.Default()
	cfg.Listen = "127.0.0.1:0"
	cfg.NodeName = "engine-test"
	cfg.Role = config.RoleNeutral
	cfg.KeyringPath = filepath.Join(t.TempDir(), "keyring.json")
	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return eng
}

// TestCloseIdempotent 验证 Close 重复调用安全且不会死锁。
func TestCloseIdempotent(t *testing.T) {
	eng := newTestEngine(t)
	if err := eng.Close(context.Background()); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := eng.Close(context.Background()); err != nil {
		t.Fatalf("second Close (idempotent): %v", err)
	}
}

// TestCloseAfterStart 验证启动后 Close 正常取消、无死锁、正常取消不算错误。
func TestCloseAfterStart(t *testing.T) {
	eng := newTestEngine(t)
	if err := eng.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close after Start: %v", err)
	}
	// Err() 不应消费 runDone,重复调用仍可用。
	if err := eng.Err(); err != nil {
		t.Fatalf("Err() after Close: %v", err)
	}
}

// TestConcurrentClose 验证并发 Close 不会死锁或 panic。
func TestConcurrentClose(t *testing.T) {
	eng := newTestEngine(t)
	if err := eng.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = eng.Close(ctx)
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Close deadlocked")
	}
}

// TestCloseBeforeStart 验证未启动时 Close 走卸载路径(CLI 场景)。
func TestCloseBeforeStart(t *testing.T) {
	eng := newTestEngine(t)
	if err := eng.Close(context.Background()); err != nil {
		t.Fatalf("Close before Start: %v", err)
	}
}
