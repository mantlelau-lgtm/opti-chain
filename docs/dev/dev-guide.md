# 开发指南

## 环境搭建

1. Go ≥ 1.26、Node ≥ 18
2. 后端：`go run ./cmd/server`（:8088，SQLite 自动建表）
3. 前端：`cd web && npm install && npm run dev`（:5173，代理 /api → :8088）
4. 验证：`go build ./... && go vet ./...`；`cd web && npm run build`

## 目录结构

```
cmd/server/main.go        装配入口（config→database→repository→service→handler→router）
internal/
  config/                 环境变量配置（SCM_ADDR / SCMDB_DRIVER / SCMDB_DSN）
  database/               双引擎连接 + AutoMigrate
  model/                  GORM 模型 + 状态常量（base_data/procurement/sales/inventory/planning）
  repository/             数据访问；genericRepo 泛型 CRUD；*InTx 参与外部事务
  service/                业务规则与事务边界；errors.go 定义业务错误→信封码映射
  handler/                参数绑定 + 信封响应，不含业务逻辑
  router/                 路由注册
  pkg/response, pkg/query 信封与分页工具
web/src/
  api/                    axios 封装（信封解包）+ 按模块 API
  components/CrudTable    通用主数据 CRUD 表格（支持 extraActions 行操作扩展）
  pages/, layouts/        模块页面与布局
```

## 代码规范

- 分层单向依赖；handler 零业务；事务边界在 service。
- 并发正确性：带守卫的原子 UPDATE + RowsAffected，禁止先 SELECT 后 UPDATE。
- 跨服务组合事务：被组合方暴露 `applyXxxInTx(tx, ...)` 或 repo `*InTx`。
- 头+明细写入：`Omit(clause.Associations)` + 显式循环。
- decimal 一律 `shopspring/decimal`；JSON 字符串传输；SQLite 守卫谓词**列在比较左侧**（类型亲和性，见 architecture.md）。
- 错误：service 返回 `errorsBadRequest/errNotFound/errf(ErrConflict,...)`，handler 用 `mapErr` 映射。
- 前端页面复用 CrudTable；定制页面参照 PurchaseOrderPage（明细行 Form.List）。

## 提交规范

Conventional 风格前缀 + 中文摘要：`init:`、`feat:`、`fix:`、`docs:`。一次提交一个主题。

## 测试

当前无单元测试；验证方式为**端到端脚本**（python urllib 调 API 断言信封码与数据状态），覆盖：收货闭环 19 项、销售闭环 18+2 项、表头编辑。新增闭环功能须补充同类 E2E 断言（负例必测：越权状态、超收、超卖、超限）。

## 依赖说明

### 后端（go.mod）

| 依赖 | 用途 |
| --- | --- |
| gin-gonic/gin | HTTP 路由/中间件 |
| gorm.io/gorm | ORM（AutoMigrate、事务、守卫更新） |
| glebarez/sqlite | 纯 Go SQLite 驱动（免 cgo） |
| gorm.io/driver/mysql | MySQL 驱动 |
| shopspring/decimal | 精确十进制（金额/数量） |

### 前端（web/package.json）

| 依赖 | 用途 |
| --- | --- |
| react / react-dom 18 | UI 运行时 |
| antd 5 | 组件库（Table/Modal/Form/Tag...） |
| @ant-design/icons | 图标 |
| react-router-dom | 前端路由 |
| axios | HTTP（信封解包） |
| dayjs | 日期 |
| vite 5 + @vitejs/plugin-react | 构建/开发服务器 |

升级依赖前确认 antd 大版本 API 兼容（Form.List、Table expandable 用法）。
