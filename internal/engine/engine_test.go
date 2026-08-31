package engine

import (
	"bytes"
	"context"
	"os"
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
	if err := eng.Start(context.Background()); err == nil {
		t.Fatal("Start after Close unexpectedly succeeded")
	}
}

func TestConcurrentStartOnlyOneSucceeds(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close(context.Background())
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- eng.Start(context.Background())
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful Starts = %d, want 1", successes)
	}
}

func TestKeyringMutationPersistsBeforeClose(t *testing.T) {
	cfg := config.Default()
	cfg.Listen = "127.0.0.1:0"
	cfg.KeyringPath = filepath.Join(t.TempDir(), "keyring.json")
	eng, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close(context.Background())
	if _, err := eng.IssueBootstrapToken(context.Background(), "write-through", time.Minute); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(cfg.KeyringPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("write-through")) {
		t.Fatal("issued token was not persisted before Close")
	}
}

func TestPersistentKeyringExclusiveLock(t *testing.T) {
	cfg := config.Default()
	cfg.Listen = "127.0.0.1:0"
	cfg.KeyringPath = filepath.Join(t.TempDir(), "keyring.json")
	first, err := New(cfg)
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("second engine acquired same keyring lock")
	}
	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	second, err := New(cfg)
	if err != nil {
		t.Fatalf("New after lock release: %v", err)
	}
	_ = second.Close(context.Background())
}
