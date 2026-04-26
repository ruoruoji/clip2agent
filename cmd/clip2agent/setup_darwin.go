//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/diagnostics"
	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/paths"
)

func runSetup(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var binDir string
	var install bool
	var withHotkey bool
	var withLaunchd bool
	var forceConfig bool
	var rebuild bool
	var verify bool
	var jsonOut bool
	fs.StringVar(&binDir, "bin-dir", defaultBinDir(), "安装目录（默认 ~/.local/bin）")
	fs.BoolVar(&install, "install", true, "安装/覆盖到 bin-dir")
	fs.BoolVar(&withHotkey, "hotkey", true, "构建/安装 clip2agent-hotkey")
	fs.BoolVar(&withLaunchd, "launchd", true, "安装并启动 LaunchAgent（仅当 hotkey=true）")
	fs.BoolVar(&forceConfig, "force-config", false, "覆盖 hotkey.json")
	fs.BoolVar(&rebuild, "rebuild", false, "强制重新 swift build")
	fs.BoolVar(&verify, "verify", false, "只做检查并输出下一步建议（无副作用）")
	fs.BoolVar(&jsonOut, "json", false, "配合 --verify 输出 JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if verify {
		rep := diagnostics.SetupVerifyJSON(ctx, diagnostics.SetupVerifyOptions{BinDir: binDir})
		if jsonOut {
			b, err := diagnostics.MarshalReport(rep)
			if err != nil {
				err2 := errs.WrapCode("E007", "生成 JSON 失败", err)
				errs.Fprint(os.Stderr, err2)
				fmt.Fprintln(os.Stderr)
				return 1
			}
			os.Stdout.Write(append(b, '\n'))
		} else {
			fmt.Print(diagnostics.SetupVerifyText(rep))
		}
		if rep.Error != nil {
			return 1
		}
		// verify 仅在“核心缺失”时返回非零。
		if len(rep.Missing) > 0 {
			return 1
		}
		return 0
	}

	root, err := findRepoRootForSetup()
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}
	macDir := filepath.Join(root, "native", "macos")

	// 找到构建产物（优先复用已有 .build，避免在资源受限环境下编译失败）。
	helperBuilt, err := findSwiftBuildProduct(macDir, "clip2agent-macos")
	needBuild := rebuild || err != nil
	if needBuild {
		if err := runSwiftBuild(ctx, macDir); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		helperBuilt, err = findSwiftBuildProduct(macDir, "clip2agent-macos")
	}
	if err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	var hotkeyBuilt string
	if withHotkey {
		hotkeyBuilt, err = findSwiftBuildProduct(macDir, "clip2agent-hotkey")
		if err != nil && needBuild {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if err != nil {
			if err := runSwiftBuild(ctx, macDir); err != nil {
				errs.Fprint(os.Stderr, err)
				fmt.Fprintln(os.Stderr)
				return 1
			}
			hotkeyBuilt, err = findSwiftBuildProduct(macDir, "clip2agent-hotkey")
		}
		if err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}

	// 初始化配置
	if err := ensureHotkeyConfig(forceConfig); err != nil {
		errs.Fprint(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		return 1
	}

	// 安装到 ~/.local/bin（并把 clip2agent 自身也放进去，避免 go run 的临时二进制）
	selfExe, _ := os.Executable()
	clip2agentDst := filepath.Join(binDir, "clip2agent")
	helperDst := filepath.Join(binDir, "clip2agent-macos")
	hotkeyDst := filepath.Join(binDir, "clip2agent-hotkey")
	if install {
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			errs.Fprint(os.Stderr, errs.WrapCode("E007", "创建 bin-dir 失败", err))
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if selfExe != "" {
			_ = copyFile(selfExe, clip2agentDst, 0o755)
		}
		if err := copyFile(helperBuilt, helperDst, 0o755); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
		if withHotkey {
			if err := copyFile(hotkeyBuilt, hotkeyDst, 0o755); err != nil {
				errs.Fprint(os.Stderr, err)
				fmt.Fprintln(os.Stderr)
				return 1
			}
		}
	}

	if withHotkey && withLaunchd {
		clipPath := clip2agentDst
		if !install {
			clipPath = selfExe
		}
		helperPath := helperDst
		if !install {
			helperPath = helperBuilt
		}
		hkPath := hotkeyDst
		if !install {
			hkPath = hotkeyBuilt
		}
		if err := installAndStartLaunchAgent(binDir, clipPath, helperPath, hkPath); err != nil {
			errs.Fprint(os.Stderr, err)
			fmt.Fprintln(os.Stderr)
			return 1
		}
	}

	fmt.Printf("ok: config=%s\n", paths.HotkeyConfigPath())
	if install {
		fmt.Printf("ok: bin-dir=%s\n", binDir)
	}
	if withHotkey {
		fmt.Printf("ok: hotkey=%v launchd=%v\n", withHotkey, withLaunchd)
	}
	fmt.Println("next:")
	fmt.Println("- 运行: clip2agent doctor")
	if withHotkey {
		fmt.Println("- 运行: clip2agent hotkey doctor")
		fmt.Println("- 菜单栏：找到 C2A（clip2agent-hotkey）")
		fmt.Println("- 若启用自动粘贴：给你的终端与 clip2agent-hotkey 授予 系统设置→隐私与安全性→辅助功能 权限")
		fmt.Println("- 触发热键：control+option+command+v")
	}
	return 0
}

func defaultBinDir() string {
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return filepath.Join(".", ".local", "bin")
	}
	return filepath.Join(h, ".local", "bin")
}

