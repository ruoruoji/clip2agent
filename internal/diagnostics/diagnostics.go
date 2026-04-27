package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/paths"
)

type InspectOptions struct {
	GOOS   string
	GOARCH string
}

type InspectReport struct {
	SchemaVersion int               `json:"schema_version"`
	Platform      string            `json:"platform"`
	GOOS          string            `json:"goos"`
	GOARCH        string            `json:"goarch"`
	Env           map[string]string `json:"env,omitempty"`
	Checks        map[string]bool   `json:"checks,omitempty"`
	Paths         map[string]string `json:"paths,omitempty"`
	Notes         []string          `json:"notes,omitempty"`
}

func InspectJSON(_ context.Context, opt InspectOptions) InspectReport {
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if opt.GOARCH == "" {
		opt.GOARCH = runtime.GOARCH
	}
	rep := InspectReport{
		SchemaVersion: 1,
		Platform:      fmt.Sprintf("%s/%s", opt.GOOS, opt.GOARCH),
		GOOS:          opt.GOOS,
		GOARCH:        opt.GOARCH,
		Env:           map[string]string{},
		Checks:        map[string]bool{},
		Paths:         map[string]string{},
	}
	if v := strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")); v != "" {
		rep.Env["WAYLAND_DISPLAY"] = v
	}
	if v := strings.TrimSpace(os.Getenv("DISPLAY")); v != "" {
		rep.Env["DISPLAY"] = v
	}

	switch opt.GOOS {
	case "darwin":
		rep.Env["CLIP2AGENT_MACOS_HELPER"] = strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER"))
		rep.Checks["macos_helper_in_path"] = lookPathOK("clip2agent-macos")
		rep.Paths["hotkey_config"] = paths.HotkeyConfigPath()
		rep.Paths["hotkey_launchagent"] = paths.HotkeyLaunchAgentPlistPath()
	case "linux":
		rep.Checks["wl-paste"] = lookPathOK("wl-paste")
		rep.Checks["wl-copy"] = lookPathOK("wl-copy")
		rep.Checks["xclip"] = lookPathOK("xclip")
		rep.Checks["xsel"] = lookPathOK("xsel")
		rep.Checks["xbindkeys"] = lookPathOK("xbindkeys")
		rep.Paths["hotkey_config"] = paths.HotkeyConfigPath()
	case "windows":
		rep.Checks["powershell"] = lookPathOK("powershell") || lookPathOK("powershell.exe")
		rep.Checks["pwsh"] = lookPathOK("pwsh") || lookPathOK("pwsh.exe")
		rep.Paths["hotkey_config"] = paths.HotkeyConfigPath()
	default:
		rep.Notes = append(rep.Notes, "unsupported platform")
	}
	return rep
}

func Inspect(_ context.Context, opt InspectOptions) string {
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if opt.GOARCH == "" {
		opt.GOARCH = runtime.GOARCH
	}

	var b strings.Builder
	fmt.Fprintf(&b, "platform: %s/%s\n", opt.GOOS, opt.GOARCH)
	if v := os.Getenv("WAYLAND_DISPLAY"); v != "" {
		fmt.Fprintf(&b, "WAYLAND_DISPLAY: %s\n", v)
	}
	if v := os.Getenv("DISPLAY"); v != "" {
		fmt.Fprintf(&b, "DISPLAY: %s\n", v)
	}

	switch opt.GOOS {
	case "darwin":
		fmt.Fprintf(&b, "macos_helper_env: %s\n", strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER")))
		fmt.Fprintf(&b, "macos_helper_in_path: %v\n", lookPathOK("clip2agent-macos"))
		fmt.Fprintf(&b, "hotkey_config: %s\n", paths.HotkeyConfigPath())
		fmt.Fprintf(&b, "hotkey_launchagent: %s\n", paths.HotkeyLaunchAgentPlistPath())
	case "linux":
		fmt.Fprintf(&b, "wl-paste: %v\n", lookPathOK("wl-paste"))
		fmt.Fprintf(&b, "wl-copy: %v\n", lookPathOK("wl-copy"))
		fmt.Fprintf(&b, "xclip: %v\n", lookPathOK("xclip"))
		fmt.Fprintf(&b, "xsel: %v\n", lookPathOK("xsel"))
		fmt.Fprintf(&b, "xbindkeys: %v\n", lookPathOK("xbindkeys"))
		fmt.Fprintf(&b, "hotkey_config: %s\n", paths.HotkeyConfigPath())
	case "windows":
		fmt.Fprintf(&b, "powershell: %v\n", lookPathOK("powershell") || lookPathOK("powershell.exe"))
		fmt.Fprintf(&b, "pwsh: %v\n", lookPathOK("pwsh") || lookPathOK("pwsh.exe"))
		fmt.Fprintf(&b, "hotkey_config: %s\n", paths.HotkeyConfigPath())
	default:
		fmt.Fprintf(&b, "note: unsupported platform\n")
	}

	return b.String()
}

type DoctorOptions struct {
	GOOS   string
	GOARCH string
}

type ErrorJSON struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Hint    string `json:"hint,omitempty"`
	Cause   string `json:"cause,omitempty"`
}

