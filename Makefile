.PHONY: build build-mac build-windows clean test

# 默认构建当前平台
build:
	go build -o bin/clipboardsync ./cmd/clipboardsync/

# macOS 构建
build-mac:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/clipboardsync-darwin-amd64 ./cmd/clipboardsync/

# Windows 交叉编译（需要 mingw-w64 工具链）
build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o bin/clipboardsync-windows-amd64.exe ./cmd/clipboardsync/

# 运行测试
test:
	go test ./... -v

# 清理构建产物
clean:
	rm -rf bin/
