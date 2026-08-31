# 版本变更记录

## [0.3.0] - 2026-08-31

### Added
- 研发模块：产品主档（独立表 base_product）、版本化 BOM（base_bom / base_bom_detail）
- BOM 生命周期：DRAFT 编辑/删除 → 发布 RELEASED（自动作废旧默认版、成为生效版本），已发布不可改
- 权限：新增 rnd 模块 + bom:view/edit/release 权限点（幂等种子，老库自动补）
- 前端：BOM 管理页（产品选择、版本列表、组件明细编辑、发布/删除）

## [Unreleased]

### Added
- （规划）批次管理与保质期追溯（inv_batch + 库存批次维度）
- （规划）销售发货执行：出库推进 shipped_qty、释放锁定、自动完成
- （规划）寻源链：采购申请 → 询报价比价 → 定标转 PO
- （规划）独立质检单（inv_qc_order，含让步接收）、盘点单（inv_check）
- （规划）套餐 gating（P4 运营骨架）

## [0.2.0] - 2026-08-31

### Added
- 认证：JWT 登录（含租户码）、Auth 中间件、种子 admin/admin123
- RBAC：六固定角色（表存储）+ 32 个权限点 + 模块目录（sys_module/sys_permission/sys_role/sys_role_permission），RequirePerm 按路由鉴权（403）
- 多租户：sys_tenant + 租户管理（平台）/ 用户管理（租户内，平台可跨租户引导）；14 张业务表 tenant_id 行级隔离 + 复合唯一索引迁移
- 前端：租户登录页、菜单按权限渲染、用户管理页、租户管理页（启用/停用）、/auth/me

### Changed
- 登录需租户码；旧数据自动收养到 demo 租户

## [0.1.0] - 2026-08-30

### Added
- 基础数据：物料 / 供应商（准入审核）/ 客户（信用额度）/ 仓库 / 库位
- 采购：采购订单状态机、收货闭环（合格入库 / 拒收追溯 / 防超收 / 自动完成）、收货记录
- 销售：销售订单、审批锁库防超卖（原子守卫 SQL）、信用额度控制、取消释放
- 仓储：实时库存（含锁定量）、手工出入库（出库防超卖）、库存流水审计
- 计划：需求管理、MRP 运算（毛需求 + 安全库存 − 库存 − 在途）、建议转采购单
- 前端：React 18 + antd 5 全模块页面，含收货/审批/审核等操作弹窗
- 文档：docs/ 标准结构（user/dev/iterations/reports）

### Fixed
- 修复 GORM 级联双插导致建单 500（PO/库存单/收货单统一 `Omit(clause.Associations)`）
- 修复 SQLite 类型亲和性导致锁库守卫恒失败（谓词改写为列在比较左侧）
- 修复需求接口只收 RFC3339、前端发 `YYYY-MM-DD` 的兼容问题
- 修复采购订单编辑误走全量更新（新增表头专用更新路径）
- 修复前端 `Form.List` 缺闭合标签导致整站无法构建、`MaterialPage` 缺 `Input` 导入导致白屏
- 修复 vite 仅监听 IPv6 回环导致页面不可达（`host: true`）