type DoctorReport struct {
	SchemaVersion int               `json:"schema_version"`
	Timestamp     string            `json:"timestamp"`
	Platform      string            `json:"platform"`
	GOOS          string            `json:"goos"`
	GOARCH        string            `json:"goarch"`
	Checks        map[string]bool   `json:"checks,omitempty"`
	Missing       []string          `json:"missing,omitempty"`
	Notes         []string          `json:"notes,omitempty"`
	Fix           []string          `json:"fix,omitempty"`
	Next          []string          `json:"next,omitempty"`
	Paths         map[string]string `json:"paths,omitempty"`
	Error         *ErrorJSON        `json:"error,omitempty"`
}

type DoctorResult struct {
	Text string
	Err  error
}

// --- setup verify (darwin) ---

type SetupVerifyOptions struct {
	BinDir string
}

type SetupVerifyReport struct {
	SchemaVersion int               `json:"schema_version"`
	Timestamp     string            `json:"timestamp"`
	Platform      string            `json:"platform"`
	GOOS          string            `json:"goos"`
	GOARCH        string            `json:"goarch"`
	Checks        map[string]bool   `json:"checks,omitempty"`
	Missing       []string          `json:"missing,omitempty"`
	Notes         []string          `json:"notes,omitempty"`
	Fix           []string          `json:"fix,omitempty"`
	Next          []string          `json:"next,omitempty"`
	Paths         map[string]string `json:"paths,omitempty"`
	Error         *ErrorJSON        `json:"error,omitempty"`
}

func appendUniqueStrings(dst []string, vals ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, v := range dst {
		seen[v] = struct{}{}
	}
	for _, v := range vals {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		dst = append(dst, v)
		seen[v] = struct{}{}
	}
	return dst
}

func addDarwinResetAdvice(fix *[]string, next *[]string) {
	*fix = appendUniqueStrings(*fix,
		"若当前是本地开发中的脏环境/残留态，先运行: clip2agent uninstall --purge --yes",
		"清理后运行: clip2agent doctor",
		"重新安装后运行: clip2agent setup",
	)
	*next = appendUniqueStrings(*next,
		"clip2agent uninstall --purge --yes",
		"clip2agent doctor",
		"clip2agent setup",
		"clip2agent setup --verify",
	)
}

