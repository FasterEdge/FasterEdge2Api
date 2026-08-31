package server

import (
	"net/http"
	"strings"

	"github.com/FasterEdge/FasterEdge/ability"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
)

type setRoleReq struct {
	Role string `json:"role"`
}

func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "RoleAbility", ability.CommandGetRole, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
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
	val, err := s.authed(r, "RoleAbility", ability.CommandSetRole, ability.RoleAbilityArgs{Role: string(role)})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}
