// Package engine 负责组装 FasterEdge Atom 并把它暴露给 HTTP API 层。
// 它根据配置决定本节点的集群角色,注册对应的角色能力,
// 并统一通过 AuthenticatedCommandContext 执行远程调用(OneKey 认证)。
package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	fasteredge "github.com/FasterEdge/FasterEdge"
	"github.com/FasterEdge/FasterEdge/ability"
	"github.com/FasterEdge/FasterEdge/data"
	"github.com/FasterEdge/FasterEdge/types"

	"github.com/FasterEdge/FasterEdge2Api/internal/config"
)

// Engine 持有已装配的 Atom 并管理其生命周期。
type Engine struct {
	cfg  config.Config
	atom *types.Atom

	runCtx    context.Context
	cancelRun context.CancelFunc
	runDone   chan error
	started   bool
}

// New 根据配置构建 Atom:
//  1. 注册基础组件(BaseData / BaseAbility / NetMapData / KeyringData);
//  2. 注册集群能力(RoleAbility / NetMapAbility / OneKeyAbility);
//  3. 按角色注册 CloudRoleAbility 或 EdgeRoleAbility(neutral 则都不注册);
//  4. 设置节点名与角色,安装 OneKey 命令认证器,挂载全部组件。
func New(cfg config.Config) (*Engine, error) {
	cfg.Role = normalizeRole(cfg.Role)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	atom := fasteredge.InitAtom()
	if err := populateAtom(atom, cfg); err != nil {
		return nil, err
	}
	return &Engine{cfg: cfg, atom: atom}, nil
}

// populateAtom 注册数据、能力、角色并挂载。
func populateAtom(atom *types.Atom, cfg config.Config) error {
	if err := atom.AddData(data.NewNetMapData()); err != nil {
		return err
	}
	kr := data.NewKeyringData()
	if path := cfg.KeyringPath; path != "" {
		kr = data.NewPersistentKeyringData(path)
	}
	if err := atom.AddData(kr); err != nil {
		return err
	}
	if err := atom.AddAbility(&ability.RoleAbility{}); err != nil {
		return err
	}
	if err := atom.AddAbility(ability.NewNetMapAbility()); err != nil {
		return err
	}
	if err := atom.AddAbility(ability.NewOneKeyAbility()); err != nil {
		return err
	}

	// 配置节点名(挂载前即可执行,NetMapData 仅依赖自身)。
	if out := atom.CommandContext(context.Background(), "NetMapData",
		data.NetMapCommandSetNodeName, data.NetMapSetNodeNameArgs{Name: cfg.NodeName}); out.Err != nil {
		return fmt.Errorf("set node name: %w", out.Err)
	}

	// 设置角色后再注册角色能力,保证 Cloud/EdgeRoleAbility 挂载检查通过。
	switch cfg.Role {
	case config.RoleCloud:
		if out := atom.CommandContext(context.Background(), "RoleAbility",
			ability.CommandSetRole, ability.RoleAbilityArgs{Role: "cloud"}); out.Err != nil {
			return fmt.Errorf("set cloud role: %w", out.Err)
		}
		if err := atom.AddAbility(ability.NewCloudRoleAbility()); err != nil {
			return err
		}
	case config.RoleEdge:
		if out := atom.CommandContext(context.Background(), "RoleAbility",
			ability.CommandSetRole, ability.RoleAbilityArgs{Role: "edge"}); out.Err != nil {
			return fmt.Errorf("set edge role: %w", out.Err)
		}
		if err := atom.AddAbility(ability.NewEdgeRoleAbility()); err != nil {
			return err
		}
	}

	// 安装 OneKey 命令认证器:所有远程调用必须携带有效令牌。
	oneKey, ok := atom.Ability("OneKeyAbility")
	if !ok {
		return errors.New("onekey ability not registered")
	}
	auth, ok := oneKey.(types.CommandAuthenticator)
	if !ok {
		return errors.New("onekey ability does not implement CommandAuthenticator")
	}
	if err := atom.SetCommandAuthenticator(auth); err != nil {
		return err
	}

	// 挂载全部组件(拓扑排序,按依赖顺序)。
	if err := atom.PreRun(); err != nil {
		return fmt.Errorf("mount atom: %w", err)
	}

	// 持久化 Keyring 时立即保存初始快照:
	// 若快照文件尚不存在(首次运行),把挂载时生成的密钥落盘,
	// 这样后续 CLI / 其他进程读取到的是同一份密钥。
	if path := cfg.KeyringPath; path != "" {
		if kr, ok := atom.Data("KeyringData"); ok {
			if kd, ok := kr.(*data.KeyringData); ok {
				if err := kd.SaveSnapshot(path); err != nil {
					return fmt.Errorf("persist initial keyring snapshot: %w", err)
				}
			}
		}
	}
	return nil
}