func SetupVerifyJSON(ctx context.Context, opt SetupVerifyOptions) SetupVerifyReport {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	rep := SetupVerifyReport{
		SchemaVersion: 1,
		Timestamp:     time.Now().Format(time.RFC3339),
		Platform:      fmt.Sprintf("%s/%s", goos, goarch),
		GOOS:          goos,
		GOARCH:        goarch,
		Checks:        map[string]bool{},
		Paths:         map[string]string{},
	}

	if strings.TrimSpace(opt.BinDir) != "" {
		rep.Paths["bin_dir"] = opt.BinDir
		if !pathContainsDir(os.Getenv("PATH"), opt.BinDir) {
			rep.Notes = append(rep.Notes, "PATH 未包含 bin-dir")
			rep.Fix = append(rep.Fix, fmt.Sprintf("export PATH=\"%s:$PATH\"", opt.BinDir))
			rep.Fix = append(rep.Fix, "将上面的 export 写入 shell rc（例如 ~/.zshrc）")
			rep.Checks["path_contains_bin_dir"] = false
		} else {
			rep.Checks["path_contains_bin_dir"] = true
		}
	}

	// self exe
	selfExe, _ := os.Executable()
	rep.Paths["self_exe"] = strings.TrimSpace(selfExe)
	selfDir := ""
	if strings.TrimSpace(selfExe) != "" {
		selfDir = filepath.Dir(selfExe)
		rep.Paths["self_dir"] = selfDir
	}

	// binaries
	rep.Checks["clip2agent_in_path"] = lookPathOK("clip2agent")
	rep.Checks["clip2agent_hotkey_in_path"] = lookPathOK("clip2agent-hotkey")
	rep.Checks["clip2agent_macos_in_path"] = lookPathOK("clip2agent-macos")

	if selfDir != "" {
		rep.Checks["clip2agent_sibling"] = fileExists(filepath.Join(selfDir, "clip2agent"))
		rep.Checks["clip2agent_hotkey_sibling"] = fileExists(filepath.Join(selfDir, "clip2agent-hotkey"))
		rep.Checks["clip2agent_macos_sibling"] = fileExists(filepath.Join(selfDir, "clip2agent-macos"))
	}

	// macOS helper resolution (最关键)
	if goos == "darwin" {
		helperEnv := strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER"))
		rep.Paths["macos_helper_env"] = helperEnv
		helperOK := false
		if helperEnv != "" {
			if fileExists(helperEnv) {
				helperOK = true
				rep.Checks["macos_helper_from_env"] = true
			} else {
				rep.Checks["macos_helper_from_env"] = false
				rep.Notes = append(rep.Notes, "CLIP2AGENT_MACOS_HELPER 指向的文件不存在")
				rep.Fix = append(rep.Fix, "运行: clip2agent setup")
				addDarwinResetAdvice(&rep.Fix, &rep.Next)
			}
		}
		if !helperOK {
			if rep.Checks["clip2agent_macos_in_path"] {
				helperOK = true
				rep.Checks["macos_helper"] = true
			} else if selfDir != "" && rep.Checks["clip2agent_macos_sibling"] {
				helperOK = true
				rep.Checks["macos_helper"] = true
			}
		}
		if !helperOK {
			rep.Missing = append(rep.Missing, "clip2agent-macos")
			rep.Notes = append(rep.Notes, "`setup --verify` 面向安装后验证；若你刚完成清理，这是待重装状态")
			rep.Fix = append(rep.Fix, "运行: clip2agent setup")
			addDarwinResetAdvice(&rep.Fix, &rep.Next)
			rep.Error = errToJSON(errs.E("E003", "缺少 macOS 剪贴板 helper（clip2agent-macos）"))
			return rep
		}
		rep.Checks["macos_helper"] = true
		rep.Notes = append(rep.Notes, "auto-paste 权限请在菜单栏 C2A（clip2agent-hotkey）中查看/引导")
	}

	// hotkey config
	cfgPath := paths.HotkeyConfigPath()
	rep.Paths["hotkey_config"] = cfgPath
	if cfgPath != "" {
		if fileExists(cfgPath) {
			rep.Checks["hotkey_config_exists"] = true
			if err := validateHotkeyConfig(cfgPath); err != nil {
				rep.Checks["hotkey_config_valid"] = false
				rep.Notes = append(rep.Notes, "hotkey.json 配置不可用")
				rep.Fix = append(rep.Fix, "运行: clip2agent config init --force")
				addDarwinResetAdvice(&rep.Fix, &rep.Next)
			} else {
				rep.Checks["hotkey_config_valid"] = true
				// 额外检查：至少有一条 enabled=true
				if n := countEnabledBindings(cfgPath); n <= 0 {
					rep.Notes = append(rep.Notes, "hotkey.json 中没有启用的 bindings")
					rep.Fix = append(rep.Fix, "运行: clip2agent config init --force")
					// 不作为 missing；但在 verify 中提示。
					rep.Checks["hotkey_bindings_enabled"] = false
				} else {
					rep.Checks["hotkey_bindings_enabled"] = true
				}
			}
		} else {
			rep.Checks["hotkey_config_exists"] = false
			rep.Notes = append(rep.Notes, "hotkey.json 未生成")
			rep.Fix = append(rep.Fix, "运行: clip2agent config init")
			addDarwinResetAdvice(&rep.Fix, &rep.Next)
		}
	}

	// LaunchAgent
	if goos == "darwin" {
		plist := paths.HotkeyLaunchAgentPlistPath()
		rep.Paths["hotkey_launchagent"] = plist
		if plist != "" {
			rep.Checks["hotkey_launchagent_installed"] = fileExists(plist)
			uid := os.Getuid()
			svc := "gui/" + strconv.Itoa(uid) + "/" + paths.HotkeyLaunchAgentLabel
			rep.Paths["launchctl_service"] = svc
			ok, hint := checkLaunchctlService(ctx, svc)
			rep.Checks["hotkey_service_running"] = ok
			if !ok {
				rep.Notes = append(rep.Notes, "hotkey service not running")
				rep.Fix = append(rep.Fix, "运行: clip2agent hotkey restart")
				addDarwinResetAdvice(&rep.Fix, &rep.Next)
				if hint != "" {
					rep.Paths["launchctl_print"] = hint
				}
			}
			if !rep.Checks["hotkey_launchagent_installed"] {
				rep.Notes = append(rep.Notes, "hotkey LaunchAgent not installed")
				rep.Fix = append(rep.Fix, "运行: clip2agent hotkey install")
				addDarwinResetAdvice(&rep.Fix, &rep.Next)
			}
		}
	}

	rep.Next = appendUniqueStrings(rep.Next, "clip2agent doctor", "clip2agent hotkey doctor")
	return rep
}

