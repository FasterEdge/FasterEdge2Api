package server

import (
	"net/http"

	"github.com/FasterEdge/FasterEdge/ability"
	"github.com/FasterEdge/FasterEdge/data"
)

// handleLogo 返回框架 logo(公共)。
func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	val, err := s.engine.TrustedCommand(r.Context(), "BaseData", data.CommandLogo, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleInfo 返回框架版本信息(公共)。
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	val, err := s.engine.TrustedCommand(r.Context(), "BaseData", data.CommandInfo, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleComponents 返回已注册组件清单(公共)。
func (s *Server) handleComponents(w http.ResponseWriter, r *http.Request) {
	dataNames, err := s.engine.TrustedCommand(r.Context(), "BaseAbility", ability.CommandListDataNames, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	abilityNames, err := s.engine.TrustedCommand(r.Context(), "BaseAbility", ability.CommandListAbilityNames, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, map[string]any{
		"data":      dataNames,
		"abilities": abilityNames,
	})
}
