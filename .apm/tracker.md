---
title: ClipboardSync
---

# APM Tracker

## Task Tracking

**Stage 1:**

| Task | Status | Agent | Branch |
|------|--------|-------|--------|
| 1.1 | Ready | network-agent | |
| 1.2 | Ready | clipboard-agent | |
| 1.3 | Waiting: 1.1 | network-agent | |

## Worker Tracking

| Agent | Instance | Notes |
|-------|----------|-------|
| network-agent | 0 | 未初始化 |
| clipboard-agent | 0 | 未初始化 |
| gui-agent | 0 | 未初始化 |

## Version Control

| Repository | Base Branch | Branch Convention | Commit Convention |
|-----------|-------------|-------------------|-------------------|
| clipboardsync | master | type/short-description | type: description (feat, fix, refactor, docs, test, chore) |

## Working Notes

- 用户无后端/系统编程经验，任务指引需包含明确的环境搭建步骤
- 产品需自解释，GUI 文本、提示和错误信息应清晰易懂
- 用户同时拥有 Mac 和 Windows 机器可用于测试
- 跨编译挑战（CGO for SQLite、平台特定剪贴板 API）应在 Stage 1 尽早验证
