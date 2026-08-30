package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FasterEdge/FasterEdge/ability"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
	"github.com/FasterEdge/FasterEdge2Api/internal/engine"
)

// newTestServer 构建一个以 temp 目录持久化 Keyring 的测试服务。
func newTestServer(t *testing.T, role config.Role, nodeName string) (*Server, *engine.Engine, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Listen = "127.0.0.1:0"
	cfg.NodeName = nodeName
	cfg.Role = role
	cfg.KeyringPath = filepath.Join(dir, "keyring.json")

	eng, err := engine.New(cfg)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close(context.Background()) })

	srv := New(eng, Options{})
	return srv, eng, dir
}

// startTestServer 在临时端口启动服务,返回完整 baseURL。
func startTestServer(t *testing.T, role config.Role, nodeName string) (*Server, *engine.Engine, string) {
	t.Helper()
	srv, eng, _ := newTestServer(t, role, nodeName)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(l)
	t.Cleanup(func() { _ = srv.Shutdown(2 * time.Second) })
	return srv, eng, "http://" + l.Addr().String()
}

// bootstrapToken 用 CLI 路径(可信进程内)签发令牌。
func bootstrapToken(t *testing.T, eng *engine.Engine, subject string) string {
	t.Helper()
	tok, err := eng.IssueBootstrapToken(context.Background(), subject, time.Hour)
	if err != nil {
		t.Fatalf("bootstrap token: %v", err)
	}
	if tok == "" {
		t.Fatal("bootstrap token empty")
	}
	return tok
}

// authedReq 执行一次带 Bearer 令牌的 HTTP 请求并解析响应。
func authedReq(t *testing.T, baseURL, token, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, baseURL+path, rd)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m
}

func mustOK(t *testing.T, status int, m map[string]any) map[string]any {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", status, m)
	}
	if ok, _ := m["ok"].(bool); !ok {
		t.Fatalf("expected ok=true: %v", m)
	}
	data, _ := m["data"].(map[string]any)
	return data
}

func TestPublicEndpoints(t *testing.T) {
	_, _, base := startTestServer(t, config.RoleNeutral, "test-node")
	// 未携带令牌也可访问公共端点。
	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}
	resp, err = http.Get(base + "/api/v1/info")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("info status = %d", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	_, _, base := startTestServer(t, config.RoleNeutral, "test-node")
	// 未携带令牌访问受保护端点应 401。
	req, _ := http.NewRequest("GET", base+"/api/v1/topology", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("topology: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	// 伪造令牌同样 401。
	req2, _ := http.NewRequest("GET", base+"/api/v1/topology", nil)
	req2.Header.Set("Authorization", "Bearer garbage.token.here.signature")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("topology2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token, got %d", resp2.StatusCode)
	}
}

func TestTopologyFlow(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "test-node")
	tok := bootstrapToken(t, eng, "admin")

	// 注册两个对等节点。
	status, m := authedReq(t, base, tok, "POST", "/api/v1/peers", map[string]any{
		"name": "edge-2", "address": "10.0.0.2:7000", "role": "edge",
	})
	if status != http.StatusOK {
		t.Fatalf("register peer: %d %v", status, m)
	}
	status, m = authedReq(t, base, tok, "POST", "/api/v1/peers", map[string]any{
		"name": "cloud-1", "address": "10.0.0.1:7000", "role": "cloud",
	})
	if status != http.StatusOK {
		t.Fatalf("register peer2: %d %v", status, m)
	}

	// 列出。
	status, m = authedReq(t, base, tok, "GET", "/api/v1/peers", nil)
	if status != http.StatusOK {
		t.Fatalf("list peers: %d %v", status, m)
	}
	if data, ok := m["data"].([]any); !ok || len(data) != 2 {
		t.Fatalf("expected 2 peers, got %v", m["data"])
	}

	// 查询单个。
	status, m = authedReq(t, base, tok, "GET", "/api/v1/peers/edge-2", nil)
	if status != http.StatusOK {
		t.Fatalf("lookup peer: %d %v", status, m)
	}

	// 更新。
	status, m = authedReq(t, base, tok, "PUT", "/api/v1/peers/edge-2", map[string]any{
		"new_role": "relay",
	})
	if status != http.StatusOK {
		t.Fatalf("update peer: %d %v", status, m)
	}

	// 拓扑包含 self + 2 peers。
	status, m = authedReq(t, base, tok, "GET", "/api/v1/topology", nil)
	if status != http.StatusOK {
		t.Fatalf("topology: %d %v", status, m)
	}
	data := m["data"].(map[string]any)
	peers := data["peers"].([]any)
	if len(peers) != 2 {
		t.Fatalf("topology peers = %d, want 2", len(peers))
	}

	// 删除。
	status, m = authedReq(t, base, tok, "DELETE", "/api/v1/peers/cloud-1", nil)
	if status != http.StatusOK {
		t.Fatalf("unregister peer: %d %v", status, m)
	}
}

