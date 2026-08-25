# Codex Sessions

一个快速、美观的 Codex CLI 本地 session 浏览器。

它把账号登录、custom API provider、VS Code 和不同项目目录创建的 sessions 汇总到同一个可搜索界面中。选择 session 后，仍然交给 Codex 官方的 `resume` 或 `fork` 命令继续。

> 非官方社区工具，与 OpenAI 无隶属或背书关系。

[English](README.md)

## 功能

- 统一显示 `openai`、`custom` 和多个 `CODEX_HOME` 中的 sessions
- 先按工作目录浏览，再查看其中的对话
- 搜索项目路径、rename 名称、第一句话、provider、模型和 UUID
- 使用原 provider、账号 provider 或 custom provider 恢复
- 支持 fork，不修改原对话
- 显式透传 `--yolo`，界面会显示醒目警告
- 只读访问 SQLite，失败时回退扫描 JSONL rollout
- 自动隐藏没有任何用户消息的空 session
- 纯 Go 单文件二进制，不依赖 Python、CGO、`sqlite3` 或 `fzf`
- 支持 macOS、Linux、Windows 的 x86-64 和 ARM64

## 安装

使用 Go 1.24.2 或更新版本：

```bash
go install github.com/wminjay/codex-sessions/cmd/codex-sessions@latest
```

也可以从 [GitHub Releases](https://github.com/wminjay/codex-sessions/releases) 下载对应平台的压缩包。

可选短别名：

```bash
alias cxs='codex-sessions'
```

## 使用

```bash
codex-sessions
```

启动后先显示目录列表，按 `enter` 进入目录，再选择 session。

### 快捷键

| 按键 | 目录页 | Session 页 |
|---|---|---|
| `↑` / `↓`、`j` / `k` | 选择目录 | 选择 session |
| `enter` | 进入目录 | 使用原 provider 恢复 |
| `/` | 搜索目录及其中的 sessions | 搜索当前目录的 sessions |
| `c` | — | 使用 `custom` provider 恢复 |
| `a` | — | 使用账号 `openai` provider 恢复 |
| `f` | — | 使用原 provider fork |
| `C` / `A` | — | 使用 custom / 账号 provider fork |
| `esc`、`b`、`←` | 退出 | 返回目录页 |
| `q` | 退出 | 退出 |

### 常用命令

```bash
# 启用 YOLO 模式
codex-sessions -yolo

# 只查看，不启动
codex-sessions --list --limit 20
codex-sessions --json --query codex-sessions

# 用 custom provider 恢复已知 session
codex-sessions --session 01a02e1c --provider custom

# 只打印最终命令
codex-sessions --session 01a02e1c --provider custom -yolo --dry-run

# 合并多个独立存储
codex-sessions \
  --codex-home ~/.codex \
  --codex-home ~/.codex-work
```

如果 custom provider 位于 `$CODEX_HOME/custom.config.toml`，使用：

```bash
codex-sessions --custom-profile custom
```

工具不会读取或保存 API Key，也不会修改 Codex 的 SQLite 或 rollout 文件。

## 开发

```bash
go test ./...
go vet ./...
go build ./cmd/codex-sessions
```

底层设计见 [docs/architecture.md](docs/architecture.md)。

## 许可证

[MIT](LICENSE)
