# clip2agent

[![CI](https://github.com/ruoruoji/clip2agent/actions/workflows/ci.yml/badge.svg)](https://github.com/ruoruoji/clip2agent/actions/workflows/ci.yml)
[![Release](https://github.com/ruoruoji/clip2agent/actions/workflows/release.yml/badge.svg)](https://github.com/ruoruoji/clip2agent/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

`clip2agent` 把系统剪贴板里的图片转换成终端 Agent 可直接消费的输入内容

它适合这样的场景：你刚截图了报错弹窗、设计稿、监控图或控制台页面，希望直接在终端里喂给 `Coco`、OpenAI 兼容链路或其他多模态 Agent，而不是手动保存图片、拷贝路径、再拼 prompt。

## 为什么做这个项目

- 剪贴板里已经有图片，但终端工具通常不能直接引用它
- macOS、Linux、Windows 的剪贴板能力不一致，脚本很难复用
- 不同 Agent 对图片输入方式不同，有的要路径，有的要 Base64 或 Data URI
- 快捷键触发时，手工保存文件和复制内容的链路太长

`clip2agent` 把这条链路统一成：**读取剪贴板图片 → 规范化 → 输出路径 / Base64 / Data URI / OCR 文本 → 可选写回剪贴板**。

说明：下文示例默认你已安装并可直接运行 `clip2agent`；如果你在仓库里源码态运行，请替换为 `go run ./cmd/clip2agent ...`。

## 快速开始

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | sh
```

macOS 想启用菜单栏 / 全局热键工作流：

```bash
clip2agent setup
```

## 特性

- 支持多种输出模式：`path`、`base64`、`data-uri`、`ocr`、`hybrid`
- 内置常用预设：`coco`、`openai-json`
- 支持诊断与排障：`inspect`、`doctor`
- 支持临时文件 TTL 清理与保留文件模式
- 支持全局快捷键工作流
  - macOS：菜单栏常驻 + `LaunchAgent`
  - Linux X11：`xbindkeys`
  - Linux Wayland：触发模式，交给桌面环境绑定快捷键
  - Windows：前台常驻监听
- 支持写回系统剪贴板，适合“截图后直接粘贴到 Agent”

## 支持平台

- `macOS`
- `Linux X11`
- `Linux Wayland`
- `Windows`

说明：

- `setup` 目前主要为 `macOS` 提供一键安装体验
- `Linux` 和 `Windows` 主要通过 CLI 主命令与各平台热键机制完成接入
- `Wayland` 受安全模型限制，不提供通用全局按键监听，推荐绑定 `clip2agent hotkey trigger`

## 安装

### 方式一：一键安装脚本（macOS / Linux）

`curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | sh`

常用参数：

- 安装指定版本：`curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | VERSION=v0.1.0 sh`
- 指定安装目录：`curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | BIN_DIR=$HOME/.local/bin sh`
- macOS 跳过 helper：`curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | INSTALL_MACOS_HELPERS=false sh`

脚本会：

- 自动识别 `OS/ARCH`
- 下载对应 release 资产
- 校验 `checksums.txt`
- 安装 `clip2agent`
- 在 macOS 上默认额外安装 `clip2agent-macos` 与 `clip2agent-hotkey`

### 卸载

推荐使用 CLI 一键卸载（跨平台）：

```bash
clip2agent uninstall


# 彻底清理（会删除 kept-dir，可能包含你保留的图片；需要显式确认）
clip2agent uninstall --purge --yes
```

如果你是在仓库里做本地开发，且需要“清理现有环境后重新 setup / 安装”，推荐直接走后面的“本地开发重置（清理后重装）”流程；源码态下更推荐直接用 `go run ./cmd/clip2agent ...` 执行这些命令。

### 方式二：从 GitHub Releases 下载

到 Releases 页面下载对应平台的压缩包，解压后将 `clip2agent` 放到你的 `PATH` 中。

如果你在 macOS 上使用全局热键，请下载与架构匹配的 helper 包：

- `clip2agent_<version>_darwin_amd64_helpers.tar.gz`
- `clip2agent_<version>_darwin_arm64_helpers.tar.gz`

### 方式三：源码态运行（开发用）

```bash
# 直接运行 CLI（推荐）
go run ./cmd/clip2agent <cmd>

# macOS 热键工作流一键安装（会安装 helper + hotkey + LaunchAgent + 初始化配置）
go run ./cmd/clip2agent setup
```

开发者如需单独构建 helper（`clip2agent-macos` / `clip2agent-hotkey`）：`swift build -c release --package-path native/macos`

## 常见用法

### 最常见用法

默认行为等价于 `clip2agent path`：

```bash
clip2agent
```

把图片落盘并输出适合 `Coco` 的内容（默认不附带固定提示词）：

```bash
clip2agent path --target coco
clip2agent coco
clip2agent coco --copy
```

输出 OpenAI 兼容 Base64 JSON：

```bash
clip2agent base64 --target openai --json
clip2agent openai-json
clip2agent openai-json --copy
```

输出 Data URI：

```bash
clip2agent data-uri
clip2agent data-uri --copy
```

如果目标链路不支持图片，可降级到 OCR：

```bash
clip2agent ocr
clip2agent hybrid
```

## 核心命令

- `clip2agent path`：写入临时文件并输出路径
- `clip2agent coco`：输出 `@路径`，适合快捷键链路
- `clip2agent base64 --target openai --json`：输出结构化 Base64 JSON
- `clip2agent data-uri`：输出 `data:<mime>;base64,<...>`
- `clip2agent ocr`：输出 OCR 结果
- `clip2agent hybrid`：输出图片引用 + OCR 摘要
- `clip2agent inspect`：检查剪贴板当前内容类型
- `clip2agent doctor`：检查依赖与平台能力
- `clip2agent gc`：清理临时文件
- `clip2agent config`：管理热键配置
- `clip2agent hotkey`：管理或触发全局热键能力
- `clip2agent uninstall`：卸载（默认安全；彻底清理需 `--purge --yes`）

查看帮助：

```bash
clip2agent --help
```

## 常用参数

- `--copy`：将输出文本写回系统剪贴板
- `--stdout`：仅输出载体内容，不加额外提示
- `--format keep` / `--format png` / `--format jpeg`：控制输出格式
- `--max-width` / `--max-height`：限制图片尺寸
- `--max-bytes 8m`：限制输出体积
- `--temp-dir <path>`：指定临时目录
- `--ttl 10m`：指定清理 TTL
- `--keep-file`：保留文件，不参与 GC
- `--no-strip-metadata`：尽量保留原图元数据

## 快捷键工作流

### macOS

推荐直接执行：

```bash
go run ./cmd/clip2agent setup
```

这会构建并安装：

- `clip2agent`
- `clip2agent-macos`
- `clip2agent-hotkey`

并初始化配置与 `LaunchAgent`。

额外推荐：

- 只读检查安装与下一步建议：`clip2agent setup --verify`（可加 `--json`）
- 排障汇总：`clip2agent doctor` / `clip2agent hotkey doctor`

菜单栏常驻：`C2A`（`clip2agent-hotkey`）。

- 支持自动监听 `hotkey.json` 变更并自动 reload（失败时保留上一次成功配置）
- 支持“部分成功”：单条 binding 配置错误不会导致全部热键失效，错误会在菜单中提示
- 提供 `Fix-it`：生成默认配置 / 强制重置（先备份）/ 打开配置目录 / 复制配置路径 / 运行 `setup --verify`

常用命令：

```bash
clip2agent hotkey status
clip2agent hotkey doctor
clip2agent hotkey restart
clip2agent hotkey logs
clip2agent hotkey uninstall
clip2agent uninstall
```

说明：

- `clip2agent hotkey uninstall` 仅卸载热键服务（LaunchAgent + plist），不会删除二进制/配置/临时文件
- 需要“彻底移除”时用 `clip2agent uninstall`（可选 `--purge --yes`）

日志与输出：

- macOS（菜单栏/LaunchAgent，`clip2agent-hotkey`）：日志写到 `~/Library/Logs/clip2agent.log`，可用 `clip2agent hotkey logs` 跟踪
- macOS CLI（`clip2agent ...` / `go run ./cmd/clip2agent ...` / `./clip2agent ...`）：每次执行也会追加到同一个 `~/Library/Logs/clip2agent.log`（行前缀为 `[clip2agent-cli]`）
- Linux hotkey：日志写到 `${XDG_STATE_HOME:-~/.local/state}/clip2agent/clip2agent.log`，可用 `clip2agent hotkey logs` 跟踪
- CLI 排障时仍建议把 `stdout/stderr` 一并留存
  - macOS/Linux：`clip2agent doctor --json > c2a-doctor.json 2> c2a-stderr.log`
  - Windows PowerShell：`clip2agent doctor --json *> c2a-doctor.log`
- Windows 热键：日志写到 `%LocalAppData%\clip2agent\clip2agent.log`，可用 `clip2agent hotkey logs` 跟踪；`clip2agent hotkey run` 仍会前台常驻

默认启用自动粘贴：需要授予系统“辅助功能”权限（配置项：`action.post.paste.enabled=true`，可选 `delay_ms`；如需关闭把 `enabled` 设为 `false`）。

提示：如果你在系统设置里“已经勾选了”，但菜单栏仍显示“未授权”，通常是授权到了旧路径/另一个版本。请在菜单栏 `C2A` 中查看“当前热键进程：<路径>”，或点击“复制当前热键进程路径（用于授权）”，然后在系统设置里按该路径对应的条目重新勾选。

### Linux

- `X11`：推荐用 `xbindkeys`
- `Wayland`：推荐将 `clip2agent hotkey trigger --id 1` 绑定到桌面环境快捷键

X11 示例：

```bash
sudo apt install xbindkeys xclip
clip2agent config init
clip2agent hotkey install
clip2agent hotkey status
```

### Windows

```powershell
clip2agent config init
clip2agent hotkey run
```

## 异常排查（日志 / 诊断快照 / 工件）

说明：如果你处于“源码态”（仓库里本地开发），且 `clip2agent` 二进制不在 `PATH`，可以把下文的 `clip2agent` 替换为 `go run ./cmd/clip2agent`。

### 1) 一键采集诊断快照（建议贴 Issue）

通用（所有平台）：

```bash
clip2agent inspect --json
clip2agent doctor --json
```

macOS（热键/菜单栏链路）：

```bash
clip2agent setup --verify --json
clip2agent hotkey doctor
```

建议把输出与报错一起留存：

- macOS/Linux：`clip2agent doctor --json > c2a-doctor.json 2> c2a-doctor.stderr.log`
- Windows PowerShell：`clip2agent doctor --json *> c2a-doctor.log`

### 2) 安装 / 卸载 / 构建日志怎么拿

安装脚本（macOS / Linux）：

```bash
curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | sh -x 2>&1 | tee c2a-install.log
```

macOS `setup`（构建/安装 helper + hotkey + LaunchAgent）：

```bash
clip2agent setup 2>&1 | tee c2a-setup.log

# 如需更详细的 Swift 构建输出
swift build -c release --package-path native/macos -v 2>&1 | tee c2a-swift-build.log
```

卸载（先预览再执行，便于定位“残留/删了什么”）：

```bash
clip2agent uninstall --dry-run --verbose 2>&1 | tee c2a-uninstall-plan.log

# 彻底清理（危险：会删除 kept-dir，可能包含你保留的图片）
clip2agent uninstall --purge --yes 2>&1 | tee c2a-uninstall.log
```

### 3) 配置与工件路径（需要提供哪些文件）

- hotkey 配置：`clip2agent config path`（通常为 `~/.config/clip2agent/hotkey.json`；或 `$XDG_CONFIG_HOME/clip2agent/hotkey.json`）
- macOS LaunchAgent plist：`~/Library/LaunchAgents/dev.clip2agent-hotkey.plist`
- 统一日志文件：
  - macOS：`~/Library/Logs/clip2agent.log`
  - Linux：`${XDG_STATE_HOME:-~/.local/state}/clip2agent/clip2agent.log`
  - Windows：`%LocalAppData%\clip2agent\clip2agent.log`
- Linux X11（xbindkeys）：`~/.config/clip2agent/xbindkeys.conf` 与 `~/.config/autostart/clip2agent-xbindkeys.desktop`（若设置 `XDG_CONFIG_HOME` 则以其为准）
- 临时目录（默认）：`$TMPDIR/clip2agent`（由系统决定，CLI 可用 `--temp-dir` 覆盖）
- kept-dir（默认）：用户 cache 目录下的 `clip2agent/kept`（例如 macOS 常见为 `~/Library/Caches/clip2agent/kept`）

### 4) 外部依赖与 provider 选择（为什么这台机子不工作）

- Linux Wayland：依赖 `wl-paste` / `wl-copy`（通常来自 `wl-clipboard`）；可用 `clip2agent inspect` 看是否探测到
- Linux X11：依赖 `xclip` 或 `xsel`；热键额外依赖 `xbindkeys`
- Windows：依赖 `powershell.exe` 或 `pwsh.exe`
- macOS：依赖 `clip2agent-macos` helper（`setup` 会安装）；可用 `clip2agent inspect` 查看是否可用
- 热键相关常见问题见：[常见问题（FAQ）](#常见问题faq)。

### 5) 临时文件 / kept-dir / GC 生命周期（文件去哪了）

- 默认每次运行会按 `--ttl` 做临时目录清理；可用 `--ttl` 调整清理窗口
- `--keep-file` 会把文件保留为“持久化”，并跳过自动 GC，适合需要长期引用图片的场景
- `clip2agent gc` 用于手动清理临时目录；若要清理 kept-dir 需要显式指定 `--temp-dir <kept-dir>` 且加 `--force`
- “找不到文件/文件被删”问题优先核对：是否启用了 `--keep-file`、`--ttl`，以及是否使用了自定义 `--temp-dir`

### 6) 权限与系统日志（可选）

- macOS 自动粘贴需要“辅助功能”权限（只影响热键工作流的“自动粘贴”，不影响 CLI 读剪贴板）
- macOS LaunchAgent 排查：`clip2agent hotkey status` / `clip2agent hotkey doctor`；必要时可用 `launchctl print gui/$(id -u)/dev.clip2agent-hotkey`
- macOS 崩溃日志（按需提供）：`~/Library/Logs/DiagnosticReports/`（或用 Console.app 搜索 `clip2agent`）
- Windows 崩溃/异常（按需提供）：事件查看器（Event Viewer）→ Windows 日志 → 应用程序

## 依赖说明

不同平台会依赖各自的系统工具：

- macOS：`clip2agent-macos` helper
- Linux Wayland：`wl-paste`、`wl-copy`
- Linux X11：`xclip` 或 `xsel`，热键场景可选 `xbindkeys`
- Windows：`powershell.exe` 或 `pwsh.exe`

如果遇到问题，优先执行：

```bash
clip2agent doctor --json
```

## 输出模式说明

### `path`

把剪贴板图片写入临时文件，适合路径引用型链路，默认最稳。

### `coco`

输出 `@路径`，更适合快捷键链路和写回剪贴板场景。

### `base64`

输出结构化 JSON，字段包含 `mime_type`、`encoding`、`data_base64`、尺寸、哈希等信息，适合 wrapper 或 API 层消费。

### `data-uri`

适合直接拼进多模态请求文本。

### `ocr` / `hybrid`

适合作为不支持图片输入时的降级方案。

## 开发

开发者安装 CLI（可选）：

```bash
go install github.com/ruoruoji/clip2agent/cmd/clip2agent@latest
```

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go build ./cmd/clip2agent
swift build -c release --package-path native/macos
```

### 本地开发重置（清理后重装）

当你遇到以下情况时，建议走一次完整重置流程：

- `setup` / `hotkey` 反复安装卸载后状态不一致
- `LaunchAgent`、helper、配置文件看起来有残留
- 你想从源码环境重新开始验证安装链路

推荐顺序：

```bash
# 1) 先预览将删除什么
go run ./cmd/clip2agent uninstall --dry-run --verbose

# 2) 确认后执行彻底清理
go run ./cmd/clip2agent uninstall --purge --yes

# 3) 清理后先看当前环境是否还有残留
go run ./cmd/clip2agent doctor

# 4) 重新 setup / 安装
go run ./cmd/clip2agent setup

# 5) 安装后做闭环验证
go run ./cmd/clip2agent setup --verify
go run ./cmd/clip2agent doctor
```

说明：

- `clip2agent uninstall --purge --yes` 是本地开发重置的标准清理入口；源码开发场景下，推荐通过 `go run ./cmd/clip2agent uninstall --purge --yes` 调用；它会删除二进制、`hotkey.json`、默认临时目录，以及 macOS 上的 `LaunchAgent` / 日志；`--purge` 还会删除 kept-dir
- `clip2agent doctor` 更适合“清理后”检查残留；`clip2agent setup --verify` 更适合“安装后”检查结果
- `clip2agent gc --force` 只作为补充清理：当你曾使用自定义 `temp-dir` / kept-dir，或需要额外回收临时文件时再执行
- 清理后如果安装态二进制已经被删掉，继续使用仓库内入口，例如 `go run ./cmd/clip2agent setup`、`go run ./cmd/clip2agent doctor`

更多协作约定见 `CONTRIBUTING.md`。

## 发布

仓库包含基于 tag 的 GitHub Release 工作流和安装脚本 `scripts/install.sh:1`。

当推送类似 `v0.1.0` 的 tag 时，会自动：

- 运行基础校验
- 构建多平台 `clip2agent` 二进制
- 构建 macOS `amd64` / `arm64` Swift helper
- 打包产物并生成校验文件
- 附带安装脚本 `install.sh`
- 创建 GitHub Release 并上传资产

示例：

```bash
git tag v0.1.0
git push origin v0.1.0
```

## 安全与隐私

- 默认会对临时文件做 TTL 清理
- 默认倾向于移除图片元数据
- OCR 与图片处理可能产生临时文件，建议在敏感环境中结合 `doctor`、`gc` 与权限设置一起使用

漏洞披露方式见 `SECURITY.md`。

## 常见问题（FAQ）

### 安装成功但提示 command not found

- 确认你的 `PATH` 里包含安装目录（默认 `~/.local/bin`）。
  - 可执行：`echo "$PATH" | tr ':' '\n' | grep -n "\.local/bin"`
  - 临时验证：`~/.local/bin/clip2agent --help`

### goenv / pyenv shim 抢占命令

- 可能会出现 shim 抢占命令（例如报 `goenv: 'clip2agent' command not found`）。
  - 处理方式：确保 `~/.local/bin` 在 `.goenv/shims` 之前，或删除失效 shim：`rm -f ~/.goenv/shims/clip2agent`

### macOS 热键提示 command not found（LaunchAgent PATH 不同）

- `LaunchAgent` 的 `PATH` 与你终端里的 `PATH` 可能不同。
  - 用 `clip2agent hotkey doctor` 查看服务状态与 LaunchAgent `PATH` 建议
  - 若要固定使用某个二进制路径，可在热键配置里显式指定，或通过 `CLIP2AGENT_BIN` 指向 `clip2agent`

## 路线图

- 更完善的 release 产物与安装体验
- 更细粒度的热键配置能力
- 更丰富的 OCR / hybrid 工作流
- 更多下游 Agent 适配器

## 贡献

欢迎提 Issue 和 PR。

- Bug 报告：`.github/ISSUE_TEMPLATE/bug_report.md:1`
- 功能建议：`.github/ISSUE_TEMPLATE/feature_request.md:1`
- 贡献说明：`CONTRIBUTING.md:1`

## 许可证

MIT，见 `LICENSE`。
