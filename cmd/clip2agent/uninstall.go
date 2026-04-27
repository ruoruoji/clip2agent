package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ruoruoji/clip2agent/internal/errs"
	"github.com/ruoruoji/clip2agent/internal/paths"
)

type uninstallOptions struct {
	binDir string

	dryRun  bool
	verbose bool

	yes   bool
	purge bool

	noHotkey bool
	noBin    bool
	noConfig bool
	noTemp   bool
	noLogs   bool
}

// 用于 Windows：删除失败时转为“已安排后台删除”。
// 该错误不应当让卸载整体失败。
type uninstallScheduled struct{ msg string }

func (e uninstallScheduled) Error() string { return e.msg }

type uninstallLogger struct {
	op     uninstallOptions
	failed bool
}

func (l *uninstallLogger) planf(format string, args ...any) {
	fmt.Printf("plan: "+format+"\n", args...)
}

func (l *uninstallLogger) okf(format string, args ...any) {
	if !l.op.verbose {
		return
	}
	fmt.Printf("ok: "+format+"\n", args...)
}

func (l *uninstallLogger) skipf(format string, args ...any) {
	if !l.op.verbose {
		return
	}
	fmt.Printf("skip: "+format+"\n", args...)
}

func (l *uninstallLogger) warnf(format string, args ...any) {
	// 默认不刷屏：仅 verbose 才输出。
	if !l.op.verbose {
		return
	}
	fmt.Printf("warn: "+format+"\n", args...)
}

func (l *uninstallLogger) fail(err error) {
	if err == nil {
		return
	}
	l.failed = true
	err2 := errs.WrapCode("E007", "卸载失败", err)
	errs.Fprint(os.Stderr, err2)
	fmt.Fprintln(os.Stderr)
}

func (l *uninstallLogger) removeFile(path, desc string) {
	p := strings.TrimSpace(path)
	if p == "" {
		l.skipf("%s（空路径）", desc)
		return
	}
	if l.op.dryRun {
		l.planf("%s: %s", desc, p)
		return
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			l.skipf("%s（不存在）: %s", desc, p)
			return
		}
		l.fail(errs.WrapCode("E003", desc+"（stat 失败）", err))
		return
	}
	err := uninstallRemoveFile(p)
	if err != nil {
		var sch uninstallScheduled
		if errors.As(err, &sch) {
			l.warnf("%s（已安排稍后删除）: %s", desc, p)
			return
		}
		if os.IsNotExist(err) {
			l.skipf("%s（不存在）: %s", desc, p)
			return
		}
		l.fail(errs.WrapCode("E007", desc, err))
		return
	}
	l.okf("%s: %s", desc, p)
}

func (l *uninstallLogger) removeAll(path, desc string) {
	p := strings.TrimSpace(path)
	if p == "" {
		l.skipf("%s（空路径）", desc)
		return
	}
	if l.op.dryRun {
		l.planf("%s: %s", desc, p)
		return
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			l.skipf("%s（不存在）: %s", desc, p)
			return
		}
		l.fail(errs.WrapCode("E003", desc+"（stat 失败）", err))
		return
	}
	err := uninstallRemoveAll(p)
	if err != nil {
		var sch uninstallScheduled
		if errors.As(err, &sch) {
			l.warnf("%s（已安排稍后删除）: %s", desc, p)
			return
		}
		l.fail(errs.WrapCode("E007", desc, err))
		return
	}
	l.okf("%s: %s", desc, p)
}

