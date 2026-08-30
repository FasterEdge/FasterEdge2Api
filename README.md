<div align="center">
  <img src="https://avatars.githubusercontent.com/u/245985800?s=200&v=4" alt="logo" width="100" />
  <h2>FasterEdge2Api</h2>
  <h3>FasterEdge 集群拓扑与管理操作 HTTP API</h3>
</div>

### 一、项目简介

- **FasterEdge2Api** 是 [FasterEdge](https://github.com/FasterEdge/FasterEdge) 生态的 **HTTP API 网关**:把一个 FasterEdge Atom(节点)装配成可远程管理的服务,对外提供**集群拓扑管理**(NetMap)、**集群管理**(Role / Cloud / Edge)与 **OneKey 令牌认证**。
- 所有管理调用统一走 FasterEdge 的 `AuthenticatedCommandContext`(HMAC-SHA256 令牌),**不暴露本地可信的 `Command` 直调接口**,符合 FasterEdge 的远程安全规范。
- 纯 Go 标准库 `net/http` 实现,单二进制部署;角色、令牌持久化、监听地址均可配置。

### 二、主要特性

| 特性 | 说明 |
|------|------|
| 集群拓扑 | 本节点网络信息 + 对等节点表的查询与增删改(`/api/v1/node`、`/api/v1/peers`、`/api/v1/topology`) |
| 集群管理 | 角色设置,云端(`--role cloud`)/ 边缘(`--role edge`)节点能力全量暴露 |
| 令牌认证 | OneKey HMAC-SHA256 短期令牌,过期 / 吊销 / 密钥轮换后即时失效 |
| Keyring 持久化 | 密钥与令牌表以 `0600` 快照落盘,跨进程 / 重启复用 |
| 统一响应 | `{"ok":true,"data":...}` / `{"ok":false,"error":{...}}`,字段名统一 snake_case |
| 安全防护 | 请求体大小限制、请求/响应超时、panic 兜底中间件 |

### 三、快速开始

> **环境要求**:Go 1.25.5+;`FasterEdge` 通过 `replace` 指向本地路径 `../FasterEdge`。

```bash
cd FasterEdge2Api
go build ./...
go build -o fasteredge2api ./cmd/fasteredge2api
```

**1. 先签发首个访问令牌(引导)**

首次令牌只能通过 CLI 以进程内可信方式签发,并把共享密钥落盘:

```bash
./fasteredge2api token --subject admin --ttl 24h --keyring ./data/keyring.json
# 输出: admin.<issuedNanos>.<expiresNanos>.<signature>
```

**2. 再启动服务**(必须与 `token` 使用**同一** `--keyring` 路径)

```bash
# 云端节点
./fasteredge2api serve --listen :8080 --node-name cloud-1 --role cloud \
  --keyring ./data/keyring.json

# 边缘节点
./fasteredge2api serve --listen :8081 --node-name edge-1 --role edge --keyring ./data/keyring.json

# 中立节点(仅拓扑 + 角色)
./fasteredge2api serve --listen :8082 --node-name relay-1 --role neutral --keyring ./data/keyring.json
```

**3. 调用示例**

```bash
TOKEN="admin.1788063858282577000.1788071058282577000.lhB5525JGjCjuF..."
AUTH="Authorization: Bearer $TOKEN"

# 拓扑
curl -H "$AUTH" http://127.0.0.1:8080/api/v1/topology
curl -H "$AUTH" http://127.0.0.1:8080/api/v1/peers

# 注册对等节点
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"edge-2","address":"10.0.0.2:7000","role":"edge"}' \
  http://127.0.0.1:8080/api/v1/peers

# 云端:注册服务 / 心跳 / 状态
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"edge-api","version":"1.0.0","endpoint":"https://api.example.com"}' \
  http://127.0.0.1:8080/api/v1/cloud/services
curl -X POST -H "$AUTH" http://127.0.0.1:8080/api/v1/cloud/heartbeat

# 边缘:区域 / 能力 / 指标
curl -X PUT -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"zone":"cn-east"}' http://127.0.0.1:8081/api/v1/edge/zone
curl -H "$AUTH" http://127.0.0.1:8081/api/v1/edge/metrics

# 令牌管理
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"subject":"edge-2","ttl":"1h"}' http://127.0.0.1:8080/api/v1/auth/issue
curl -H "$AUTH" http://127.0.0.1:8080/api/v1/auth/status
```

### 四、目录结构

```
FasterEdge2Api/
├── cmd/fasteredge2api/           # 入口: serve / token / version
├── internal/
│   ├── config/                   # 配置加载(flag + 环境变量)
│   ├── engine/                   # Atom 装配、角色注册、生命周期、认证调用
│   └── server/                   # HTTP 路由、认证中间件、各领域 handler、响应
├── go.mod / go.sum               # replace github.com/FasterEdge/FasterEdge => ../FasterEdge
├── README.md / README_en.md
└── LICENSE                       # Apache-2.0
```

### 五、API 一览

所有管理端点均需 `Authorization: Bearer <token>`(首个令牌由 CLI 引导签发)。响应统一包裹,字段名统一为 snake_case。

**公共端点(无需认证)**

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 存活探针 |
| GET | `/api/v1/logo` | 框架 logo |
| GET | `/api/v1/info` | 框架版本信息 |
| GET | `/api/v1/components` | 已注册 Data / Ability 清单 |

**拓扑(NetMap)**

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| GET | `/api/v1/topology` | NetMapAbility `get_topology` |
| GET | `/api/v1/node` | NetMapData `info` |
| PUT | `/api/v1/node/name` | NetMapData `set_node_name` |
| GET | `/api/v1/node/interfaces` | NetMapData `interfaces` |
| PUT | `/api/v1/node/default-interface` | NetMapData `set_default_iface` |
| GET | `/api/v1/peers` | NetMapAbility `list_peers` |
| POST | `/api/v1/peers` | NetMapAbility `register_peer` |
| GET | `/api/v1/peers/{name}` | NetMapAbility `lookup_peer` |
| PUT | `/api/v1/peers/{name}` | NetMapAbility `update_peer` |
| DELETE | `/api/v1/peers/{name}` | NetMapAbility `unregister_peer` |

**角色(Role)**

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| GET | `/api/v1/role` | RoleAbility `get_role` |
| PUT | `/api/v1/role` | RoleAbility `set_role` |

**云端节点(`--role cloud`)**

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| GET | `/api/v1/cloud` | CloudRoleAbility `describe` |
| GET/PUT | `/api/v1/cloud/controller` | CloudRoleAbility `get/set_controller` |
| GET/POST | `/api/v1/cloud/services` | CloudRoleAbility `list/register_service` |
| DELETE | `/api/v1/cloud/services/{name}` | CloudRoleAbility `unregister_service` |
| GET/PUT | `/api/v1/cloud/status` | CloudRoleAbility `get/set_status` |
| POST | `/api/v1/cloud/heartbeat` | CloudRoleAbility `heartbeat` |

**边缘节点(`--role edge`)**

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| GET | `/api/v1/edge` | EdgeRoleAbility `describe` |
| GET/PUT | `/api/v1/edge/zone` | EdgeRoleAbility `get/set_zone` |
| GET/POST | `/api/v1/edge/capabilities` | EdgeRoleAbility `list/add_capability` |
| PUT | `/api/v1/edge/capabilities` | EdgeRoleAbility `set_capabilities` |
| DELETE | `/api/v1/edge/capabilities/{name}` | EdgeRoleAbility `remove_capability` |
| POST | `/api/v1/edge/latency` | EdgeRoleAbility `record_latency` |
| GET | `/api/v1/edge/metrics` | EdgeRoleAbility `get_metrics` |
| PUT | `/api/v1/edge/online` | EdgeRoleAbility `set_online` |

**认证与令牌管理(OneKey)**

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| POST | `/api/v1/auth/issue` | OneKeyAbility `issue_token` |
| POST | `/api/v1/auth/verify` | OneKeyAbility `verify_token` |
| POST | `/api/v1/auth/revoke` | OneKeyAbility `revoke_token` |
| POST | `/api/v1/auth/revoke-all` | OneKeyAbility `revoke_all` |
| GET | `/api/v1/auth/tokens` | OneKeyAbility `list_tokens` |
| GET | `/api/v1/auth/status` | OneKeyAbility `status` |
| POST | `/api/v1/auth/rotate` | OneKeyAbility `rotate` |

### 六、配置

命令行 flag 与同名环境变量均可配置(flag 优先):

| flag | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `--listen` | `FE2A_LISTEN` | `:8080` | HTTP 监听地址 |
| `--node-name` | `FE2A_NODE_NAME` | `edge-1` | 本节点名 |
| `--role` | `FE2A_ROLE` | `neutral` | `cloud` / `edge` / `neutral` |
| `--keyring` | `FE2A_KEYRING_PATH` | *(空)* | KeyringData 持久化快照路径 |
| — | `FE2A_SHUTDOWN_TIMEOUT` | `5s` | 优雅退出等待时长 |

### 七、安全说明

- 所有管理端点(除公共端点)都要求 `Authorization: Bearer <token>`;令牌为 OneKey HMAC-SHA256 签名,过期 / 吊销 / 密钥轮换后均失效。
- **首个令牌只能通过 `fasteredge2api token` 签发**(进程内可信),不开放无凭据的 HTTP 引导端点,避免口令爆破面。
- `serve` 与 `token` 必须使用**同一个** `--keyring` 路径,否则密钥不一致导致认证失败;密钥轮换后旧令牌全部失效,需重新签发。
- Keyring 快照文件以 `0600` 权限落盘(FasterEdge KeyringData 保证),请妥善保管。
- 部署建议绑定内网 / 置于反向代理之后,并启用 TLS。

### 八、开发与测试

```bash
go build ./...
go vet ./...
go test -race ./...
```

测试覆盖:公共端点、认证拦截(无令牌 / 伪造令牌 401)、拓扑增删改查、云端 / 边缘节点全流程、令牌签发 / 校验 / 吊销 / 轮换、Keyring 持久化跨进程复用。

### 九、相关项目

- [FasterEdge](https://github.com/FasterEdge/FasterEdge):底层边缘计算框架(Data / Ability / Atom 组件模型)。
- [NetMap](https://github.com/FasterEdge/NetMap):FasterEdge 网络拓扑管理器(配合 `NetMapAbility` 上报拓扑)。

### License

[Apache-2.0](LICENSE)(与 FasterEdge 一致)。