func findRepoRootForSetup() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errs.WrapCode("E007", "获取当前目录失败", err)
	}
	d := cwd
	for i := 0; i < 12; i++ {
		cand := filepath.Join(d, "native", "macos", "Package.swift")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", errs.E("E006", "未找到 native/macos/Package.swift", errs.Hint("请在仓库根目录运行: clip2agent setup"))
}

func runSwiftBuild(ctx context.Context, macDir string) error {
	if _, err := exec.LookPath("swift"); err != nil {
		return errs.E("E003", "缺少 swift", errs.Hint("安装 Xcode Command Line Tools: xcode-select --install"), errs.Cause(err))
	}
	jobs := 1
	if v := strings.TrimSpace(os.Getenv("CLIP2AGENT_SWIFT_JOBS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			jobs = n
		}
	}
	argv := []string{"build", "-c", "release", "--jobs", strconv.Itoa(jobs)}
	cmd := exec.CommandContext(ctx, "swift", argv...)
	cmd.Dir = macDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		h := strings.TrimSpace(string(out))
		if h != "" {
			return errs.E("E003", "Swift 构建失败", errs.Hint(h), errs.Cause(err))
		}
		return errs.E("E003", "Swift 构建失败", errs.Hint("可能是资源不足导致编译进程被系统终止；可尝试: CLIP2AGENT_SWIFT_JOBS=1 clip2agent setup"), errs.Cause(err))
	}
	return nil
}

func findSwiftBuildProduct(macDir string, name string) (string, error) {
	root := filepath.Join(macDir, ".build")
	var found string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(p) == name && strings.Contains(p, string(filepath.Separator)+"release"+string(filepath.Separator)) {
			found = p
			return io.EOF
		}
		return nil
	})
	if found != "" {
		return found, nil
	}
	return "", errs.E("E003", "未找到 Swift 构建产物", errs.Hint("请确认 native/macos 已成功构建并包含目标: "+name))
}

