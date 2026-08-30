// Command fasteredge2api 是 FasterEdge 集群拓扑与管理操作的 HTTP API 网关。
//
// 用法:
//
//	fasteredge2api serve                # 启动 HTTP API 服务
//	fasteredge2api token                # 签发首个访问令牌(引导)
//	fasteredge2api version              # 打印版本
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
	"github.com/FasterEdge/FasterEdge2Api/internal/engine"
	"github.com/FasterEdge/FasterEdge2Api/internal/server"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fasteredge2api:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "serve":
		return cmdServe(args[1:])
	case "token":
		return cmdToken(args[1:])
	case "version", "-version", "--version":
		fmt.Println("fasteredge2api", version)
		return nil
	case "help", "-h", "--help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() error {
	fmt.Fprintln(os.Stderr, `FasterEdge2Api - FasterEdge 集群拓扑与管理操作 HTTP API

用法:
  fasteredge2api serve [flags]  启动 HTTP API 服务
  fasteredge2api token [flags]  签发首个访问令牌(引导,进程内可信执行)
  fasteredge2api version        打印版本
  fasteredge2api help           显示本帮助

环境变量: FE2A_LISTEN / FE2A_NODE_NAME / FE2A_ROLE / FE2A_KEYRING_PATH / FE2A_SHUTDOWN_TIMEOUT
角色:     cloud | edge | neutral`)
	return nil
}

func cmdServe(args []string) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", cfg.Listen, "HTTP 监听地址")
	nodeName := fs.String("node-name", cfg.NodeName, "本节点名")
	roleStr := fs.String("role", string(cfg.Role), "集群角色: cloud | edge | neutral")
	keyring := fs.String("keyring", cfg.KeyringPath, "KeyringData 持久化快照路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.Listen = *listen
	cfg.NodeName = *nodeName
	role, err := config.ParseRole(*roleStr)
	if err != nil {
		return err
	}
	cfg.Role = role
	cfg.KeyringPath = *keyring

	eng, err := engine.New(cfg)
	if err != nil {
		return err
	}
	parent, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := eng.Start(parent); err != nil {
		return err
	}

	srv := server.New(eng, server.Options{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(cfg.Listen)
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			_ = eng.Close(context.Background())
			return err
		}
	case <-parent.Done():
		srv.Logger().Println("shutting down ...")
		_ = srv.Shutdown(5 * time.Second)
		ctx, cancelCtx := context.WithTimeout(context.Background(), cfg.ShutdownTimeout+2*time.Second)
		defer cancelCtx()
		if cerr := eng.Close(ctx); cerr != nil {
			srv.Logger().Printf("close engine: %v", cerr)
		}
	}
	return nil
}

func cmdToken(args []string) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("token", flag.ContinueOnError)
	subject := fs.String("subject", "admin", "令牌主体(通常为节点名)")
	ttlStr := fs.String("ttl", "1h", "令牌有效期(Go duration)")
	keyring := fs.String("keyring", cfg.KeyringPath, "KeyringData 持久化快照路径(需与服务端一致)")
	roleStr := fs.String("role", string(cfg.Role), "集群角色: cloud | edge | neutral")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ttl, err := time.ParseDuration(*ttlStr)
	if err != nil {
		return fmt.Errorf("invalid ttl: %w", err)
	}
	cfg.KeyringPath = *keyring
	role, err := config.ParseRole(*roleStr)
	if err != nil {
		return err
	}
	cfg.Role = role

	// 用同一 KeyringData 构建引擎,保证共享密钥一致,从而签发可被服务端验证的令牌。
	eng, err := engine.New(cfg)
	if err != nil {
		return err
	}
	defer eng.Close(context.Background())

	tok, err := eng.IssueBootstrapToken(context.Background(), *subject, ttl)
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}
