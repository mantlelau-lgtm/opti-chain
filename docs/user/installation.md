# 安装部署指南

## 环境要求

- 操作系统：macOS / Linux（Windows 理论可行，未验证）
- 后端运行时：Go ≥ 1.26（go.mod 声明 1.26.5）
- 前端运行时：Node.js ≥ 18（Vite 5 要求）
- 数据库：
  - **SQLite（默认）**：内置驱动（glebarez/sqlite），零依赖，首次启动自动建表
  - **MySQL 8（可选）**：通过环境变量切换，如 Docker Compose 部署的 MySQL 8.4

## 安装步骤

1. 克隆仓库：

   ```bash
   git clone git@github.com:mantlelau-lgtm/opti-chain.git
   cd opti-chain
   ```

2. 启动后端（默认监听 :8088，SQLite 库文件 `scm.db` 自动创建并迁移表结构）：

   ```bash
   go run ./cmd/server
   ```

3. 安装并启动前端（默认 :5173，开发模式自动代理 `/api` → :8088）：

   ```bash
   cd web
   npm install
   npm run dev
   ```

4. 浏览器打开 `http://localhost:5173`。

## 配置说明

全部通过环境变量配置，均有默认值，裸启动即可用：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SCM_ADDR` | `:8088` | 后端监听地址 |
| `SCMDB_DRIVER` | `sqlite` | 数据库驱动：`sqlite` 或 `mysql` |
| `SCMDB_DSN` | `scm.db` | SQLite 文件路径，或 MySQL DSN（`user:pass@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True`） |

切换 MySQL 示例：

```bash
SCMDB_DRIVER=mysql SCMDB_DSN='root:root123@tcp(127.0.0.1:3306)/scm?charset=utf8mb4&parseTime=True' go run ./cmd/server
```

## 验证安装

```bash
curl http://127.0.0.1:8088/api/v1/materials   # 返回 {"code":0,...}
curl -o /dev/null -w '%{http_code}' http://127.0.0.1:5173/   # 200
```

## 生产构建（前端）

```bash
cd web && npm run build   # 产物在 web/dist，交由任意静态服务器托管
```