func SetupVerifyText(rep SetupVerifyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "setup verify @ %s\n", rep.Timestamp)
	fmt.Fprintf(&b, "platform: %s\n", rep.Platform)
	if v := strings.TrimSpace(rep.Paths["bin_dir"]); v != "" {
		fmt.Fprintf(&b, "bin_dir: %s\n", v)
	}
	if v := strings.TrimSpace(rep.Paths["hotkey_config"]); v != "" {
		fmt.Fprintf(&b, "hotkey_config: %s\n", v)
	}
	if v := strings.TrimSpace(rep.Paths["hotkey_launchagent"]); v != "" {
		fmt.Fprintf(&b, "hotkey_launchagent: %s\n", v)
	}
	if v := strings.TrimSpace(rep.Paths["self_exe"]); v != "" {
		fmt.Fprintf(&b, "self_exe: %s\n", v)
	}
	if len(rep.Missing) > 0 {
		fmt.Fprintf(&b, "missing: %s\n", strings.Join(rep.Missing, ","))
	}
	for _, n := range rep.Notes {
		fmt.Fprintf(&b, "note: %s\n", n)
	}
	for _, f := range rep.Fix {
		fmt.Fprintf(&b, "fix: %s\n", f)
	}
	for _, n := range rep.Next {
		fmt.Fprintf(&b, "next: %s\n", n)
	}
	if rep.Error != nil {
		fmt.Fprintf(&b, "error: %s %s\n", rep.Error.Code, rep.Error.Message)
		if strings.TrimSpace(rep.Error.Hint) != "" {
			fmt.Fprintf(&b, "hint: %s\n", rep.Error.Hint)
		}
	}
	return b.String()
}