func ensureHotkeyConfig(force bool) error {
	p := paths.HotkeyConfigPath()
	if !force {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return errs.WrapCode("E007", "创建配置目录失败", err)
	}
	cfg := defaultHotkeyConfig()
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errs.WrapCode("E007", "生成默认配置失败", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(p, out, 0o600); err != nil {
		return errs.WrapCode("E007", "写入配置文件失败", err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return errs.WrapCode("E007", "读取源文件失败", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return errs.WrapCode("E007", "写入目标文件失败", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return errs.WrapCode("E007", "拷贝文件失败", err)
	}
	if err := out.Close(); err != nil {
		return errs.WrapCode("E007", "关闭目标文件失败", err)
	}
	_ = os.Chmod(dst, mode)
	return nil
}

func installAndStartLaunchAgent(binDir, clip2agentPath, helperPath, hotkeyPath string) error {
	uid := os.Getuid()
	label := paths.HotkeyLaunchAgentLabel
	plistPath := paths.HotkeyLaunchAgentPlistPath()
	h, _ := os.UserHomeDir()
	if strings.TrimSpace(plistPath) == "" {
		return errs.E("E007", "获取 LaunchAgent plist 路径失败")
	}
	logPath := filepath.Join(h, "Library", "Logs", "clip2agent-hotkey.log")

	if strings.TrimSpace(hotkeyPath) == "" {
		return errs.E("E006", "clip2agent-hotkey 路径为空")
	}
	if _, err := os.Stat(hotkeyPath); err != nil {
		return errs.E("E003", "clip2agent-hotkey 不存在", errs.Hint(hotkeyPath), errs.Cause(err))
	}
	if strings.TrimSpace(clip2agentPath) != "" {
		if _, err := os.Stat(clip2agentPath); err != nil {
			return errs.E("E003", "clip2agent 不存在", errs.Hint(clip2agentPath), errs.Cause(err))
		}
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return errs.WrapCode("E007", "创建 LaunchAgents 目录失败", err)
	}

	pathEnv := binDir + ":/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	env := map[string]string{
		"PATH":                    pathEnv,
		"CLIP2AGENT_BIN":          clip2agentPath,
		"CLIP2AGENT_MACOS_HELPER": strings.TrimSpace(helperPath),
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		env["XDG_CONFIG_HOME"] = xdg
	}
	plist := renderLaunchAgentPlist(label, hotkeyPath, logPath, env)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return errs.WrapCode("E007", "写入 LaunchAgent plist 失败", err)
	}

	_ = exec.Command("launchctl", "bootout", "gui/"+strconv.Itoa(uid)+"/"+label).Run()
	if out, err := exec.Command("launchctl", "bootstrap", "gui/"+strconv.Itoa(uid), plistPath).CombinedOutput(); err != nil {
		h := strings.TrimSpace(string(out))
		if h != "" {
			return errs.E("E003", "launchctl bootstrap 失败", errs.Hint(h), errs.Cause(err))
		}
		return errs.WrapCode("E003", "launchctl bootstrap 失败", err)
	}
	_ = exec.Command("launchctl", "kickstart", "-k", "gui/"+strconv.Itoa(uid)+"/"+label).Run()
	return nil
}

func renderLaunchAgentPlist(label, hotkeyPath, logPath string, env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	fmt.Fprintf(&b, "  <key>Label</key><string>%s</string>\n", xmlEscape(label))
	b.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	fmt.Fprintf(&b, "    <string>%s</string>\n", xmlEscape(hotkeyPath))
	b.WriteString("  </array>\n")
	b.WriteString("  <key>RunAtLoad</key><true/>\n")
	b.WriteString("  <key>KeepAlive</key><true/>\n")
	fmt.Fprintf(&b, "  <key>StandardOutPath</key><string>%s</string>\n", xmlEscape(logPath))
	fmt.Fprintf(&b, "  <key>StandardErrorPath</key><string>%s</string>\n", xmlEscape(logPath))
	b.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "    <key>%s</key><string>%s</string>\n", xmlEscape(k), xmlEscape(env[k]))
	}
	b.WriteString("  </dict>\n")
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
