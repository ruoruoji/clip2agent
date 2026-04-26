package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ruoruoji/clip2agent/internal/adapter"
	"github.com/ruoruoji/clip2agent/internal/clipboard"
	"github.com/ruoruoji/clip2agent/internal/diagnostics"
	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/imagex"
	"github.com/ruoruoji/clip2agent/internal/paths"
	"github.com/ruoruoji/clip2agent/internal/render"
	"github.com/ruoruoji/clip2agent/internal/temp"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 低成本上手：不带子命令时，默认等价于 `clip2agent path`。
	// 同时允许用户直接传入 path flags，如：`clip2agent --copy`。
	if len(os.Args) < 2 {
		return runPath(ctx, nil)
	}

	sub := os.Args[1]
	if strings.HasPrefix(sub, "-") && sub != "-h" && sub != "--help" {
		return runPath(ctx, os.Args[1:])
	}
	switch sub {
	case "help", "-h", "--help":
		fmt.Println(usage())
		return 0
	case "init":
		return runInit(ctx, os.Args[2:])
	case "coco":
		return runCoco(ctx, os.Args[2:])
	case "path":
		return runPath(ctx, os.Args[2:])
	case "base64":
		return runBase64(ctx, os.Args[2:])
	case "openai-json":
		return runOpenAIJSON(ctx, os.Args[2:])
	case "data-uri":
		return runDataURI(ctx, os.Args[2:])
	case "ocr":
		return runOCR(ctx, os.Args[2:])
	case "hybrid":
		return runHybrid(ctx, os.Args[2:])
	case "inspect":
		return runInspect(ctx, os.Args[2:])
	case "doctor":
		return runDoctor(ctx, os.Args[2:])
	case "gc":
		return runGC(ctx, os.Args[2:])
	case "config":
		return runConfig(ctx, os.Args[2:])
	case "setup":
		return runSetup(ctx, os.Args[2:])
	case "hotkey":
		return runHotkey(ctx, os.Args[2:])
	case "uninstall":
		return runUninstall(ctx, os.Args[2:])
	default:
		errs.Fprint(os.Stderr, errs.E(errs.Code("E006"), "不支持的子命令", errs.Hint("支持: coco/path/base64/openai-json/data-uri/inspect/doctor/gc/config/setup/hotkey/uninstall；或直接运行 `clip2agent --help`")))
		fmt.Fprintln(os.Stderr)
		return 2
	}
}

func usage() string {
	return `clip2agent - 将剪贴板图片转换为 Agent 可直接引用的文本载体

用法:
	clip2agent               # 默认等价于: clip2agent path
	clip2agent [path flags]  # 例如: clip2agent --copy
	clip2agent init <zsh|bash|fish>
	clip2agent coco   [flags]
	clip2agent path   [flags]
	clip2agent base64 [flags]
	clip2agent openai-json [flags]
	clip2agent data-uri [flags]
	clip2agent ocr [flags]
	clip2agent hybrid [flags]
	clip2agent inspect
	clip2agent doctor
	clip2agent gc     [flags]
	clip2agent config <subcommand>
	clip2agent setup  [flags]
	clip2agent hotkey <subcommand>
	clip2agent uninstall [flags]

子命令:
	path      落临时文件并输出路径（默认 target=coco）
	base64    输出 base64 结构化 JSON（默认 target=openai）
	coco      Coco 预设：输出 @路径（可配合 --copy 用于快捷键）
	openai-json OpenAI 预设：等价于 base64 --target openai --json
	data-uri  输出 Data URI（data:<mime>;base64,<...>）
	ocr       OCR（可选依赖 tesseract），默认输出 Markdown
	hybrid    输出 @路径 + OCR 摘要（可选依赖 tesseract）
	inspect   输出当前平台与剪贴板能力探测信息
	doctor    检查依赖与给出修复建议
	gc        清理过期临时文件（默认 temp-dir + ttl）
	config    管理快捷键配置（hotkey.json）
	setup     macOS 一键构建/安装 helper 与 hotkey 并初始化配置
	hotkey    macOS 全局快捷键服务（LaunchAgent）管理
	uninstall 卸载（默认安全；彻底清理需 --purge --yes）
	init      输出 shell alias（只输出到 stdout，不写任何 rc 文件）

快速排障:
	clip2agent doctor
	clip2agent setup     # macOS 一键构建/安装/初始化（含 LaunchAgent）
`
}