// Atom 返回底层 Atom(仅内部使用)。
func (e *Engine) Atom() *types.Atom { return e.atom }

// Capabilities 返回已注册的集群能力组件名(用于路由能力发现)。
func (e *Engine) Capabilities() []string {
	var names []string
	for name := range e.atom.AllAbilities() {
		names = append(names, name)
	}
	return names
}

// HasAbility 报告指定能力是否已注册并挂载。
func (e *Engine) HasAbility(name string) bool {
	_, ok := e.atom.Ability(name)
	return ok
}

// Role 返回当前节点角色。
func (e *Engine) Role() config.Role { return e.cfg.Role }

// Start 在后台启动 Atom 运行时(监督 Runner 并负责优雅卸载)。
// 返回后错误只能通过 Err() 获取。
func (e *Engine) Start(parent context.Context) error {
	if e.started {
		return errors.New("engine already started")
	}
	if parent == nil {
		return types.ErrNilContext
	}
	ctx, cancel := context.WithCancel(parent)
	e.runCtx = ctx
	e.cancelRun = cancel
	e.runDone = make(chan error, 1)
	e.started = true
	go func() {
		e.runDone <- fasteredge.RunAtom(ctx, e.atom, fasteredge.WithShutdownTimeout(e.cfg.ShutdownTimeout))
	}()
	return nil
}

// Err 返回后台运行时是否已结束及其错误(非阻塞)。
func (e *Engine) Err() error {
	if !e.started {
		return nil
	}
	select {
	case err := <-e.runDone:
		return err
	default:
		return nil
	}
}

// Close 停止后台运行并优雅卸载全部组件,返回关闭错误。
// 若未 Start,则仅执行卸载(用于 CLI 一次性场景)。
func (e *Engine) Close(ctx context.Context) error {
	if e == nil || e.atom == nil {
		return types.ErrNilAtom
	}
	if e.started {
		if e.cancelRun != nil {
			e.cancelRun()
		}
		select {
		case err := <-e.runDone:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fasteredge.CloseAtom(ctx, e.atom, fasteredge.WithShutdownTimeout(e.cfg.ShutdownTimeout))
}

// AuthenticatedCommand 以 OneKey 凭据执行一次受认证的命令调用。
// 认证与组件调度由 Atom 完成;返回命令输出值。
func (e *Engine) AuthenticatedCommand(ctx context.Context, credential ability.OneKeyCredential, component, command string, args any) (any, error) {
	if e == nil || e.atom == nil {
		return nil, types.ErrNilAtom
	}
	out := e.atom.AuthenticatedCommandContext(ctx, credential, component, command, args)
	if out.Err != nil {
		return nil, out.Err
	}
	return out.Value, nil
}

// TrustedCommand 以可信(进程内)方式执行命令,仅用于本机引导(如签发首个令牌)。
// 不应暴露给远程调用方。
func (e *Engine) TrustedCommand(ctx context.Context, component, command string, args any) (any, error) {
	if e == nil || e.atom == nil {
		return nil, types.ErrNilAtom
	}
	out := e.atom.CommandContext(ctx, component, command, args)
	if out.Err != nil {
		return nil, out.Err
	}
	return out.Value, nil
}

// IssueBootstrapToken 通过可信管道签发首个访问令牌(CLI 引导用)。
// subject 为目标主体,ttl 为有效期(<=0 使用 KeyringData 默认值)。
// 返回传输编码形式(可直接放入 Authorization: Bearer)。
func (e *Engine) IssueBootstrapToken(ctx context.Context, subject string, ttl time.Duration) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	val, err := e.TrustedCommand(ctx, "OneKeyAbility", ability.OneKeyCommandIssueToken,
		ability.OneKeyIssueTokenArgs{Subject: subject, TTL: ttl})
	if err != nil {
		return "", err
	}
	tok, ok := val.(ability.OneKeyToken)
	if !ok {
		return "", errors.New("issue token: unexpected output type")
	}
	return ability.EncodeForTransmission(tok), nil
}

func normalizeRole(r config.Role) config.Role {
	switch r {
	case config.RoleCloud, config.RoleEdge, config.RoleNeutral:
		return r
	}
	return config.RoleNeutral
}