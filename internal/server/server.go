// Package server 实现 FasterEdge2Api 的 HTTP 服务层:
// 路由、OneKey 认证中间件、错误映射与 JSON 响应。
package server

import (
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/FasterEdge/FasterEdge/types"

	"github.com/FasterEdge/FasterEdge2Api/internal/engine"
)

// Server 是 FasterEdge2Api 的 HTTP 服务。
type Server struct {
	engine *engine.Engine
	srv    *http.Server
	logger *log.Logger
}

// Options 携带服务创建时的可选参数。
type Options struct {
	Logger *log.Logger
}

// New 创建一个 HTTP 服务。路由注册后即可通过 ListenAndServe 启动。
func New(eng *engine.Engine, opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	s := &Server{engine: eng, logger: logger}
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.srv = &http.Server{
		Handler:           s.recoverMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return s
}

// Handler 返回底层 http.Handler(供测试或嵌入使用)。
func (s *Server) Handler() http.Handler { return s.srv.Handler }

// Logger 返回服务日志器。
func (s *Server) Logger() *log.Logger { return s.logger }

// Serve 在给定 net.Listener 上提供服务,直到被 Shutdown。
func (s *Server) Serve(l net.Listener) error { return s.srv.Serve(l) }

// ListenAndServe 监听 addr 并服务,直到 l 被关闭或发生了致命错误。
func (s *Server) ListenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.logger.Printf("FasterEdge2Api listening on %s (role=%s)", l.Addr(), s.engine.Role())
	return s.Serve(l)
}

// Shutdown 优雅关闭 HTTP 服务。
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := contextWithTimeout(timeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

// ------------------------- 路由注册 -------------------------

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// 公共端点(无需认证)。
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/v1/logo", s.handleLogo)
	mux.HandleFunc("GET /api/v1/info", s.handleInfo)
	mux.HandleFunc("GET /api/v1/components", s.handleComponents)

	// 拓扑(NetMapData / NetMapAbility)。
	mux.HandleFunc("GET /api/v1/topology", s.auth(s.handleTopology))
	mux.HandleFunc("GET /api/v1/node", s.auth(s.handleNodeInfo))
	mux.HandleFunc("PUT /api/v1/node/name", s.auth(s.handleSetNodeName))
	mux.HandleFunc("GET /api/v1/node/interfaces", s.auth(s.handleNodeInterfaces))
	mux.HandleFunc("PUT /api/v1/node/default-interface", s.auth(s.handleSetDefaultIface))
	mux.HandleFunc("GET /api/v1/peers", s.auth(s.handleListPeers))
	mux.HandleFunc("POST /api/v1/peers", s.auth(s.handleRegisterPeer))
	mux.HandleFunc("GET /api/v1/peers/{name}", s.auth(s.handleLookupPeer))
	mux.HandleFunc("PUT /api/v1/peers/{name}", s.auth(s.handleUpdatePeer))
	mux.HandleFunc("DELETE /api/v1/peers/{name}", s.auth(s.handleUnregisterPeer))

	// 角色(RoleAbility)。
	mux.HandleFunc("GET /api/v1/role", s.auth(s.handleGetRole))
	mux.HandleFunc("PUT /api/v1/role", s.auth(s.handleSetRole))

	// 云端(CloudRoleAbility,仅 role=cloud 时可用)。
	mux.HandleFunc("GET /api/v1/cloud", s.auth(s.handleCloudDescribe))
	mux.HandleFunc("GET /api/v1/cloud/controller", s.auth(s.handleCloudGetController))
	mux.HandleFunc("PUT /api/v1/cloud/controller", s.auth(s.handleCloudSetController))
	mux.HandleFunc("GET /api/v1/cloud/services", s.auth(s.handleCloudListServices))
	mux.HandleFunc("POST /api/v1/cloud/services", s.auth(s.handleCloudRegisterService))
	mux.HandleFunc("DELETE /api/v1/cloud/services/{name}", s.auth(s.handleCloudUnregisterService))
	mux.HandleFunc("GET /api/v1/cloud/status", s.auth(s.handleCloudGetStatus))
	mux.HandleFunc("PUT /api/v1/cloud/status", s.auth(s.handleCloudSetStatus))
	mux.HandleFunc("POST /api/v1/cloud/heartbeat", s.auth(s.handleCloudHeartbeat))

	// 边缘(EdgeRoleAbility,仅 role=edge 时可用)。
	mux.HandleFunc("GET /api/v1/edge", s.auth(s.handleEdgeDescribe))
	mux.HandleFunc("GET /api/v1/edge/zone", s.auth(s.handleEdgeGetZone))
	mux.HandleFunc("PUT /api/v1/edge/zone", s.auth(s.handleEdgeSetZone))
	mux.HandleFunc("GET /api/v1/edge/capabilities", s.auth(s.handleEdgeListCapabilities))
	mux.HandleFunc("POST /api/v1/edge/capabilities", s.auth(s.handleEdgeAddCapability))
	mux.HandleFunc("PUT /api/v1/edge/capabilities", s.auth(s.handleEdgeSetCapabilities))
	mux.HandleFunc("DELETE /api/v1/edge/capabilities/{name}", s.auth(s.handleEdgeRemoveCapability))
	mux.HandleFunc("POST /api/v1/edge/latency", s.auth(s.handleEdgeRecordLatency))
	mux.HandleFunc("GET /api/v1/edge/metrics", s.auth(s.handleEdgeGetMetrics))
	mux.HandleFunc("PUT /api/v1/edge/online", s.auth(s.handleEdgeSetOnline))

	// 认证与令牌管理(OneKeyAbility)。
	mux.HandleFunc("POST /api/v1/auth/issue", s.auth(s.handleAuthIssue))
	mux.HandleFunc("POST /api/v1/auth/verify", s.auth(s.handleAuthVerify))
	mux.HandleFunc("POST /api/v1/auth/revoke", s.auth(s.handleAuthRevoke))
	mux.HandleFunc("POST /api/v1/auth/revoke-all", s.auth(s.handleAuthRevokeAll))
	mux.HandleFunc("GET /api/v1/auth/tokens", s.auth(s.handleAuthListTokens))
	mux.HandleFunc("GET /api/v1/auth/status", s.auth(s.handleAuthStatus))
	mux.HandleFunc("POST /api/v1/auth/rotate", s.auth(s.handleAuthRotate))
}