type ocrJSON struct {
	SchemaVersion int               `json:"schema_version"`
	Mode          string            `json:"mode"`
	Engine        string            `json:"engine"`
	Lang          string            `json:"lang"`
	Text          string            `json:"text"`
	Image         render.Base64JSON `json:"image"`
}

func runOCR(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("ocr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f commonFlags
	f.bind(fs)
	var lang string
	fs.StringVar(&lang, "lang", "eng", "tesseract 语言（如 eng/chi_sim/eng+chi_sim）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := f.gcIfNeeded(ctx); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "临时目录不可写或清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	normOpt, err := f.normalizeOptions()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 2
	}
	prov := clipboard.NewSelector().Select()
	raw, err := prov.GetImage(ctx)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	img, err := imagex.Normalize(raw, normOpt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	path, err := f.tempManager().Write(ctx, img)
	if err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "写入临时文件失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}
	text, err := runTesseractOCR(ctx, path, lang)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	out := strings.TrimSpace(text)
	if f.json {
		js := ocrJSON{SchemaVersion: 1, Mode: "ocr", Engine: "tesseract", Lang: lang, Text: out, Image: render.NewBase64Payload(img).JSON}
		b, jerr := json.MarshalIndent(js, "", "  ")
		if jerr != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "生成 JSON 失败", jerr))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		out = string(b)
	}
	if f.copy {
		if err := clipboard.WriteText(ctx, out); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}
	fmt.Print(out)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return 0
}

type hybridJSON struct {
	SchemaVersion int     `json:"schema_version"`
	Mode          string  `json:"mode"`
	AtPath        string  `json:"at_path"`
	OCR           ocrJSON `json:"ocr"`
}

func runHybrid(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("hybrid", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f commonFlags
	f.bind(fs)
	var target string
	fs.StringVar(&target, "target", "coco", "目标适配器：coco|generic")
	var lang string
	fs.StringVar(&lang, "lang", "eng", "tesseract 语言（如 eng/chi_sim/eng+chi_sim）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := f.gcIfNeeded(ctx); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "临时目录不可写或清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	normOpt, err := f.normalizeOptions()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 2
	}
	prov := clipboard.NewSelector().Select()
	raw, err := prov.GetImage(ctx)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	img, err := imagex.Normalize(raw, normOpt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	path, err := f.tempManager().Write(ctx, img)
	if err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "写入临时文件失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}
	ocrText, err := runTesseractOCR(ctx, path, lang)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	at := "@" + path
	ocrOut := strings.TrimSpace(ocrText)
	if f.json {
		ocr := ocrJSON{SchemaVersion: 1, Mode: "ocr", Engine: "tesseract", Lang: lang, Text: ocrOut, Image: render.NewBase64Payload(img).JSON}
		js := hybridJSON{SchemaVersion: 1, Mode: "hybrid", AtPath: at, OCR: ocr}
		b, jerr := json.MarshalIndent(js, "", "  ")
		if jerr != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "生成 JSON 失败", jerr))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		out := string(b)
		if f.copy {
			if err := clipboard.WriteText(ctx, out); err != nil {
				errs.Fprint(os.Stderr, err)
				fmt.Fprintln(os.Stderr)
				return 1
			}
		}
		fmt.Println(out)
		return 0
	}

	// 文本输出：尽量贴近 coco-hotkey 的使用习惯。
	out := at + "\n\nOCR:\n" + ocrOut
	if strings.ToLower(strings.TrimSpace(target)) == "coco" && !f.stdout {
		// 在 coco target 下，补一个简短提示模板，但不要改动 path 的旧行为。
		out = "请结合图片与 OCR 文本分析内容并给出结论：\n" + at + "\n\nOCR:\n" + ocrOut
	}
	if f.copy {
		if err := clipboard.WriteText(ctx, out); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}
	fmt.Print(out)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return 0
}

