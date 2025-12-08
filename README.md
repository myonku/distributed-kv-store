# distributed-kv-store

一个学习/实验用的分布式 KV 存储，目前实现了最小可运行的 **单机模式**，后续会扩展为 Raft 主从和一致性哈希分片模式。

## 运行单机模式

当前 `cmd/kvnode` 中的入口以单机方式启动一个 HTTP KV 服务。

### 构建

```bash
cd cmd/kvnode
go build
```

### 启动

在仓库根目录下已经提供示例配置 `settings.toml`，监听 `127.0.0.1:8080`。

```bash
# 在仓库根目录
go run ./cmd/kvnode
# 或显式指定配置路径
# go run ./cmd/kvnode ./settings.toml
```

### 调用示例

```bash
# 写入
curl -X PUT "http://127.0.0.1:8080/kv?key=foo" -d "bar"

# 读取
curl "http://127.0.0.1:8080/kv?key=foo"

# 删除
curl -X DELETE "http://127.0.0.1:8080/kv?key=foo"
```

## 设计说明（简要）

- `configs`：统一的应用配置，支持 `standalone`/`raft`/`chash` 三种模式。
- `internal/store`：
  - `Storage`：抽象日志 + 状态机存储，目前是内存实现，后续可以接 WAL/快照。
  - `KVStore`：在单机模式下基于 `Storage` 提供简单 KV 语义，未来可被 Raft/CHash 复用。
- `internal/api`：
  - `KVService` 接口：对 HTTP / 内部 RPC 层暴露统一的 KV 操作。
  - `StartHTTPServer`：当前只实现了最简单的 `/kv` 接口，便于快速验证。
- `cmd/kvnode`：
  - 读取 `settings.toml`，初始化 `Storage` 和 `KVStore`，适配为 `KVService` 并启动 HTTP Server。

未来可以在不改动 HTTP 层的前提下，逐步替换 `KVService` 实现为 Raft 或一致性哈希模式。
