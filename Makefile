.PHONY: build test vet fmt lint clean restart migrate-memory

# 编译后端和前端
build:
	go build -o bin/server ./cmd/server
	go build -o bin/scm-mcp ./cmd/scm-mcp
	cd web && npm run build

# 仅编译后端
build-backend:
	go build -o bin/server ./cmd/server
	go build -o bin/scm-mcp ./cmd/scm-mcp

# 单元测试
test:
	go test -race -cover ./...

# 代码静态检查
vet:
	go vet ./...

# 格式化
fmt:
	gofmt -w .

# 整理依赖
tidy:
	go mod tidy

# 一次性迁移：把 sys_assistant_memory 表里的历史对话导出为 JSONL 并清空该表
# 需先停服；加 ARGS="-dry-run" 可只导出不清表
# 与 restart.sh 一致地加载 .env，否则会连到默认的 sqlite 而不是真实数据库
migrate-memory:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	go run ./cmd/migrate-memory $(ARGS)

# 一键重启
restart:
	./restart.sh

# 清理编译产物
clean:
	rm -f bin/server bin/scm-mcp