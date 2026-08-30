# 运维/操作手册

## 日常运维

- 启动：`go run ./cmd/server`（或编译后 `./server`）；前端 `npm run dev` / 生产托管 `web/dist`
- 切换 MySQL：设置 `SCMDB_DRIVER=mysql` + `SCMDB_DSN`（见 installation.md）

## 监控

- 健康检查：`GET /api/v1/materials` 返回 code 0
- Gin debug 日志含每次请求的方法/路径/状态/耗时

## 备份与恢复

- SQLite：定期复制 `scm.db`（停机或 WAL 一致性下）；恢复即覆盖回原路径
- MySQL：`mysqldump` 常规备份

## 故障排查

| 现象 | 处理 |
| --- | --- |
| `address already in use` | 旧进程占用 :8088，结束或改 `SCM_ADDR` |
| 前端 5173 打不开 | 确认 vite `host: true` 并重启；硬刷新浏览器 |
| 业务报错 | 见 faq.md；信封 message 字段即原因 |
