.PHONY: build test vet fmt lint clean restart

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

# 一键重启
restart:
	./restart.sh

# 清理编译产物
clean:
	rm -f bin/server bin/scm-mcp