func Doctor(ctx context.Context, opt DoctorOptions) DoctorResult {
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if opt.GOARCH == "" {
		opt.GOARCH = runtime.GOARCH
	}

	var b strings.Builder
	fmt.Fprintf(&b, "doctor @ %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "platform: %s/%s\n", opt.GOOS, opt.GOARCH)
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "tips: 首次使用建议先运行 doctor，再复制一张图片执行 `clip2agent coco --copy` 或 `clip2agent openai-json --copy`")

	// clip2agent 是否在 PATH（不阻塞，仅提示）
	if !lookPathOK("clip2agent") {
		fmt.Fprintln(&b, "note: clip2agent not found in PATH")
		fmt.Fprintln(&b, "fix: (repo) go install ./cmd/clip2agent")
		fmt.Fprintln(&b, "fix: 确保 PATH 包含安装目录（如 ~/.local/bin）")
		if opt.GOOS == "darwin" {
			fmt.Fprintln(&b, "fix: clip2agent setup")
		}
	}

	// temp dir 可写性
	if err := checkTempWritable(); err != nil {
		return DoctorResult{Text: b.String(), Err: errs.WrapCode("E007", "临时目录不可写", err)}
	}

	switch opt.GOOS {
	case "darwin":
		if !lookPathOK("clip2agent-macos") && strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER")) == "" {
			fmt.Fprintln(&b, "missing: clip2agent-macos")
			fmt.Fprintln(&b, "note: `setup --verify` 更适合安装后验证；如果你刚完成清理，这是待重装状态")
			fmt.Fprintln(&b, "fix: 若怀疑当前环境有残留，先运行: clip2agent uninstall --purge --yes")
			fmt.Fprintln(&b, "fix: 清理后运行: clip2agent doctor")
			fmt.Fprintln(&b, "fix: clip2agent setup")
			fmt.Fprintln(&b, "fix: (repo) go run ./cmd/clip2agent setup")
			fmt.Fprintln(&b, "next: clip2agent setup --verify")
			return DoctorResult{Text: b.String(), Err: errs.E("E003", "缺少 macOS 剪贴板 helper（clip2agent-macos）")}
		}
		fmt.Fprintln(&b, "ok: macOS helper")
		fmt.Fprintln(&b, "note: 菜单栏常驻为 C2A（clip2agent-hotkey）")
		fmt.Fprintln(&b, "note: 若启用自动粘贴（post.paste.enabled=true），需要授予系统设置→隐私与安全性→辅助功能 权限")
		// hotkey 配置与 LaunchAgent 属于可选项：给出提示但不阻塞 doctor。
		cfgPath := paths.HotkeyConfigPath()
		if _, err := os.Stat(cfgPath); err != nil {
			fmt.Fprintln(&b, "note: hotkey config missing")
			fmt.Fprintln(&b, "fix: clip2agent config init")
			fmt.Fprintln(&b, "fix: 若当前环境有残留，先运行: clip2agent uninstall --purge --yes")
		} else {
			if err := validateHotkeyConfig(cfgPath); err != nil {
				fmt.Fprintln(&b, "note: hotkey config invalid")
				errs.Fprint(&b, err)
				fmt.Fprintln(&b)
				fmt.Fprintln(&b, "fix: clip2agent config init --force")
				fmt.Fprintln(&b, "fix: 若怀疑是脏环境/残留态，先运行: clip2agent uninstall --purge --yes")
			}
		}
		if plist := paths.HotkeyLaunchAgentPlistPath(); strings.TrimSpace(plist) != "" {
			if _, err := os.Stat(plist); err != nil {
				fmt.Fprintln(&b, "note: hotkey LaunchAgent not installed")
				fmt.Fprintln(&b, "fix: clip2agent hotkey install")
				fmt.Fprintln(&b, "fix: 若当前环境曾反复安装/卸载，先运行: clip2agent uninstall --purge --yes")
			} else {
				uid := os.Getuid()
				svc := "gui/" + strconv.Itoa(uid) + "/" + paths.HotkeyLaunchAgentLabel
				if ok, _ := checkLaunchctlService(ctx, svc); ok {
					fmt.Fprintln(&b, "ok: hotkey service")
				} else {
					fmt.Fprintln(&b, "note: hotkey service not running")
					fmt.Fprintln(&b, "fix: clip2agent hotkey restart")
					fmt.Fprintln(&b, "fix: 若怀疑 LaunchAgent 残留，先运行: clip2agent uninstall --purge --yes")
				}
			}
		}
		fmt.Fprintln(&b, "next: clip2agent inspect")
		fmt.Fprintln(&b, "next: clip2agent coco --copy")
		return DoctorResult{Text: b.String(), Err: nil}
	case "linux":
		wayland := os.Getenv("WAYLAND_DISPLAY") != ""
		x11 := os.Getenv("DISPLAY") != ""
		if wayland {
			missing := []string{}
			if !lookPathOK("wl-paste") {
				missing = append(missing, "wl-paste")
			}
			if !lookPathOK("wl-copy") {
				missing = append(missing, "wl-copy")
			}
			if len(missing) > 0 {
				fmt.Fprintf(&b, "missing: %s\n", strings.Join(missing, "/"))
				fmt.Fprintln(&b, "hint: sudo apt install wl-clipboard")
				return DoctorResult{Text: b.String(), Err: errs.E("E003", "缺少 wl-clipboard 组件")}
			}
			fmt.Fprintln(&b, "ok: wl-clipboard (wl-paste/wl-copy)")
			fmt.Fprintln(&b, "note: Wayland 下全局热键无法通用实现")
			fmt.Fprintln(&b, "fix: 使用桌面环境快捷键绑定到: clip2agent hotkey trigger --id 1")
			return DoctorResult{Text: b.String(), Err: nil}
		}
		if x11 {
			if !lookPathOK("xclip") && !lookPathOK("xsel") {
				fmt.Fprintln(&b, "missing: xclip/xsel")
				fmt.Fprintln(&b, "hint: sudo apt install xclip")
				return DoctorResult{Text: b.String(), Err: errs.E("E003", "缺少 xclip/xsel")}
			}
			fmt.Fprintf(&b, "ok: xclip=%v xsel=%v\n", lookPathOK("xclip"), lookPathOK("xsel"))
			if !lookPathOK("xbindkeys") {
				fmt.Fprintln(&b, "note: xbindkeys missing (global hotkey helper)")
				fmt.Fprintln(&b, "fix: sudo apt install xbindkeys")
				fmt.Fprintln(&b, "fix: clip2agent hotkey install")
			}
			return DoctorResult{Text: b.String(), Err: nil}
		}
		fmt.Fprintln(&b, "note: 未检测到 WAYLAND_DISPLAY/DISPLAY，无法确定会话类型")
		fmt.Fprintf(&b, "tools: wl-paste=%v xclip=%v xsel=%v\n", lookPathOK("wl-paste"), lookPathOK("xclip"), lookPathOK("xsel"))
		if !lookPathOK("wl-paste") && !lookPathOK("xclip") && !lookPathOK("xsel") {
			fmt.Fprintln(&b, "hint: Wayland 安装 wl-clipboard；X11 安装 xclip")
			return DoctorResult{Text: b.String(), Err: errs.E("E003", "缺少 Linux 剪贴板工具")}
		}
		return DoctorResult{Text: b.String(), Err: nil}
	case "windows":
		psOK := lookPathOK("powershell") || lookPathOK("powershell.exe") || lookPathOK("pwsh") || lookPathOK("pwsh.exe")
		if !psOK {
			fmt.Fprintln(&b, "missing: powershell")
			fmt.Fprintln(&b, "fix: 安装/启用 Windows PowerShell 或 PowerShell 7 (pwsh)")
			return DoctorResult{Text: b.String(), Err: errs.E("E003", "缺少 PowerShell")}
		}
		fmt.Fprintln(&b, "ok: PowerShell")
		if _, err := os.Stat(paths.HotkeyConfigPath()); err != nil {
			fmt.Fprintln(&b, "note: hotkey config missing")
			fmt.Fprintln(&b, "fix: clip2agent config init")
		} else {
			if err := validateHotkeyConfig(paths.HotkeyConfigPath()); err != nil {
				fmt.Fprintln(&b, "note: hotkey config invalid")
				errs.Fprint(&b, err)
				fmt.Fprintln(&b)
				fmt.Fprintln(&b, "fix: clip2agent config init --force")
			}
		}
		fmt.Fprintln(&b, "note: Windows 全局热键通过 `clip2agent hotkey run` 常驻实现")
		fmt.Fprintln(&b, "next: clip2agent hotkey run")
		return DoctorResult{Text: b.String(), Err: nil}
	default:
		fmt.Fprintln(&b, "note: 当前平台暂不支持")
		return DoctorResult{Text: b.String(), Err: errs.E("E006", "当前平台暂不支持")}
	}
}

