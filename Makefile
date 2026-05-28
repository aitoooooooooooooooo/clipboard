.PHONY: help dev build mac windows all clean test

help: ## 显示帮助
	@echo ""
	@echo "  ClipboardSync 构建命令"
	@echo "  ─────────────────────────────────"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36mmake %-10s\033[0m %s\n", $$1, $$2}'
	@echo ""

dev: ## 开发模式（热重载窗口）
	wails dev

build: mac ## 打包当前平台（等同于 make mac）

mac: ## 打包 macOS 版本
	wails build -platform darwin/universal -clean

windows: ## 打包 Windows 版本
	wails build -platform windows/amd64 -clean

all: mac windows ## 同时打包 Mac 和 Windows 版本

test: ## 运行测试
	go test ./... -v

clean: ## 清理构建产物
	rm -rf bin/ build/
