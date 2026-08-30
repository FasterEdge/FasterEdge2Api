package server

import (
	"net/http"

	"github.com/FasterEdge/FasterEdge/ability"
)

// setRoleReq 是设置角色的请求体。
type setRoleReq struct {
	Role string `json:"role"`
}

// handleGetRole 返回当前节点角色。
func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "RoleAbility", ability.CommandGetRole, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleSetRole 设置当前节点角色。
// 注意:已注册 Cloud/EdgeRoleAbility 的节点应保持其初始角色,否则后续命令会因角色不匹配而失败。
func (s *Server) handleSetRole(w http.ResponseWriter, r *http.Request) {
	var req setRoleReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "RoleAbility", ability.CommandSetRole, ability.RoleAbilityArgs{Role: req.Role})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}