func DoctorJSON(ctx context.Context, opt DoctorOptions) DoctorReport {
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if opt.GOARCH == "" {
		opt.GOARCH = runtime.GOARCH
	}
	rep := DoctorReport{
		SchemaVersion: 1,
		Timestamp:     time.Now().Format(time.RFC3339),
		Platform:      fmt.Sprintf("%s/%s", opt.GOOS, opt.GOARCH),
		GOOS:          opt.GOOS,
		GOARCH:        opt.GOARCH,
		Checks:        map[string]bool{},
		Paths:         map[string]string{},
	}
	rep.Next = append(rep.Next, "clip2agent inspect", "clip2agent coco --copy")

	// clip2agent 是否在 PATH（不阻塞，仅提示）
	if !lookPathOK("clip2agent") {
		rep.Notes = append(rep.Notes, "clip2agent not found in PATH")
		rep.Fix = append(rep.Fix, "(repo) go install ./cmd/clip2agent")
		rep.Fix = append(rep.Fix, "确保 PATH 包含安装目录（如 ~/.local/bin）")
		if opt.GOOS == "darwin" {
			rep.Fix = append(rep.Fix, "clip2agent setup")
		}
	}

	// temp dir 可写性
	if err := checkTempWritable(); err != nil {
		rep.Error = errToJSON(errs.WrapCode("E007", "临时目录不可写", err))
		return rep
	}

	switch opt.GOOS {
	case "darwin":
		rep.Checks["macos_helper_in_path"] = lookPathOK("clip2agent-macos")
		rep.Checks["macos_helper_env"] = strings.TrimSpace(os.Getenv("CLIP2AGENT_MACOS_HELPER")) != ""
		rep.Paths["hotkey_config"] = paths.HotkeyConfigPath()
		rep.Paths["hotkey_launchagent"] = paths.HotkeyLaunchAgentPlistPath()
		if !rep.Checks["macos_helper_in_path"] && !rep.Checks["macos_helper_env"] {
			rep.Missing = append(rep.Missing, "clip2agent-macos")
			rep.Notes = append(rep.Notes, "`setup --verify` 更适合安装后验证；如果你刚完成清理，这是待重装状态")
			addDarwinResetAdvice(&rep.Fix, &rep.Next)
			rep.Fix = append(rep.Fix, "clip2agent setup", "(repo) go run ./cmd/clip2agent setup")
			rep.Error = errToJSON(errs.E("E003", "缺少 macOS 剪贴板 helper（clip2agent-macos）"))
			return rep
		}
		rep.Checks["macos_helper"] = true
		rep.Notes = append(rep.Notes, "auto-paste 需要 macOS 辅助功能(Accessibility) 权限（由 clip2agent-hotkey 菜单栏引导）")
		cfgPath := rep.Paths["hotkey_config"]
		if _, err := os.Stat(cfgPath); err != nil {
			rep.Notes = append(rep.Notes, "hotkey config missing")
			rep.Fix = append(rep.Fix, "clip2agent config init")
			addDarwinResetAdvice(&rep.Fix, &rep.Next)
			rep.Checks["hotkey_config_exists"] = false
		} else {
			rep.Checks["hotkey_config_exists"] = true
			if err := validateHotkeyConfig(cfgPath); err != nil {
				rep.Notes = append(rep.Notes, "hotkey config invalid")
				rep.Fix = append(rep.Fix, "clip2agent config init --force")
				addDarwinResetAdvice(&rep.Fix, &rep.Next)
				rep.Checks["hotkey_config_valid"] = false
			} else {
				rep.Checks["hotkey_config_valid"] = true
			}
		}
		if rep.Paths["hotkey_launchagent"] != "" {
			if _, err := os.Stat(rep.Paths["hotkey_launchagent"]); err != nil {
				rep.Notes = append(rep.Notes, "hotkey LaunchAgent not installed")
				rep.Fix = append(rep.Fix, "clip2agent hotkey install")
				addDarwinResetAdvice(&rep.Fix, &rep.Next)
				rep.Checks["hotkey_launchagent_installed"] = false
			} else {
				rep.Checks["hotkey_launchagent_installed"] = true
				uid := os.Getuid()
				svc := "gui/" + strconv.Itoa(uid) + "/" + paths.HotkeyLaunchAgentLabel
				ok, _ := checkLaunchctlService(ctx, svc)
				rep.Checks["hotkey_service_running"] = ok
				if !ok {
					rep.Fix = append(rep.Fix, "clip2agent hotkey restart")
					addDarwinResetAdvice(&rep.Fix, &rep.Next)
				}
			}
		}
		rep.Next = appendUniqueStrings(rep.Next, "clip2agent inspect", "clip2agent coco --copy")
		return rep
	case "linux":
		wayland := os.Getenv("WAYLAND_DISPLAY") != ""
		x11 := os.Getenv("DISPLAY") != ""
		rep.Checks["wayland"] = wayland
		rep.Checks["x11"] = x11
		rep.Paths["hotkey_config"] = paths.HotkeyConfigPath()
		if wayland {
			rep.Checks["wl-paste"] = lookPathOK("wl-paste")
			rep.Checks["wl-copy"] = lookPathOK("wl-copy")
			missing := []string{}
			if !rep.Checks["wl-paste"] {
				missing = append(missing, "wl-paste")
			}
			if !rep.Checks["wl-copy"] {
				missing = append(missing, "wl-copy")
			}
			if len(missing) > 0 {
				rep.Missing = append(rep.Missing, missing...)
				rep.Fix = append(rep.Fix, "sudo apt install wl-clipboard")
				rep.Error = errToJSON(errs.E("E003", "缺少 wl-clipboard 组件"))
				return rep
			}
			rep.Notes = append(rep.Notes, "Wayland 下全局热键无法通用实现")
			rep.Fix = append(rep.Fix, "使用桌面环境快捷键绑定到: clip2agent hotkey trigger --id 1")
			return rep
		}
		if x11 {
			rep.Checks["xclip"] = lookPathOK("xclip")
			rep.Checks["xsel"] = lookPathOK("xsel")
			if !rep.Checks["xclip"] && !rep.Checks["xsel"] {
				rep.Missing = append(rep.Missing, "xclip/xsel")
				rep.Fix = append(rep.Fix, "sudo apt install xclip")
				rep.Error = errToJSON(errs.E("E003", "缺少 xclip/xsel"))
				return rep
			}
			rep.Checks["xbindkeys"] = lookPathOK("xbindkeys")
			if !rep.Checks["xbindkeys"] {
				rep.Notes = append(rep.Notes, "xbindkeys missing (global hotkey helper)")
				rep.Fix = append(rep.Fix, "sudo apt install xbindkeys", "clip2agent hotkey install")
			}
			return rep
		}
		rep.Notes = append(rep.Notes, "未检测到 WAYLAND_DISPLAY/DISPLAY，无法确定会话类型")
		rep.Checks["wl-paste"] = lookPathOK("wl-paste")
		rep.Checks["xclip"] = lookPathOK("xclip")
		rep.Checks["xsel"] = lookPathOK("xsel")
		if !rep.Checks["wl-paste"] && !rep.Checks["xclip"] && !rep.Checks["xsel"] {
			rep.Fix = append(rep.Fix, "Wayland 安装 wl-clipboard；X11 安装 xclip")
			rep.Error = errToJSON(errs.E("E003", "缺少 Linux 剪贴板工具"))
			return rep
		}
		return rep
	case "windows":
		psOK := lookPathOK("powershell") || lookPathOK("powershell.exe") || lookPathOK("pwsh") || lookPathOK("pwsh.exe")
		rep.Checks["powershell"] = psOK
		rep.Paths["hotkey_config"] = paths.HotkeyConfigPath()
		if !psOK {
			rep.Missing = append(rep.Missing, "powershell")
			rep.Fix = append(rep.Fix, "安装/启用 Windows PowerShell 或 PowerShell 7 (pwsh)")
			rep.Error = errToJSON(errs.E("E003", "缺少 PowerShell"))
			return rep
		}
		if _, err := os.Stat(rep.Paths["hotkey_config"]); err != nil {
			rep.Notes = append(rep.Notes, "hotkey config missing")
			rep.Fix = append(rep.Fix, "clip2agent config init")
			rep.Checks["hotkey_config_exists"] = false
		} else {
			rep.Checks["hotkey_config_exists"] = true
			if err := validateHotkeyConfig(rep.Paths["hotkey_config"]); err != nil {
				rep.Notes = append(rep.Notes, "hotkey config invalid")
				rep.Fix = append(rep.Fix, "clip2agent config init --force")
				rep.Checks["hotkey_config_valid"] = false
			} else {
				rep.Checks["hotkey_config_valid"] = true
			}
		}
		rep.Notes = append(rep.Notes, "Windows 全局热键通过 `clip2agent hotkey run` 常驻实现")
		rep.Next = append(rep.Next, "clip2agent hotkey run")
		return rep
	default:
		rep.Error = errToJSON(errs.E("E006", "当前平台暂不支持"))
		return rep
	}
}