func runInit(_ context.Context, args []string) int {
	if len(args) < 1 {
		errs.Fprint(os.Stderr, errs.E("E006", "用法: clip2agent init <zsh|bash|fish>"))
		fmt.Fprintln(os.Stderr)
		return 2
	}
	shell := strings.ToLower(strings.TrimSpace(args[0]))
	switch shell {
	case "zsh", "bash":
		fmt.Println("# 在你的 shell rc 里粘贴以下内容（本命令不会自动写入任何文件）")
		fmt.Println("alias c2a='clip2agent'")
		fmt.Println("alias c2acoco='clip2agent coco --copy'")
		fmt.Println("alias c2aopenai='clip2agent openai-json --copy'")
		fmt.Println("alias c2auri='clip2agent data-uri --copy'")
		fmt.Println("# 示例：复制图片后运行 c2acoco / c2aopenai / c2auri")
		return 0
	case "fish":
		fmt.Println("# 在 fish 配置里粘贴以下内容（本命令不会自动写入任何文件）")
		fmt.Println("alias c2a 'clip2agent'")
		fmt.Println("alias c2acoco 'clip2agent coco --copy'")
		fmt.Println("alias c2aopenai 'clip2agent openai-json --copy'")
		fmt.Println("alias c2auri 'clip2agent data-uri --copy'")
		fmt.Println("# 示例：复制图片后运行 c2acoco / c2aopenai / c2auri")
		return 0
	default:
		errs.Fprint(os.Stderr, errs.E("E006", "不支持的 shell", errs.Hint("支持: zsh|bash|fish")))
		fmt.Fprintln(os.Stderr)
		return 2
	}
}

type commonFlags struct {
	format    string
	maxWidth  int
	maxHeight int
	maxBytes  string

	tempDir  string
	keptDir  string
	ttl      time.Duration
	keepFile bool

	stdout          bool
	json            bool
	copy            bool
	debug           bool
	noStripMetadata bool
}

func (c *commonFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.format, "format", "keep", "png|jpeg|webp|keep")
	fs.IntVar(&c.maxWidth, "max-width", 2048, "最大宽度（像素）")
	fs.IntVar(&c.maxHeight, "max-height", 2048, "最大高度（像素）")
	fs.StringVar(&c.maxBytes, "max-bytes", "8m", "最大体积（如 8m/512k/0 表示不限制）")

	fs.StringVar(&c.tempDir, "temp-dir", "", "临时目录（默认使用系统临时目录下的 clip2agent/）")
	fs.StringVar(&c.keptDir, "kept-dir", "", "持久目录（仅在 --keep-file 且未指定 --temp-dir 时生效）")
	fs.DurationVar(&c.ttl, "ttl", 10*time.Minute, "临时文件 TTL（用于下次运行清理）")
	fs.BoolVar(&c.keepFile, "keep-file", false, "保留文件（不进行 GC；且默认写入 kept-dir）")

	fs.BoolVar(&c.stdout, "stdout", false, "仅输出载体（不加额外提示）")
	fs.BoolVar(&c.json, "json", false, "输出 JSON")
	fs.BoolVar(&c.copy, "copy", false, "将输出写回系统剪贴板（用于快捷键工作流）")
	fs.BoolVar(&c.debug, "debug", false, "输出调试信息（不打印完整 base64）")
	fs.BoolVar(&c.noStripMetadata, "no-strip-metadata", false, "尽可能保留原始字节（可能包含元数据；仅在无需 resize/压缩/转码时生效）")
}

func (c commonFlags) normalizeOptions() (imagex.NormalizeOptions, error) {
	maxBytes, err := imagex.ParseByteSize(c.maxBytes)
	if err != nil {
		return imagex.NormalizeOptions{}, errs.E(errs.Code("E006"), "max-bytes 参数非法", errs.Cause(err))
	}
	return imagex.NormalizeOptions{
		Format:          c.format,
		MaxWidth:        c.maxWidth,
		MaxHeight:       c.maxHeight,
		MaxBytes:        maxBytes,
		NoStripMetadata: c.noStripMetadata,
	}, nil
}

func (c commonFlags) tempManager() *temp.Manager {
	base := strings.TrimSpace(c.tempDir)
	if base == "" && c.keepFile {
		base = strings.TrimSpace(c.keptDir)
		if base == "" {
			base = paths.DefaultKeptDir()
		}
	}
	return temp.NewManager(temp.Options{BaseDir: base})
}

