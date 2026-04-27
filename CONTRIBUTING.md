# Contributing

感谢你对 `clip2agent` 的关注。

## 开始之前

- 提交 Issue 或 PR 前，请先确认该问题或需求没有重复。
- 涉及行为变更、命令行参数调整或跨平台差异时，请在描述里写清楚目标平台与预期行为。
- 如果改动会影响 `README.md` 中的使用方式，请同步更新文档。

## 本地开发

### Go CLI

```bash
go test ./...
go vet ./...
go build ./cmd/clip2agent
```

### Go 格式化

```bash
gofmt -w ./cmd ./internal
```

### macOS Swift helper

```bash
cd native/macos
swift build -c release
```

### 本地开发重置（清理后重装）

当你需要清理现有环境、重新验证安装链路时，按下面顺序执行：

```bash
go run ./cmd/clip2agent uninstall --dry-run --verbose
go run ./cmd/clip2agent uninstall --purge --yes
go run ./cmd/clip2agent doctor
go run ./cmd/clip2agent setup
go run ./cmd/clip2agent setup --verify
```

- `doctor` 更适合清理后的残留检查；`setup --verify` 更适合安装后验证
- 涉及安装 / 卸载 / 开发流程变更时，请同步更新 `README.md`、帮助文案与诊断输出

## 提交规范

- 保持 PR 尽量小而清晰，避免同时混入无关重构。
- 提交前请确保本地测试通过，且 Go 代码已完成格式化。
- 新增功能时，优先补充或更新对应测试。
- 涉及平台兼容问题时，请说明验证环境，例如 `macOS`、`Linux X11`、`Linux Wayland`、`Windows`。

## 文档与可维护性

- 新增 CLI 选项、子命令或行为变更时，请更新 `README.md`。
- 错误信息、帮助文案与 README 中的命令示例应保持一致。
- 不要提交本地构建产物、日志文件或 IDE 配置。