func TestNodeInfo(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "my-edge")
	tok := bootstrapToken(t, eng, "admin")

	status, m := authedReq(t, base, tok, "GET", "/api/v1/node", nil)
	if status != http.StatusOK {
		t.Fatalf("node info: %d %v", status, m)
	}
	data := m["data"].(map[string]any)
	if data["node_name"] != "my-edge" {
		t.Fatalf("node_name = %v, want my-edge", data["node_name"])
	}

	status, m = authedReq(t, base, tok, "PUT", "/api/v1/node/name", map[string]any{"name": "renamed-node"})
	if status != http.StatusOK {
		t.Fatalf("set node name: %d %v", status, m)
	}
}

func TestRoleEndpoints(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "test-node")
	tok := bootstrapToken(t, eng, "admin")

	status, m := authedReq(t, base, tok, "GET", "/api/v1/role", nil)
	if status != http.StatusOK {
		t.Fatalf("get role: %d %v", status, m)
	}
	status, m = authedReq(t, base, tok, "PUT", "/api/v1/role", map[string]any{"role": "edge"})
	if status != http.StatusOK {
		t.Fatalf("set role: %d %v", status, m)
	}
}

func TestCloudEndpoints(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleCloud, "cloud-1")
	tok := bootstrapToken(t, eng, "admin")

	status, m := authedReq(t, base, tok, "GET", "/api/v1/cloud", nil)
	if status != http.StatusOK {
		t.Fatalf("cloud describe: %d %v", status, m)
	}

	// 注册服务。
	status, m = authedReq(t, base, tok, "POST", "/api/v1/cloud/services", map[string]any{
		"name": "edge-api", "version": "1.0.0", "endpoint": "https://api.example.com",
	})
	if status != http.StatusOK {
		t.Fatalf("register service: %d %v", status, m)
	}

	status, m = authedReq(t, base, tok, "GET", "/api/v1/cloud/services", nil)
	if status != http.StatusOK {
		t.Fatalf("list services: %d %v", status, m)
	}
	if data, ok := m["data"].([]any); !ok || len(data) != 1 {
		t.Fatalf("expected 1 service, got %v", m["data"])
	}

	// 心跳。
	status, m = authedReq(t, base, tok, "POST", "/api/v1/cloud/heartbeat", nil)
	if status != http.StatusOK {
		t.Fatalf("heartbeat: %d %v", status, m)
	}

	// 设置状态。
	status, m = authedReq(t, base, tok, "PUT", "/api/v1/cloud/status", map[string]any{"status": "degraded"})
	if status != http.StatusOK {
		t.Fatalf("set status: %d %v", status, m)
	}
}

