// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package server

import (
	"net/http"
	"strings"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
)

type setRoleReq struct {
	Role string `json:"role"`
}

func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	// 2Api 角色由 engine 持有。核心 get_role 仅反映已下发的 cloud|edge 角色,
	// neutral 节点从未向核心下发角色, 直接回显 engine 角色避免空值。
	writeData(w, string(s.engine.Role()))
}

// handleSetRole 仅接受当前启动角色。角色能力在挂载时确定,运行期切换会造成不一致。
func (s *Server) handleSetRole(w http.ResponseWriter, r *http.Request) {
	var req setRoleReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	role := config.Role(strings.ToLower(strings.TrimSpace(req.Role)))
	if role != config.RoleCloud && role != config.RoleEdge && role != config.RoleNeutral {
		writeError(w, http.StatusBadRequest, "role must be cloud, edge, or neutral")
		return
	}
	if role != s.engine.Role() {
		writeError(w, http.StatusConflict, "runtime role is immutable; restart with --role to change it")
		return
	}
	// 核心框架将 set_role 限制为本地进程内调用(角色能力在挂载时确定),
	// 运行期角色不可变: 与当前角色一致时直接回显, 不再下发 set_role。
	writeData(w, string(role))
}