func errToJSON(err error) *ErrorJSON {
	if err == nil {
		return nil
	}
	var ce *errs.ClipError
	if errors.As(err, &ce) {
		out := &ErrorJSON{Code: ce.Code, Message: ce.Message, Hint: ce.Hint}
		if ce.Cause != nil {
			out.Cause = ce.Cause.Error()
		}
		return out
	}
	return &ErrorJSON{Message: err.Error()}
}

func MarshalReport(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

type hotkeyConfigFile struct {
	SchemaVersion int `json:"schema_version,omitempty"`
	Bindings      []struct {
		Enabled  *bool  `json:"enabled,omitempty"`
		Shortcut string `json:"shortcut"`
		Action   struct {
			Type    string            `json:"type"`
			Args    []string          `json:"args,omitempty"`
			Command string            `json:"command,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
			Post    any               `json:"post,omitempty"`
		} `json:"action"`
	} `json:"bindings"`
}

func validateHotkeyConfig(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return errs.WrapCode("E003", "读取 hotkey 配置失败", err)
	}
	var cfg hotkeyConfigFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		return errs.E("E006", "hotkey 配置 JSON 解析失败", errs.Cause(err))
	}
	if cfg.SchemaVersion != 0 && cfg.SchemaVersion != 1 {
		return errs.E("E006", "不支持的 hotkey 配置 schema_version", errs.Hint("请运行: clip2agent config init --force"))
	}
	if len(cfg.Bindings) == 0 {
		return errs.E("E006", "hotkey bindings 为空", errs.Hint("运行: clip2agent config init"))
	}
	for i, bd := range cfg.Bindings {
		if bd.Enabled != nil && *bd.Enabled == false {
			continue
		}
		if strings.TrimSpace(bd.Shortcut) == "" {
			return errs.E("E006", fmt.Sprintf("binding #%d shortcut 为空", i+1))
		}
		t := strings.ToLower(strings.TrimSpace(bd.Action.Type))
		switch t {
		case "clip2agent":
			if len(bd.Action.Args) == 0 {
				return errs.E("E006", fmt.Sprintf("binding #%d action.args 为空", i+1), errs.Hint("示例: {type:'clip2agent', args:['coco','--copy']}"))
			}
		case "exec":
			if strings.TrimSpace(bd.Action.Command) == "" {
				return errs.E("E006", fmt.Sprintf("binding #%d action.command 为空", i+1))
			}
		default:
			return errs.E("E006", fmt.Sprintf("binding #%d 不支持的 action.type: %s", i+1, bd.Action.Type), errs.Hint("支持: clip2agent/exec"))
		}
	}
	return nil
}

func checkLaunchctlService(ctx context.Context, service string) (bool, string) {
	if strings.TrimSpace(service) == "" {
		return false, ""
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		return false, ""
	}
	out, err := exec.CommandContext(ctx, "launchctl", "print", service).CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	return true, strings.TrimSpace(string(out))
}

func lookPathOK(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func checkTempWritable() error {
	f, err := os.CreateTemp("", "clip2agent-writecheck-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func pathContainsDir(pathEnv string, dir string) bool {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" {
		return false
	}
	for _, p := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if filepath.Clean(strings.TrimSpace(p)) == dir {
			return true
		}
	}
	return false
}

func countEnabledBindings(configPath string) int {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return 0
	}
	var cfg hotkeyConfigFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		return 0
	}
	n := 0
	for _, bd := range cfg.Bindings {
		if bd.Enabled != nil && *bd.Enabled == false {
			continue
		}
		n++
	}
	return n
}

// config/hotkey/launchagent 路径统一在 internal/paths 中维护。