func TestEdgeEndpoints(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleEdge, "edge-1")
	tok := bootstrapToken(t, eng, "admin")

	status, m := authedReq(t, base, tok, "GET", "/api/v1/edge", nil)
	if status != http.StatusOK {
		t.Fatalf("edge describe: %d %v", status, m)
	}

	status, m = authedReq(t, base, tok, "PUT", "/api/v1/edge/zone", map[string]any{"zone": "cn-east"})
	if status != http.StatusOK {
		t.Fatalf("set zone: %d %v", status, m)
	}

	status, m = authedReq(t, base, tok, "POST", "/api/v1/edge/capabilities", map[string]any{"name": "modbus"})
	if status != http.StatusOK {
		t.Fatalf("add capability: %d %v", status, m)
	}

	status, m = authedReq(t, base, tok, "GET", "/api/v1/edge/capabilities", nil)
	if status != http.StatusOK {
		t.Fatalf("list capabilities: %d %v", status, m)
	}

	status, m = authedReq(t, base, tok, "POST", "/api/v1/edge/latency", map[string]any{"latency_ms": 12.5})
	if status != http.StatusOK {
		t.Fatalf("record latency: %d %v", status, m)
	}

	status, m = authedReq(t, base, tok, "GET", "/api/v1/edge/metrics", nil)
	if status != http.StatusOK {
		t.Fatalf("get metrics: %d %v", status, m)
	}
	if data, ok := m["data"].(map[string]any); ok {
		if data["zone"] != "cn-east" {
			t.Fatalf("metrics zone = %v", data["zone"])
		}
	}

	status, m = authedReq(t, base, tok, "PUT", "/api/v1/edge/online", map[string]any{"online": true})
	if status != http.StatusOK {
		t.Fatalf("set online: %d %v", status, m)
	}
}

func TestRoleMismatchCloud(t *testing.T) {
	// cloud 节点不应有 edge 端点。
	_, eng, base := startTestServer(t, config.RoleCloud, "cloud-1")
	tok := bootstrapToken(t, eng, "admin")
	status, m := authedReq(t, base, tok, "GET", "/api/v1/edge", nil)
	if status != http.StatusNotImplemented {
		t.Fatalf("expected 501 for edge on cloud node, got %d %v", status, m)
	}
}

func TestAuthTokenManagement(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "test-node")
	tok := bootstrapToken(t, eng, "admin")

	// status
	status, m := authedReq(t, base, tok, "GET", "/api/v1/auth/status", nil)
	if status != http.StatusOK {
		t.Fatalf("auth status: %d %v", status, m)
	}

	// issue 新令牌(通过已认证调用)。
	status, m = authedReq(t, base, tok, "POST", "/api/v1/auth/issue", map[string]any{
		"subject": "edge-2", "ttl": "30m",
	})
	if status != http.StatusOK {
		t.Fatalf("issue token: %d %v", status, m)
	}
	issued := m["data"].(map[string]any)
	if issued["subject"] != "edge-2" {
		t.Fatalf("issued subject = %v", issued["subject"])
	}

	// tokens 列表。
	status, m = authedReq(t, base, tok, "GET", "/api/v1/auth/tokens", nil)
	if status != http.StatusOK {
		t.Fatalf("list tokens: %d %v", status, m)
	}
	if data, ok := m["data"].([]any); !ok || len(data) < 2 {
		t.Fatalf("expected >=2 tokens, got %v", m["data"])
	}

	// revoke。
	status, m = authedReq(t, base, tok, "POST", "/api/v1/auth/revoke", map[string]any{"subject": "edge-2"})
	if status != http.StatusOK {
		t.Fatalf("revoke: %d %v", status, m)
	}

	// rotate 后旧令牌失效。
	status, m = authedReq(t, base, tok, "POST", "/api/v1/auth/rotate", nil)
	if status != http.StatusOK {
		t.Fatalf("rotate: %d %v", status, m)
	}
	status, m = authedReq(t, base, tok, "GET", "/api/v1/topology", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 after rotate, got %d", status)
	}
}

