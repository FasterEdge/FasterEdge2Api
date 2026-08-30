<div align="center">
  <img src="https://avatars.githubusercontent.com/u/245985800?s=200&v=4" alt="logo" width="100" />
  <h2>FasterEdge2Api</h2>
  <h3>HTTP API for FasterEdge Cluster Topology &amp; Management</h3>
</div>

### 1. Overview

- **FasterEdge2Api** is an **HTTP API gateway** for the [FasterEdge](https://github.com/FasterEdge/FasterEdge) ecosystem: it assembles a FasterEdge Atom (a node) into a remotely manageable service and exposes **cluster topology management** (NetMap), **cluster management** (Role / Cloud / Edge) and **OneKey token authentication**.
- Every management call goes through FasterEdge's `AuthenticatedCommandContext` (HMAC-SHA256 tokens); the trusted in-process `Command` entry is never exposed remotely.
- Implemented with the Go standard library (`net/http`), single-binary deployment; role, keyring persistence and listen address are all configurable.

### 2. Features

| Feature | Description |
|---------|-------------|
| Cluster topology | Query and CRUD over local node info + peer table (`/api/v1/node`, `/api/v1/peers`, `/api/v1/topology`) |
| Cluster management | Role setting; full cloud (`--role cloud`) / edge (`--role edge`) node capabilities |
| Token auth | OneKey HMAC-SHA256 short-lived tokens; invalid immediately after expiry / revocation / rotation |
| Keyring persistence | Secret + token table stored in a `0600` snapshot; reused across processes and restarts |
| Uniform responses | `{"ok":true,"data":...}` / `{"ok":false,"error":{...}}`; snake_case field names |
| Hardening | Request body size limit, read/write/idle timeouts, panic recovery middleware |

### 3. Quick Start

> **Requirements**: Go 1.25.5+; `FasterEdge` is wired via `replace` to the local path `../FasterEdge`.

```bash
cd FasterEdge2Api
go build ./...
go build -o fasteredge2api ./cmd/fasteredge2api
```

**Step 1 — issue the first access token (bootstrap)**

The very first token must be issued in-process via the CLI, which also persists the shared secret:

```bash
./fasteredge2api token --subject admin --ttl 24h --keyring ./data/keyring.json
# prints: admin.<issuedNanos>.<expiresNanos>.<signature>
```

**Step 2 — start the server** (must share the **same** `--keyring` path as `token`)

```bash
# Cloud node
./fasteredge2api serve --listen :8080 --node-name cloud-1 --role cloud \
  --keyring ./data/keyring.json

# Edge node
./fasteredge2api serve --listen :8081 --node-name edge-1 --role edge --keyring ./data/keyring.json

# Neutral node (topology + role only)
./fasteredge2api serve --listen :8082 --node-name relay-1 --role neutral --keyring ./data/keyring.json
```

**Step 3 — call the API**

```bash
TOKEN="admin.1788063858282577000.1788071058282577000.lhB5525JGjCjuF..."
AUTH="Authorization: Bearer $TOKEN"

curl -H "$AUTH" http://127.0.0.1:8080/api/v1/topology
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"edge-2","address":"10.0.0.2:7000","role":"edge"}' \
  http://127.0.0.1:8080/api/v1/peers
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"edge-api","version":"1.0.0","endpoint":"https://api.example.com"}' \
  http://127.0.0.1:8080/api/v1/cloud/services
curl -H "$AUTH" http://127.0.0.1:8081/api/v1/edge/metrics
curl -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"subject":"edge-2","ttl":"1h"}' http://127.0.0.1:8080/api/v1/auth/issue
```

### 4. API Reference

Every management endpoint requires `Authorization: Bearer <token>`.

**Public endpoints (no auth)**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness probe |
| GET | `/api/v1/logo` | Framework logo |
| GET | `/api/v1/info` | Framework version info |
| GET | `/api/v1/components` | Registered Data / Ability list |

**Topology (NetMap)**

| Method | Path | FasterEdge command |
|--------|------|--------------------|
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

**Role**

| Method | Path | FasterEdge command |
|--------|------|--------------------|
| GET | `/api/v1/role` | RoleAbility `get_role` |
| PUT | `/api/v1/role` | RoleAbility `set_role` |

**Cloud node (`--role cloud`)**

| Method | Path | FasterEdge command |
|--------|------|--------------------|
| GET | `/api/v1/cloud` | CloudRoleAbility `describe` |
| GET/PUT | `/api/v1/cloud/controller` | CloudRoleAbility `get/set_controller` |
| GET/POST | `/api/v1/cloud/services` | CloudRoleAbility `list/register_service` |
| DELETE | `/api/v1/cloud/services/{name}` | CloudRoleAbility `unregister_service` |
| GET/PUT | `/api/v1/cloud/status` | CloudRoleAbility `get/set_status` |
| POST | `/api/v1/cloud/heartbeat` | CloudRoleAbility `heartbeat` |

**Edge node (`--role edge`)**

| Method | Path | FasterEdge command |
|--------|------|--------------------|
| GET | `/api/v1/edge` | EdgeRoleAbility `describe` |
| GET/PUT | `/api/v1/edge/zone` | EdgeRoleAbility `get/set_zone` |
| GET/POST | `/api/v1/edge/capabilities` | EdgeRoleAbility `list/add_capability` |
| PUT | `/api/v1/edge/capabilities` | EdgeRoleAbility `set_capabilities` |
| DELETE | `/api/v1/edge/capabilities/{name}` | EdgeRoleAbility `remove_capability` |
| POST | `/api/v1/edge/latency` | EdgeRoleAbility `record_latency` |
| GET | `/api/v1/edge/metrics` | EdgeRoleAbility `get_metrics` |
| PUT | `/api/v1/edge/online` | EdgeRoleAbility `set_online` |

**Auth & token management (OneKey)**

| Method | Path | FasterEdge command |
|--------|------|--------------------|
| POST | `/api/v1/auth/issue` | OneKeyAbility `issue_token` |
| POST | `/api/v1/auth/verify` | OneKeyAbility `verify_token` |
| POST | `/api/v1/auth/revoke` | OneKeyAbility `revoke_token` |
| POST | `/api/v1/auth/revoke-all` | OneKeyAbility `revoke_all` |
| GET | `/api/v1/auth/tokens` | OneKeyAbility `list_tokens` |
| GET | `/api/v1/auth/status` | OneKeyAbility `status` |
| POST | `/api/v1/auth/rotate` | OneKeyAbility `rotate` |

### 5. Configuration

Flags override the same-named environment variables:

| flag | env var | default | description |
|------|---------|---------|-------------|
| `--listen` | `FE2A_LISTEN` | `:8080` | HTTP listen address |
| `--node-name` | `FE2A_NODE_NAME` | `edge-1` | Local node name |
| `--role` | `FE2A_ROLE` | `neutral` | `cloud` / `edge` / `neutral` |
| `--keyring` | `FE2A_KEYRING_PATH` | *(empty)* | KeyringData snapshot path |
| — | `FE2A_SHUTDOWN_TIMEOUT` | `5s` | Graceful shutdown timeout |

### 6. Security Notes

- Every management endpoint (except the public ones) requires `Authorization: Bearer <token>`; tokens are OneKey HMAC-SHA256 signed and die immediately on expiry / revocation / rotation.
- The first token can only be issued via `fasteredge2api token` (trusted in-process); there is no unauthenticated HTTP bootstrap endpoint.
- `serve` and `token` **must** use the same `--keyring` path, otherwise authentication fails; after rotation all old tokens are invalid and must be re-issued.
- The keyring snapshot is written with `0600` permissions (enforced by FasterEdge KeyringData) — keep it safe.
- For production, bind to a private network / place behind a reverse proxy and enable TLS.

### 7. Development

```bash
go build ./...
go vet ./...
go test -race ./...
```

Tests cover: public endpoints, auth enforcement (401 without / with forged token), topology CRUD, full cloud / edge flows, token issue / verify / revoke / rotate, and keyring persistence across processes.

### 8. Related Projects

- [FasterEdge](https://github.com/FasterEdge/FasterEdge) — the underlying edge-computing framework (Data / Ability / Atom component model).
- [NetMap](https://github.com/FasterEdge/NetMap) — FasterEdge network topology manager (ingests topology via `NetMapAbility`).

### License

[Apache-2.0](LICENSE) (same as FasterEdge).