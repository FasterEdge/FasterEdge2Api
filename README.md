# FasterEdge2Api

FasterEdge2Api 是把 [FasterEdge](https://github.com/FasterEdge/FasterEdge) 的**集群拓扑**与**集群管理**操作封装成 HTTP REST API 的 Go 网关服务。

它把一个 FasterEdge Atom(节点)装配为可远程管理的 HTTP 端点:对外提供节点拓扑管理(NetMap)、角色管理(Role)、云端/边缘节点管理(Cloud / Edge Role)以及 OneKey 令牌认证,所有管理操作统一走 FasterEdge 的 `AuthenticatedCommandContext`(HMAC-SHA256 令牌),不暴露本地可信的 `Command` 直调接口。

## 架构

```
                        ┌──────────────────────────────────────┐
 ┌──────────┐  HTTP    │            FasterEdge2Api             │
 │  curl /  │─────────▶│  ┌────────────┐   ┌────────────────┐  │
 │ 客户端    │  Bearer  │  │ HTTP 路由器 │──▶│ OneKey 认证中间件 │  │
 └──────────┘  Token   │  └────────────┘   └───────┬────────┘  │
                        │                          ▼            │
                        │       Atom.AuthenticatedCommandContext │
                        │  ┌──────────────────────────────────┐  │
                        │  │ NetMapData / NetMapAbility(拓扑)  │  │
                        │  │ RoleAbility(角色)                 │  │
                        │  │ CloudRoleAbility(云端)            │  │
                        │  │ EdgeRoleAbility(边缘)             │  │
                        │  │ KeyringData / OneKeyAbility(认证) │  │
                        │  └──────────────────────────────────┘  │
                        └──────────────────────────────────────┘
```

- **集群拓扑**:NetMapData(本节点网络信息)+ NetMapAbility(对等节点表)。
- **集群管理**:RoleAbility(角色)、CloudRoleAbility(云端:控制器/服务/状态/心跳)、EdgeRoleAbility(边缘:区域/能力/延迟/在线)。
- **认证**:KeyringData 共享密钥 + OneKeyAbility 签发短期 HMAC 令牌;远程调用必须携带 `Authorization: Bearer <token>`。

## 快速开始

### 1. 构建

要求 Go 1.25.5+。`FasterEdge` 通过 `replace` 指向本地路径 `../FasterEdge`:

```bash
cd FasterEdge2Api
go build ./...
# 生成可执行文件
go build -o fasteredge2api ./cmd/fasteredge2api
```

### 2. 签发首个访问令牌(引导)

**必须先签发令牌,再启动服务**。CLI 以进程内可信方式执行 `issue_token`,并把 KeyringData 快照(含共享密钥)落盘:

```bash
./fasteredge2api token --subject admin --ttl 24h --keyring ./data/keyring.json
# 输出: admin.<issuedNanos>.<expiresNanos>.<signature>
```

### 3. 启动服务

```bash
# 云端节点(role=cloud,注册 CloudRoleAbility)
FE2A_KEYRING_PATH=./data/keyring.json ./fasteredge2api serve \
  --listen :8080 --node-name cloud-1 --role cloud

# 边缘节点(role=edge,注册 EdgeRoleAbility)
./fasteredge2api serve --listen :8081 --node-name edge-1 --role edge

# 中立节点(仅拓扑 + 角色)
./fasteredge2api serve --listen :8082 --node-name relay-1 --role neutral
```

> **重要**:`serve` 与 `token` 必须使用**同一个** `--keyring` 路径(或同一 `FE2A_KEYRING_PATH`),否则密钥不一致导致认证失败。密钥轮换后旧令牌全部失效,需重新签发。

### 4. 调用示例

```bash
TOKEN="admin.1788063858282577000.1788071058282577000.lhB5525JGjCjuF..."
AUTH="Authorization: Bearer $TOKEN"

# 拓扑
curl -H "$AUTH" :8080/api/v1/topology
curl -H "$AUTH" :8080/api/v1/node
curl -H "$AUTH" :8080/api/v1/peers

# 注册对等节点
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"edge-2","address":"10.0.0.2:7000","role":"edge"}' \
  :8080/api/v1/peers

# 云端:注册服务 / 心跳 / 状态
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"edge-api","version":"1.0.0","endpoint":"https://api.example.com"}' \
  :8080/api/v1/cloud/services
curl -X POST -H "$AUTH" :8080/api/v1/cloud/heartbeat
curl -X PUT -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"status":"healthy"}' :8080/api/v1/cloud/status

# 边缘:区域 / 能力 / 延迟 / 在线
curl -X PUT -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"zone":"cn-east"}' :8081/api/v1/edge/zone
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"modbus"}' :8081/api/v1/edge/capabilities
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"latency_ms":12.5}' :8081/api/v1/edge/latency
curl -H "$AUTH" :8081/api/v1/edge/metrics

# 令牌管理
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"subject":"edge-2","ttl":"1h"}' :8080/api/v1/auth/issue
curl -H "$AUTH" :8080/api/v1/auth/status
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"subject":"edge-2"}' :8080/api/v1/auth/revoke
```

## API 一览

响应统一包裹:`{"ok":true,"data":...}` / `{"ok":false,"error":{"code":...,"message":...}}`,字段名统一为 snake_case。

### 公共端点(无需认证)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 存活探针 |
| GET | `/api/v1/logo` | 框架 logo |
| GET | `/api/v1/info` | 框架版本信息 |
| GET | `/api/v1/components` | 已注册 Data / Ability 清单 |

### 拓扑(需认证)

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

### 角色(需认证)

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| GET | `/api/v1/role` | RoleAbility `get_role` |
| PUT | `/api/v1/role` | RoleAbility `set_role` |

### 云端节点(需认证,`--role cloud`)

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| GET | `/api/v1/cloud` | CloudRoleAbility `describe` |
| GET/PUT | `/api/v1/cloud/controller` | CloudRoleAbility `get/set_controller` |
| GET/POST | `/api/v1/cloud/services` | CloudRoleAbility `list/register_service` |
| DELETE | `/api/v1/cloud/services/{name}` | CloudRoleAbility `unregister_service` |
| GET/PUT | `/api/v1/cloud/status` | CloudRoleAbility `get/set_status` |
| POST | `/api/v1/cloud/heartbeat` | CloudRoleAbility `heartbeat` |

### 边缘节点(需认证,`--role edge`)

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

### 认证与令牌管理(需认证)

| 方法 | 路径 | FasterEdge 命令 |
|------|------|-----------------|
| POST | `/api/v1/auth/issue` | OneKeyAbility `issue_token` |
| POST | `/api/v1/auth/verify` | OneKeyAbility `verify_token` |
| POST | `/api/v1/auth/revoke` | OneKeyAbility `revoke_token` |
| POST | `/api/v1/auth/revoke-all` | OneKeyAbility `revoke_all` |
| GET | `/api/v1/auth/tokens` | OneKeyAbility `list_tokens` |
| GET | `/api/v1/auth/status` | OneKeyAbility `status` |
| POST | `/api/v1/auth/rotate` | OneKeyAbility `rotate` |

## 配置

命令行 flag 与同名环境变量均可配置(flag 优先):

| flag | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `--listen` | `FE2A_LISTEN` | `:8080` | HTTP 监听地址 |
| `--node-name` | `FE2A_NODE_NAME` | `edge-1` | 本节点名 |
| `--role` | `FE2A_ROLE` | `neutral` | `cloud` / `edge` / `neutral` |
| `--keyring` | `FE2A_KEYRING_PATH` | *(空)* | KeyringData 持久化快照路径(留空则不持久化) |
| — | `FE2A_SHUTDOWN_TIMEOUT` | `5s` | 优雅退出等待时长 |

## 安全说明

- 所有管理端点(除公共端点外)都要求 `Authorization: Bearer <token>`;令牌为 FasterEdge OneKey HMAC-SHA256 签名,过期 / 吊销 / 密钥轮换后均失效。
- 首个令牌只能通过 `fasteredge2api token`(进程内可信)签发,不开放无凭据的 HTTP 引导端点,避免口令爆破面。
- Keyring 快照文件以 `0600` 权限落盘(由 FasterEdge KeyringData 保证),请妥善保管。
- 部署时建议将服务绑定内网 / 反向代理之后,并启用 TLS。

## 开发

```bash
go build ./...
go vet ./...
go test -race ./...
```

测试覆盖:公共端点、认证拦截、拓扑增删改查、云端/边缘节点全流程、令牌签发/吊销/轮换、Keyring 持久化跨进程复用。

## 相关项目

- [FasterEdge](https://github.com/FasterEdge/FasterEdge):底层边缘计算框架(Data / Ability / Atom 组件模型)。

## License

见仓库根目录 LICENSE(与 FasterEdge 一致)。