func runUninstall(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var op uninstallOptions
	fs.StringVar(&op.binDir, "bin-dir", "", "二进制所在目录（默认取当前可执行文件目录）")
	fs.BoolVar(&op.dryRun, "dry-run", false, "只打印将执行的动作，不实际删除；适合先预览本地开发重置的清理范围")
	fs.BoolVar(&op.verbose, "verbose", false, "输出每项处理结果")

	fs.BoolVar(&op.yes, "yes", false, "跳过危险操作确认（用于脚本）")
	fs.BoolVar(&op.purge, "purge", false, "彻底清理：删除 kept-dir（高风险，需要 --yes）")

	fs.BoolVar(&op.noHotkey, "no-hotkey", false, "不卸载热键相关（服务/LaunchAgent/xbindkeys 等）")
	fs.BoolVar(&op.noBin, "no-bin", false, "不删除 bin-dir 下二进制")
	fs.BoolVar(&op.noConfig, "no-config", false, "不删除 hotkey 配置（hotkey.json）")
	fs.BoolVar(&op.noTemp, "no-temp", false, "不删除默认临时目录（os.TempDir()/clip2agent）")
	fs.BoolVar(&op.noLogs, "no-logs", false, "不删除运行日志（CLI / hotkey）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if op.purge && !op.yes {
		errs.Fprint(os.Stderr, errs.E("E006", "危险操作需要确认", errs.Hint("要删除 kept-dir，请使用: clip2agent uninstall --purge --yes")))
		fmt.Fprintln(os.Stderr)
		return 2
	}

	if strings.TrimSpace(op.binDir) == "" {
		op.binDir = defaultUninstallBinDir()
	}
	op.binDir = filepath.Clean(op.binDir)

	log := &uninstallLogger{op: op}

	if !op.noHotkey {
		uninstallPlatformHotkey(ctx, op, log)
	}
	if !op.noLogs {
		uninstallPlatformLogs(ctx, op, log)
	}
	if !op.noConfig {
		log.removeFile(paths.HotkeyConfigPath(), "删除 hotkey 配置")
	}
	if !op.noTemp {
		defTemp := filepath.Join(os.TempDir(), "clip2agent")
		// 仅允许删除默认 temp-dir，避免误删。
		if filepath.Clean(defTemp) != filepath.Clean(filepath.Join(os.TempDir(), "clip2agent")) {
			// 理论不会发生；保守起见直接跳过。
			log.warnf("默认临时目录异常，跳过清理: %s", defTemp)
		} else {
			log.removeAll(defTemp, "删除默认临时目录")
		}
	}
	if op.purge {
		// 仅允许删除 DefaultKeptDir。
		log.removeAll(paths.DefaultKeptDir(), "删除 kept-dir（purge）")
	}

	if !op.noBin {
		bins := []string{"clip2agent", "clip2agent-macos", "clip2agent-hotkey"}
		for _, name := range bins {
			p := filepath.Join(op.binDir, name)
			log.removeFile(p, "删除二进制")
		}
		if runtime.GOOS == "windows" {
			// Windows 上经常需要关闭正在运行的 hotkey/run 后才能删除。
			log.warnf("Windows 提示：如文件被占用，请先退出正在运行的 clip2agent/hotkey，再重试或重启后生效")
		}
	}

	if op.dryRun {
		printUninstallNextSteps(op, true)
		return 0
	}
	if log.failed {
		return 1
	}
	if !op.verbose {
		fmt.Println("ok")
	}
	printUninstallNextSteps(op, false)
	return 0
}

func printUninstallNextSteps(op uninstallOptions, dryRun bool) {
	if dryRun {
		fmt.Println("next:")
		fmt.Println("- 确认上面的清理范围符合预期")
		if op.purge {
			fmt.Println("- 当前命令已覆盖本地开发重置所需的彻底清理范围")
		} else {
			fmt.Println("- 需要做本地开发重置时，运行: clip2agent uninstall --purge --yes")
			fmt.Println("- 若在仓库中开发，也可运行: go run ./cmd/clip2agent uninstall --purge --yes")
		}
		fmt.Println("- 实际清理完成后运行: clip2agent doctor")
		fmt.Println("- 若在仓库中开发，也可运行: go run ./cmd/clip2agent doctor")
		if runtime.GOOS == "darwin" {
			fmt.Println("- macOS 重新安装后验证: clip2agent setup && clip2agent setup --verify")
			fmt.Println("- 若在仓库中开发，也可运行: go run ./cmd/clip2agent setup && go run ./cmd/clip2agent setup --verify")
		}
		return
	}

	fmt.Println("next:")
	if op.purge {
		fmt.Println("- 已完成本地开发重置的清理阶段")
	} else {
		fmt.Println("- 如需彻底清理当前环境，再运行: clip2agent uninstall --purge --yes")
		fmt.Println("- 若在仓库中开发，也可运行: go run ./cmd/clip2agent uninstall --purge --yes")
	}
	fmt.Println("- 清理后检查环境: clip2agent doctor")
	fmt.Println("- 若在仓库中开发，也可运行: go run ./cmd/clip2agent doctor")
	if runtime.GOOS == "darwin" {
		fmt.Println("- macOS 重新 setup: clip2agent setup")
		fmt.Println("- 安装后验证: clip2agent setup --verify")
		fmt.Println("- 若在仓库中开发，也可运行: go run ./cmd/clip2agent setup && go run ./cmd/clip2agent setup --verify")
	}
}

func defaultUninstallBinDir() string {
	if exe, err := os.Executable(); err == nil {
		exe = strings.TrimSpace(exe)
		if exe != "" {
			return filepath.Dir(exe)
		}
	}
	h, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(h) != "" {
		return filepath.Join(h, ".local", "bin")
	}
	return "."
}
