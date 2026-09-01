// FasterEdge 开源项目 · https://github.com/FasterEdge · https://gitee.com/FasterEdge
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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
	"github.com/FasterEdge/FasterEdge2Api/internal/engine"
	"github.com/FasterEdge/FasterEdge2Api/internal/server"
)

const version = "1.0.20260902"

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

环境变量: FE2A_LISTEN / FE2A_NODE_NAME / FE2A_ROLE / FE2A_KEYRING_PATH / FE2A_TLS_CERT / FE2A_TLS_KEY / FE2A_SHUTDOWN_TIMEOUT
角色:     cloud | edge | neutral`)
	return nil
}

func closeEngine(eng *engine.Engine, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()
	return eng.Close(ctx)
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
	tlsCert := fs.String("tls-cert", cfg.TLSCert, "TLS 证书路径(需与 --tls-key 同时提供)")
	tlsKey := fs.String("tls-key", cfg.TLSKey, "TLS 私钥路径(需与 --tls-cert 同时提供)")
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
	cfg.TLSCert = *tlsCert
	cfg.TLSKey = *tlsKey

	eng, err := engine.New(cfg)
	if err != nil {
		return err
	}
	parent, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := eng.Start(parent); err != nil {
		_ = closeEngine(eng, cfg.ShutdownTimeout)
		return err
	}

	srv := server.New(eng, server.Options{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeTLS(cfg.Listen, cfg.TLSCert, cfg.TLSKey)
	}()

	select {
	case serveErr := <-errCh:
		closeErr := closeEngine(eng, cfg.ShutdownTimeout)
		if serveErr != nil && !errors.Is(serveErr, context.Canceled) && !errors.Is(serveErr, http.ErrServerClosed) {
			return errors.Join(serveErr, closeErr)
		}
		return closeErr
	case <-parent.Done():
		srv.Logger().Println("shutting down ...")
		shutdownErr := srv.Shutdown(cfg.ShutdownTimeout)
		closeErr := closeEngine(eng, cfg.ShutdownTimeout)
		return errors.Join(shutdownErr, closeErr)
	}
}

func cmdToken(args []string) (retErr error) {
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
	if *subject != "admin" {
		return errors.New("offline token bootstrap only permits subject admin; issue node tokens through the authenticated HTTP API")
	}
	cfg.KeyringPath = *keyring
	if cfg.KeyringPath == "" {
		return errors.New("token requires --keyring (or FE2A_KEYRING_PATH) so the server can reuse the credential")
	}
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
	defer func() { retErr = errors.Join(retErr, closeEngine(eng, cfg.ShutdownTimeout)) }()

	tok, err := eng.IssueBootstrapToken(context.Background(), *subject, ttl)
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}