// ------------------------- 中间件 -------------------------

// recoverMiddleware 兜底 panic,避免单个请求拖垮服务。
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.logger.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, v)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// auth 中间件解析 Authorization: Bearer <token>,把凭据注入请求上下文。
// 凭据为 OneKey 传输编码形式(subject.issuedNanos.expiresNanos.signature)。
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cred, ok := credentialFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}
		ctx := withCredential(r.Context(), cred)
		next(w, r.WithContext(ctx))
	}
}

// handleHealthz 存活探针。
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now()})
}

// errStatus 把 FasterEdge 错误映射为 HTTP 状态码。
func errStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, types.ErrAuthenticationRequired) || errors.Is(err, types.ErrAuthenticationFailed) {
		return http.StatusUnauthorized
	}
	if errors.Is(err, types.ErrInvalidArguments) || errors.Is(err, types.ErrUnsupportedCommand) {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound
		}
		return http.StatusBadRequest
	}
	if errors.Is(err, types.ErrDuplicateComponent) {
		return http.StatusConflict
	}
	if errors.Is(err, types.ErrMissingDependency) || errors.Is(err, types.ErrWrongDependencyType) {
		return http.StatusNotImplemented
	}
	return http.StatusInternalServerError
}

// fmtErr 生成统一的错误响应体。
func fmtErr(status int, err error) map[string]any {
	msg := "internal server error"
	if err != nil {
		msg = err.Error()
	}
	return map[string]any{"ok": false, "error": map[string]any{"code": status, "message": msg}}
}

// authed 从请求上下文取出 OneKey 凭据,执行一次受认证的命令调用并返回输出值。
func (s *Server) authed(r *http.Request, component, command string, args any) (any, error) {
	cred, ok := credentialFromContext(r.Context())
	if !ok {
		return nil, types.ErrAuthenticationRequired
	}
	return s.engine.AuthenticatedCommand(r.Context(), cred, component, command, args)
}