func (c commonFlags) gcIfNeeded(ctx context.Context) error {
	if c.keepFile {
		return nil
	}
	return c.tempManager().GC(ctx, c.ttl)
}

func runPath(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("path", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f commonFlags
	f.bind(fs)
	var target string
	fs.StringVar(&target, "target", "coco", "目标适配器：coco|generic")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := f.gcIfNeeded(ctx); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "临时目录不可写或清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	normOpt, err := f.normalizeOptions()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 2
	}

	prov := clipboard.NewSelector().Select()
	raw, err := prov.GetImage(ctx)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	img, err := imagex.Normalize(raw, normOpt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	path, err := f.tempManager().Write(ctx, img)
	if err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "写入临时文件失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	payload := render.NewPathPayload(img, path)
	ad := adapter.ByTarget(target)
	if ad == nil {
		errs.Fprint(os.Stderr, errs.E(errs.Code("E006"), "目标适配器不支持", errs.Hint("target=coco|generic")))
		fmt.Fprintln(os.Stderr)
		return 2
	}
	out, err := ad.AdaptPath(payload, adapter.Options{OnlyPayload: f.stdout})
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	if f.copy {
		if err := clipboard.WriteText(ctx, out); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}

	fmt.Print(out)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return 0
}

func runBase64(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("base64", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f commonFlags
	f.bind(fs)
	var target string
	fs.StringVar(&target, "target", "openai", "目标适配器：openai|generic")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := f.gcIfNeeded(ctx); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "临时目录不可写或清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	normOpt, err := f.normalizeOptions()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 2
	}

	prov := clipboard.NewSelector().Select()
	raw, err := prov.GetImage(ctx)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	img, err := imagex.Normalize(raw, normOpt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	ad := adapter.ByTarget(target)
	if ad == nil {
		errs.Fprint(os.Stderr, errs.E(errs.Code("E006"), "目标适配器不支持", errs.Hint("target=openai|generic")))
		fmt.Fprintln(os.Stderr)
		return 2
	}

	payload := render.NewBase64Payload(img)
	out, err := ad.AdaptBase64(payload, adapter.Options{JSON: true})
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	if f.copy {
		if err := clipboard.WriteText(ctx, out); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}

	fmt.Print(out)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		fmt.Println()
	}
	_ = f.json // base64 默认输出 JSON，保留参数兼容 plan.md
	return 0
}

func runCoco(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("coco", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f commonFlags
	f.bind(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// 产品化预设：等价于 path --target coco，但默认输出 @path（而不是纯 path）。
	return runPathWithOptions(ctx, f, "coco", adapter.Options{OnlyPayload: f.stdout, CocoAtPath: true})
}

func runOpenAIJSON(ctx context.Context, args []string) int {
	// 产品化预设：等价于 base64 --target openai --json
	// 兼容：base64 本就默认输出 JSON。
	clean := dropTargetFlag(args)
	clean = append([]string{"--target", "openai", "--json"}, clean...)
	return runBase64(ctx, clean)
}

func dropTargetFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--target" {
			i++
			continue
		}
		if strings.HasPrefix(a, "--target=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func runPathWithOptions(ctx context.Context, f commonFlags, target string, opt adapter.Options) int {
	if err := f.gcIfNeeded(ctx); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "临时目录不可写或清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	normOpt, err := f.normalizeOptions()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 2
	}

	prov := clipboard.NewSelector().Select()
	raw, err := prov.GetImage(ctx)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	img, err := imagex.Normalize(raw, normOpt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	path, err := f.tempManager().Write(ctx, img)
	if err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "写入临时文件失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	payload := render.NewPathPayload(img, path)
	ad := adapter.ByTarget(target)
	if ad == nil {
		errs.Fprint(os.Stderr, errs.E(errs.Code("E006"), "目标适配器不支持"))
		fmt.Fprintln(os.Stderr)
		return 2
	}
	out, err := ad.AdaptPath(payload, opt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	if f.copy {
		if err := clipboard.WriteText(ctx, out); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}

	fmt.Print(out)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return 0
}

func runDataURI(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("data-uri", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f commonFlags
	f.bind(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := f.gcIfNeeded(ctx); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "临时目录不可写或清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}

	normOpt, err := f.normalizeOptions()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 2
	}

	prov := clipboard.NewSelector().Select()
	raw, err := prov.GetImage(ctx)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	img, err := imagex.Normalize(raw, normOpt)
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	payload := render.NewDataURIPayload(img)
	out := payload.DataURI
	if f.json {
		b, jerr := json.MarshalIndent(payload.JSON, "", "  ")
		if jerr != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "生成 JSON 失败", jerr))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		out = string(b)
	}
	if f.copy {
		if err := clipboard.WriteText(ctx, out); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}

	fmt.Print(out)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return 0
}

func runInspect(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "输出 JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if asJSON {
		rep := diagnostics.InspectJSON(ctx, diagnostics.InspectOptions{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
		b, err := diagnostics.MarshalReport(rep)
		if err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "生成 JSON 失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Println(string(b))
		return 0
	}

	info := diagnostics.Inspect(ctx, diagnostics.InspectOptions{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	fmt.Print(info)
	if len(info) == 0 || info[len(info)-1] != '\n' {
		fmt.Println()
	}
	return 0
}

func runDoctor(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "输出 JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if asJSON {
		rep := diagnostics.DoctorJSON(ctx, diagnostics.DoctorOptions{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
		b, err := diagnostics.MarshalReport(rep)
		if err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "生成 JSON 失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Println(string(b))
		if rep.Error != nil {
			return 1
		}
		return 0
	}

	res := diagnostics.Doctor(ctx, diagnostics.DoctorOptions{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	if res.Err != nil {
		errs.Fprint(os.Stderr, res.Err)
		fmt.Fprintln(os.Stderr)
	}
	if res.Text != "" {
		fmt.Print(res.Text)
		if res.Text[len(res.Text)-1] != '\n' {
			fmt.Println()
		}
	}
	if res.Err != nil {
		return 1
	}
	return 0
}

func runGC(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("gc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var tempDir string
	var ttl time.Duration
	var force bool
	fs.StringVar(&tempDir, "temp-dir", "", "临时目录（默认使用系统临时目录下的 clip2agent/）")
	fs.DurationVar(&ttl, "ttl", 10*time.Minute, "TTL")
	fs.BoolVar(&force, "force", false, "允许清理 kept-dir（危险，默认拒绝）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(tempDir) == "" {
		// default temp dir: os.TempDir()/clip2agent
		// 这里保持 temp.Manager 的默认行为。
	} else {
		// 明确边界：默认拒绝清理 kept-dir。
		if filepath.Clean(tempDir) == filepath.Clean(paths.DefaultKeptDir()) && !force {
			errs.Fprint(os.Stderr, errs.E("E006", "拒绝清理 kept-dir", errs.Hint("如确认需要清理，请加 --force")))
			fmt.Fprintln(os.Stderr)
			return 2
		}
	}
	if err := temp.NewManager(temp.Options{BaseDir: tempDir}).GC(ctx, ttl); err != nil {
		errs.Fprint(os.Stderr, errs.WrapCode("E007", "清理失败", err))
		fmt.Fprintln(os.Stderr)
		return 1
	}
	return 0
}

// --- config subcommand ---

type hotkeyConfigFile struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Bindings      []hotkeyBinding `json:"bindings"`
}

type hotkeyBinding struct {
	ID       string       `json:"id,omitempty"`
	Name     string       `json:"name,omitempty"`
	Enabled  *bool        `json:"enabled,omitempty"`
	Shortcut string       `json:"shortcut"`
	Action   hotkeyAction `json:"action"`
}

type hotkeyAction struct {
	Type    string            `json:"type"`
	Args    []string          `json:"args,omitempty"`
	Command string            `json:"command,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Post    *hotkeyPost       `json:"post,omitempty"`
}

type hotkeyPost struct {
	Paste *hotkeyPaste `json:"paste,omitempty"`
}

type hotkeyPaste struct {
	Enabled *bool `json:"enabled,omitempty"`
	DelayMs *int  `json:"delay_ms,omitempty"`
}

func runConfig(_ context.Context, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: clip2agent config <init|show|path|bindings>")
		return 2
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "path":
		fmt.Println(paths.HotkeyConfigPath())
		return 0
	case "show":
		p := paths.HotkeyConfigPath()
		b, err := os.ReadFile(p)
		if err != nil {
			errs.Fprint(os.Stderr, errs.E("E003", "配置文件不存在或不可读", errs.Hint("运行: clip2agent config init"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Print(string(b))
		if len(b) == 0 || b[len(b)-1] != '\n' {
			fmt.Println()
		}
		return 0
	case "bindings":
		p := paths.HotkeyConfigPath()
		b, err := os.ReadFile(p)
		if err != nil {
			errs.Fprint(os.Stderr, errs.E("E003", "配置文件不存在或不可读", errs.Hint("运行: clip2agent config init"), errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		var cfg hotkeyConfigFile
		if err := json.Unmarshal(b, &cfg); err != nil {
			errs.Fprint(os.Stderr, errs.E("E006", "配置文件 JSON 解析失败", errs.Cause(err)))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if len(cfg.Bindings) == 0 {
			fmt.Println("(no bindings)")
			return 0
		}
		for i, bd := range cfg.Bindings {
			act := strings.ToLower(strings.TrimSpace(bd.Action.Type))
			label := bd.Shortcut
			if strings.TrimSpace(bd.Name) != "" {
				label = bd.Name
			} else if strings.TrimSpace(bd.ID) != "" {
				label = bd.ID
			}
			enabled := true
			if bd.Enabled != nil {
				enabled = *bd.Enabled
			}
			desc := act
			if len(bd.Action.Args) > 0 {
				desc += " " + strings.Join(bd.Action.Args, " ")
			} else if bd.Action.Command != "" {
				desc += " " + bd.Action.Command
			}
			fmt.Printf("%d. %s (enabled=%v) -> %s\n", i+1, strings.TrimSpace(label), enabled, strings.TrimSpace(desc))
		}
		return 0
	case "init":
		fs := flag.NewFlagSet("config init", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var force bool
		fs.BoolVar(&force, "force", false, "覆盖已存在的配置文件")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		p := paths.HotkeyConfigPath()
		if !force {
			if _, err := os.Stat(p); err == nil {
				fmt.Println(p)
				return 0
			}
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "创建配置目录失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		cfg := defaultHotkeyConfig()
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "生成默认配置失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		out = append(out, '\n')
		if err := os.WriteFile(p, out, 0o600); err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "写入配置文件失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		fmt.Println(p)
		return 0
	default:
		errs.Fprint(os.Stderr, errs.E("E006", "不支持的 config 子命令", errs.Hint("支持: init/show/path/bindings")))
		fmt.Fprintln(os.Stderr)
		return 2
	}
}

func defaultHotkeyConfig() hotkeyConfigFile {
	// 为便于 diff：按 shortcut 排序输出。
	falseVal := false
	delay := 80
	bindings := []hotkeyBinding{
		{
			ID:       "coco",
			Name:     "Coco 引用",
			Shortcut: "control+option+command+v",
			Action: hotkeyAction{
				Type: "clip2agent",
				Args: []string{"path", "--target", "coco", "--copy"},
				Post: &hotkeyPost{Paste: &hotkeyPaste{Enabled: &falseVal, DelayMs: &delay}},
			},
		},
		{
			ID:       "openai-json",
			Name:     "OpenAI JSON",
			Shortcut: "control+option+command+b",
			Action: hotkeyAction{
				Type: "clip2agent",
				Args: []string{"base64", "--target", "openai", "--json", "--copy"},
			},
		},
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Shortcut < bindings[j].Shortcut })
	return hotkeyConfigFile{SchemaVersion: 1, Bindings: bindings}
}

func loadHotkeyConfig() (hotkeyConfigFile, error) {
	p := paths.HotkeyConfigPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return hotkeyConfigFile{}, errs.E("E003", "配置文件不存在或不可读", errs.Hint("运行: clip2agent config init"), errs.Cause(err))
	}
	var cfg hotkeyConfigFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		return hotkeyConfigFile{}, errs.E("E006", "配置文件 JSON 解析失败", errs.Cause(err))
	}
	if cfg.SchemaVersion != 0 && cfg.SchemaVersion != 1 {
		return hotkeyConfigFile{}, errs.E("E006", "不支持的 hotkey 配置 schema_version", errs.Hint("请运行: clip2agent config init --force"))
	}
	return cfg, nil
}