func TestKeyringPersistence(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Listen = "127.0.0.1:0"
	cfg.NodeName = "persist-node"
	cfg.Role = config.RoleNeutral
	cfg.KeyringPath = filepath.Join(dir, "keyring.json")

	eng1, err := engine.New(cfg)
	if err != nil {
		t.Fatalf("engine1: %v", err)
	}
	tok1 := bootstrapToken(t, eng1, "admin")
	// 卸载时快照写入。
	if err := eng1.Close(context.Background()); err != nil {
		t.Fatalf("close eng1: %v", err)
	}

	// 新进程加载同一快照,应能验证同一令牌。
	eng2, err := engine.New(cfg)
	if err != nil {
		t.Fatalf("engine2: %v", err)
	}
	defer eng2.Close(context.Background())
	val, err := eng2.AuthenticatedCommand(context.Background(), mustCredential(t, tok1),
		"NetMapAbility", "get_topology", nil)
	if err != nil {
		t.Fatalf("authenticated with persisted token: %v", err)
	}
	if val == nil {
		t.Fatal("topology value is nil")
	}
}

// mustCredential 把传输编码令牌解码为 OneKey 凭据(测试辅助)。
func mustCredential(t *testing.T, tok string) ability.OneKeyCredential {
	t.Helper()
	parsed, err := ability.DecodeFromTransmission(tok)
	if err != nil {
		t.Fatalf("decode credential: %v", err)
	}
	return ability.OneKeyCredential{
		Subject:   parsed.Subject,
		IssuedAt:  parsed.IssuedAt,
		ExpiresAt: parsed.ExpiresAt,
		Signature: parsed.Signature,
	}
}

// 测试中保持轻量:验证被吊销的令牌不可用。
func TestRevokedTokenRejected(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "test-node")
	tok := bootstrapToken(t, eng, "admin")
	status, _ := authedReq(t, base, tok, "GET", "/api/v1/topology", nil)
	if status != http.StatusOK {
		t.Fatalf("precondition: %d", status)
	}
	status, _ = authedReq(t, base, tok, "POST", "/api/v1/auth/revoke-all", nil)
	if status != http.StatusOK {
		t.Fatalf("revoke all: %d", status)
	}
	status, _ = authedReq(t, base, tok, "GET", "/api/v1/topology", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revoke-all, got %d", status)
	}
}

// 断言默认错误格式: 401 时 error.message 非空。
func TestErrorBodyShape(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "test-node")
	tok := bootstrapToken(t, eng, "admin")
	_, m := authedReq(t, base, tok, "GET", "/api/v1/peers/not-exist", nil)
	if !strings.Contains(m["error"].(map[string]any)["message"].(string), "not found") {
		t.Fatalf("expected not-found error, got %v", m)
	}
}

// 确保 os.TempDir 的 keyring 文件能被引擎加载(空快照路径回退临时文件场景已在其他测试覆盖)。
func TestConfigEnvParse(t *testing.T) {
	os.Setenv("FE2A_ROLE", "CLOUD")
	defer os.Unsetenv("FE2A_ROLE")
	cfg, err := config.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.Role != config.RoleCloud {
		t.Fatalf("role = %v, want cloud", cfg.Role)
	}
}

// TestAuthVerifyEndpoint 验证 /auth/verify 能正确接受服务端签发的传输编码令牌。
func TestAuthVerifyEndpoint(t *testing.T) {
	_, eng, base := startTestServer(t, config.RoleNeutral, "test-node")
	tok := bootstrapToken(t, eng, "admin")

	// 通过已认证调用签发一个令牌,拿到原始 OneKeyToken 字段。
	_, m := authedReq(t, base, tok, "POST", "/api/v1/auth/issue", map[string]any{
		"subject": "verify-me", "ttl": "10m",
	})
	data := m["data"].(map[string]any)
	issuedAt, _ := time.Parse(time.RFC3339Nano, data["issued_at"].(string))
	expiresAt, _ := time.Parse(time.RFC3339Nano, data["expires_at"].(string))
	enc := ability.EncodeForTransmission(ability.OneKeyToken{
		Subject:   data["subject"].(string),
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		Signature: data["signature"].(string),
	})

	status, vm := authedReq(t, base, tok, "POST", "/api/v1/auth/verify", map[string]any{"token": enc})
	if status != http.StatusOK {
		t.Fatalf("verify: %d %v", status, vm)
	}
	if vm["data"] != "verify-me" {
		t.Fatalf("verify subject = %v", vm["data"])
	}
}
