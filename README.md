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

```bash
curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/install.sh | sh
```

说明：该脚本优先从 GitHub Releases 拉取预编译产物；如果仓库还没有发布 Release，会自动 fallback 到 `go install`（fallback 模式仅安装 `clip2agent`，不会安装 macOS helpers）。

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

# 仅查看将执行的动作（不实际删除）
clip2agent uninstall --dry-run

# 彻底清理（会删除 kept-dir，可能包含你保留的图片；需要显式确认）
clip2agent uninstall --purge --yes
```

如果你是通过脚本安装的，也可以用脚本卸载（macOS / Linux）：

```bash
curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/uninstall.sh | sh

# dry-run
curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/uninstall.sh | DRY_RUN=true sh

# purge（危险：删除 kept-dir）
curl -fsSL https://raw.githubusercontent.com/ruoruoji/clip2agent/main/scripts/uninstall.sh | PURGE=true YES=true sh
```

### 方式二：从 GitHub Releases 下载

到 Releases 页面下载对应平台的压缩包，解压后将 `clip2agent` 放到你的 `PATH` 中。

如果你在 macOS 上使用全局热键，请下载与架构匹配的 helper 包：

- `clip2agent_<version>_darwin_amd64_helpers.tar.gz`
- `clip2agent_<version>_darwin_arm64_helpers.tar.gz`

### 方式三：使用 `go install`

```bash
go install github.com/ruoruoji/clip2agent/cmd/clip2agent@latest
```

### 方式四：从源码构建

```bash
go build -o clip2agent ./cmd/clip2agent
```

macOS 如需 helper：

```bash
swift build -c release --package-path native/macos
```

## 快速开始

### 先做环境检查

```bash
clip2agent doctor
clip2agent inspect

# macOS（热键/菜单栏工作流）可先做只读检查：
clip2agent setup --verify
# 需要结构化结果时：
clip2agent setup --verify --json
```

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

如果启用了自动粘贴，需要授予系统“辅助功能”权限（配置项：`action.post.paste.enabled=true`，可选 `delay_ms`）。

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

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go build ./cmd/clip2agent
swift build -c release --package-path native/macos
```

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
