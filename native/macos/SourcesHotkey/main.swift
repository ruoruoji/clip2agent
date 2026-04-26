import AppKit
import ApplicationServices
import Carbon
import Darwin
import Foundation

// macOS 全局热键 daemon：读取配置文件注册热键，触发后执行配置的命令。
// 设计目标：常驻 + 菜单栏入口 + 动作可配置（默认执行 clip2agent 并用 --copy 写回剪贴板）；可选自动粘贴（需要辅助功能权限）。

// MARK: - Config

struct HotkeyConfigFile: Codable {
  struct Binding: Codable {
    // 可选：用于更稳定的标识与菜单栏展示
    let id: String?
    let name: String?
    let enabled: Bool?

    let shortcut: String
    let action: Action

    var displayName: String {
      if let name, !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
        return name
      }
      if let id, !id.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
        return id
      }
      return shortcut
    }
  }

  struct Action: Codable {
    struct Post: Codable {
      struct Paste: Codable {
        let enabled: Bool?
        let delayMs: Int?

        enum CodingKeys: String, CodingKey {
          case enabled
          case delayMs = "delay_ms"
        }
      }

      let paste: Paste?
    }

    // type: "clip2agent" | "exec"
    let type: String
    let args: [String]?
    let command: String?
    let env: [String: String]?
    let post: Post?
  }

  let bindings: [Binding]
}

// MARK: - Shortcut parsing

private let modifierOrder = ["control", "option", "alt", "shift", "command", "cmd"]

private func parseShortcut(_ raw: String) throws -> (keyCode: UInt32, modifiers: UInt32) {
  let lowered = raw
    .trimmingCharacters(in: .whitespacesAndNewlines)
    .lowercased()
  if lowered.isEmpty {
    throw NSError(domain: "clip2agent-hotkey", code: 1, userInfo: [NSLocalizedDescriptionKey: "shortcut 不能为空"]) 
  }

  let tokens = lowered.split(separator: "+").map { String($0) }.filter { !$0.isEmpty }
  if tokens.isEmpty {
    throw NSError(domain: "clip2agent-hotkey", code: 1, userInfo: [NSLocalizedDescriptionKey: "shortcut 解析失败: \(raw)"]) 
  }

  var mods = Set<String>()
  var key: String? = nil
  for t in tokens {
    if modifierOrder.contains(t) {
      mods.insert(t)
    } else {
      // 最后一个非 modifier token 作为 key
      key = t
    }
  }

  guard let keyToken = key else {
    throw NSError(domain: "clip2agent-hotkey", code: 1, userInfo: [NSLocalizedDescriptionKey: "shortcut 缺少 key: \(raw)"]) 
  }

  guard let keyCode = keyCodeMap[keyToken] else {
    throw NSError(domain: "clip2agent-hotkey", code: 1, userInfo: [NSLocalizedDescriptionKey: "不支持的 key: \(keyToken)"]) 
  }

  var modifierFlags: UInt32 = 0
  if mods.contains("control") {
    modifierFlags |= UInt32(controlKey)
  }
  if mods.contains("option") || mods.contains("alt") {
    modifierFlags |= UInt32(optionKey)
  }
  if mods.contains("shift") {
    modifierFlags |= UInt32(shiftKey)
  }
  if mods.contains("command") || mods.contains("cmd") {
    modifierFlags |= UInt32(cmdKey)
  }

  return (keyCode, modifierFlags)
}

// 常用 keycode 映射（US 键盘布局）。
// 参考：HIToolbox/Events.h；尽量覆盖 clip2agent 常见热键需求。
private let keyCodeMap: [String: UInt32] = [
  // letters
  "a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9,
  "b": 11, "q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17,
  "1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23, "=": 24, "9": 25, "7": 26, "-": 27,
  "8": 28, "0": 29, "]": 30, "o": 31, "u": 32, "[": 33, "i": 34, "p": 35,
  "l": 37, "j": 38, "'": 39, "k": 40, ";": 41, "\\": 42, ",": 43, "/": 44, "n": 45, "m": 46, ".": 47,
  // special
  "space": 49,
  "return": 36,
  "enter": 76,
  "tab": 48,
  "escape": 53,
  "esc": 53,
  "delete": 51,
  "backspace": 51,
]

// MARK: - Runtime

final class HotkeyDaemon: NSObject, NSApplicationDelegate {
  private var hotKeyRefs: [UInt32: EventHotKeyRef?] = [:]
  private var bindingsByID: [UInt32: HotkeyConfigFile.Binding] = [:]
  private var allBindings: [HotkeyConfigFile.Binding] = []
  private var lastTriggerAt: Date = .distantPast

  private var statusItem: NSStatusItem?
  private let actionQueue = DispatchQueue(label: "clip2agent-hotkey.action", qos: .userInitiated)
  private var eventHandlerInstalled = false
  private var lastErrorSummary: String = ""
  private var lastActionSummary: String = ""

  private struct InvalidBinding {
    let displayName: String
    let shortcut: String
    let reason: String
  }

  private var invalidBindings: [InvalidBinding] = []
  private var lastLoadRegistered: Int = 0
  private var lastLoadEnabled: Int = 0

  private var configWatchFD: Int32 = -1
  private var configWatchSource: DispatchSourceFileSystemObject?
  private var reloadWorkItem: DispatchWorkItem?

  func applicationDidFinishLaunching(_ notification: Notification) {
    // 菜单栏常驻，不显示 Dock 图标。
    NSApp.setActivationPolicy(.accessory)

    setupStatusItem()
    startConfigWatcher()
    reloadBindingsAndMenu(firstLaunch: true)
    log("started")
  }

  private func setupStatusItem() {
    if statusItem != nil { return }
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    item.button?.title = "C2A"
    item.button?.toolTip = "clip2agent-hotkey"
    item.menu = NSMenu()
    statusItem = item
    rebuildMenu()
  }

  private func setStatus(_ title: String, tooltip: String? = nil) {
    statusItem?.button?.title = title
    if let tooltip {
      statusItem?.button?.toolTip = tooltip
    }
  }

  private func rebuildMenu() {
    guard let menu = statusItem?.menu else { return }
    menu.removeAllItems()

    let statusTitle: String
    if !lastErrorSummary.isEmpty {
      statusTitle = "状态：异常（点击查看日志）"
    } else if allBindings.isEmpty {
      statusTitle = "状态：未加载配置"
    } else {
      if lastLoadEnabled > 0 {
        statusTitle = "状态：运行中（已注册 \(lastLoadRegistered)/\(lastLoadEnabled)）"
      } else {
        statusTitle = "状态：运行中（无启用快捷键）"
      }
    }
    let statusItem = NSMenuItem(title: statusTitle, action: #selector(openLogFromMenu), keyEquivalent: "")
    statusItem.target = self
    menu.addItem(statusItem)

    if !lastActionSummary.isEmpty {
      let lastItem = NSMenuItem(title: "上次：\(lastActionSummary)", action: nil, keyEquivalent: "")
      lastItem.isEnabled = false
      menu.addItem(lastItem)
    }
    if !lastErrorSummary.isEmpty {
      let errItem = NSMenuItem(title: "错误：\(lastErrorSummary)", action: nil, keyEquivalent: "")
      errItem.isEnabled = false
      menu.addItem(errItem)
    }

    if !invalidBindings.isEmpty {
      let sub = NSMenu()
      for ib in invalidBindings.prefix(8) {
        let t = "\(ib.displayName)（\(ib.shortcut)）：\(ib.reason)"
        let it = NSMenuItem(title: t, action: nil, keyEquivalent: "")
        it.isEnabled = false
        sub.addItem(it)
      }
      if invalidBindings.count > 8 {
        let more = NSMenuItem(title: "… 还有 \(invalidBindings.count - 8) 条", action: nil, keyEquivalent: "")
        more.isEnabled = false
        sub.addItem(more)
      }
      let invalidItem = NSMenuItem(title: "配置错误（\(invalidBindings.count)）", action: nil, keyEquivalent: "")
      invalidItem.submenu = sub
      menu.addItem(invalidItem)
    }

    menu.addItem(NSMenuItem.separator())

    if !allBindings.isEmpty {
      for (idx, b) in allBindings.enumerated() {
        let enabled = b.enabled ?? true
        let title = "触发：\(b.displayName)（\(b.shortcut)）"
        let it = NSMenuItem(title: title, action: #selector(triggerBindingFromMenu(_:)), keyEquivalent: "")
        it.target = self
        it.representedObject = NSNumber(value: idx)
        it.isEnabled = enabled
        menu.addItem(it)
      }
      menu.addItem(NSMenuItem.separator())
    }

    let reloadItem = NSMenuItem(title: "重新加载配置", action: #selector(reloadFromMenu), keyEquivalent: "r")
    reloadItem.target = self
    menu.addItem(reloadItem)

    let openConfig = NSMenuItem(title: "打开配置文件（hotkey.json）", action: #selector(openConfigFromMenu), keyEquivalent: "")
    openConfig.target = self
    menu.addItem(openConfig)

    let fixSub = NSMenu()
    let cfgURL = configURL()
    let cfgExists = FileManager.default.fileExists(atPath: cfgURL.path)

    if !cfgExists {
      let it = NSMenuItem(title: "生成默认配置（clip2agent config init）", action: #selector(fixInitConfigFromMenu), keyEquivalent: "")
      it.target = self
      fixSub.addItem(it)
    }

    let resetIt = NSMenuItem(title: "强制重置配置（备份并重建）…", action: #selector(fixResetConfigFromMenu), keyEquivalent: "")
    resetIt.target = self
    fixSub.addItem(resetIt)

    let openDir = NSMenuItem(title: "打开配置目录", action: #selector(openConfigDirFromMenu), keyEquivalent: "")
    openDir.target = self
    fixSub.addItem(openDir)

    let copyPath = NSMenuItem(title: "复制配置路径", action: #selector(copyConfigPathFromMenu), keyEquivalent: "")
    copyPath.target = self
    fixSub.addItem(copyPath)

    fixSub.addItem(NSMenuItem.separator())

    let verify = NSMenuItem(title: "运行：clip2agent setup --verify", action: #selector(runSetupVerifyFromMenu), keyEquivalent: "")
    verify.target = self
    fixSub.addItem(verify)

    let fixItem = NSMenuItem(title: "Fix-it", action: nil, keyEquivalent: "")
    fixItem.submenu = fixSub
    menu.addItem(fixItem)

    menu.addItem(NSMenuItem.separator())

    // 辅助功能（自动粘贴需要）
    let axGranted = isAccessibilityTrusted()
    let axTitle = axGranted ? "辅助功能权限：已授权" : "辅助功能权限：未授权（自动粘贴需要）"
    let axItem = NSMenuItem(title: axTitle, action: #selector(promptAccessibilityFromMenu), keyEquivalent: "")
    axItem.target = self
    axItem.isEnabled = !axGranted
    menu.addItem(axItem)

    let openAX = NSMenuItem(title: "打开系统设置 → 辅助功能…", action: #selector(openAccessibilitySettingsFromMenu), keyEquivalent: "")
    openAX.target = self
    menu.addItem(openAX)

    menu.addItem(NSMenuItem.separator())

    let doctorItem = NSMenuItem(title: "运行：clip2agent doctor", action: #selector(runDoctorFromMenu), keyEquivalent: "")
    doctorItem.target = self
    menu.addItem(doctorItem)

    let hotkeyDoctorItem = NSMenuItem(title: "运行：clip2agent hotkey doctor", action: #selector(runHotkeyDoctorFromMenu), keyEquivalent: "")
    hotkeyDoctorItem.target = self
    menu.addItem(hotkeyDoctorItem)

    let logsItem = NSMenuItem(title: "打开日志（clip2agent-hotkey.log）", action: #selector(openLogFromMenu), keyEquivalent: "")
    logsItem.target = self
    menu.addItem(logsItem)

    menu.addItem(NSMenuItem.separator())

    let quitItem = NSMenuItem(title: "退出", action: #selector(quitFromMenu), keyEquivalent: "q")
    quitItem.target = self
    menu.addItem(quitItem)
  }

  @objc private func reloadFromMenu() {
    reloadBindingsAndMenu(firstLaunch: false)
  }

  @objc private func openConfigFromMenu() {
    NSWorkspace.shared.open(configURL())
  }

  @objc private func openConfigDirFromMenu() {
    NSWorkspace.shared.open(configURL().deletingLastPathComponent())
  }

  @objc private func copyConfigPathFromMenu() {
    let p = configURL().path
    let pb = NSPasteboard.general
    pb.clearContents()
    pb.setString(p, forType: .string)
    DispatchQueue.main.async {
      self.lastActionSummary = "复制配置路径"
      self.lastErrorSummary = ""
      self.rebuildMenu()
    }
  }

  @objc private func fixInitConfigFromMenu() {
    actionQueue.async {
      self.runAndShowOutput(title: "config init", command: "clip2agent", args: ["config", "init"], summary: "config init")
    }
  }

  @objc private func fixResetConfigFromMenu() {
    DispatchQueue.main.async {
      let ok = self.confirm(title: "强制重置配置", message: "将备份当前 hotkey.json，并运行：clip2agent config init --force。\n\n这会覆盖你的配置。")
      if !ok { return }
      self.actionQueue.async {
        self.backupConfigIfExists()
        self.runAndShowOutput(title: "config init --force", command: "clip2agent", args: ["config", "init", "--force"], summary: "config init --force")
      }
    }
  }

  @objc private func runSetupVerifyFromMenu() {
    actionQueue.async {
      self.runAndShowOutput(title: "setup --verify", command: "clip2agent", args: ["setup", "--verify"], summary: "setup --verify")
    }
  }

  @objc private func openLogFromMenu() {
    let url = logURL()
    NSWorkspace.shared.open(url)
  }

  @objc private func runDoctorFromMenu() {
    actionQueue.async {
      self.runAndUpdateSummary(command: "clip2agent", args: ["doctor"], summary: "doctor")
    }
  }

  @objc private func runHotkeyDoctorFromMenu() {
    actionQueue.async {
      self.runAndUpdateSummary(command: "clip2agent", args: ["hotkey", "doctor"], summary: "hotkey doctor")
    }
  }

  @objc private func triggerBindingFromMenu(_ sender: NSMenuItem) {
    guard let n = sender.representedObject as? NSNumber else { return }
    let idx = n.intValue
    if idx < 0 || idx >= allBindings.count { return }
    let binding = allBindings[idx]
    actionQueue.async {
      self.execute(binding)
    }
  }

  @objc private func promptAccessibilityFromMenu() {
    _ = promptAccessibility()
    rebuildMenu()
  }

  @objc private func openAccessibilitySettingsFromMenu() {
    openAccessibilitySettings()
  }

  @objc private func quitFromMenu() {
    NSApp.terminate(nil)
  }

  private func reloadBindingsAndMenu(firstLaunch: Bool) {
    let result = loadConfigAndBuildRegistration()
    if case .failure(let err) = result {
      // 不卸载上一份成功配置：降低“编辑中间态”导致全挂的风险。
      lastErrorSummary = summarize(err)
      setStatus("C2A!", tooltip: "clip2agent-hotkey: \(lastErrorSummary)")
      log("failed to reload config (keeping previous): \(err)")
      rebuildMenu()
      return
    }

    guard case .success(let applied) = result else {
      rebuildMenu()
      return
    }

    // 若本次配置“启用了快捷键但全部注册失败”，视为 reload 失败，保留旧配置。
    if applied.enabledCount > 0 && applied.registeredCount == 0 {
      invalidBindings = applied.invalidBindings
      lastErrorSummary = applied.errorSummary.isEmpty ? "hotkey.json 启用的快捷键均不可用（保留上一次成功配置）" : applied.errorSummary
      setStatus("C2A!", tooltip: "clip2agent-hotkey: \(lastErrorSummary)")
      rebuildMenu()
      return
    }

    // 应用新的注册结果
    unregisterAllHotkeys()
    hotKeyRefs = applied.hotKeyRefs
    bindingsByID = applied.bindingsByID
    allBindings = applied.allBindings
    invalidBindings = applied.invalidBindings
    lastLoadRegistered = applied.registeredCount
    lastLoadEnabled = applied.enabledCount
    lastErrorSummary = applied.errorSummary
    if lastErrorSummary.isEmpty {
      if firstLaunch {
        setStatus("C2A", tooltip: "clip2agent-hotkey: running")
      } else {
        setStatus("C2A", tooltip: "clip2agent-hotkey: reloaded")
      }
    } else {
      setStatus("C2A!", tooltip: "clip2agent-hotkey: \(lastErrorSummary)")
    }
    rebuildMenu()
  }

  private func summarize(_ error: Error) -> String {
    let s = String(describing: error)
    let trimmed = s.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.count <= 120 { return trimmed }
    let idx = trimmed.index(trimmed.startIndex, offsetBy: 120)
    return String(trimmed[..<idx]) + "…"
  }

  private struct RegistrationBuild {
    let allBindings: [HotkeyConfigFile.Binding]
    let hotKeyRefs: [UInt32: EventHotKeyRef?]
    let bindingsByID: [UInt32: HotkeyConfigFile.Binding]
    let invalidBindings: [InvalidBinding]
    let enabledCount: Int
    let registeredCount: Int
    let errorSummary: String
  }

  private func loadConfigAndBuildRegistration() -> Result<RegistrationBuild, Error> {
    let url = configURL()
    do {
      let data = try Data(contentsOf: url)
      let cfg = try JSONDecoder().decode(HotkeyConfigFile.self, from: data)
      if cfg.bindings.isEmpty {
        throw NSError(domain: "clip2agent-hotkey", code: 2, userInfo: [NSLocalizedDescriptionKey: "bindings 为空: \(url.path)"])
      }
      return .success(buildRegistration(cfg: cfg))
    } catch {
      return .failure(error)
    }
  }

  private func buildRegistration(cfg: HotkeyConfigFile) -> RegistrationBuild {
    // 注册事件回调（一次）
    if !eventHandlerInstalled {
      var eventSpec = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
      InstallEventHandler(GetApplicationEventTarget(), { _, eventRef, userData in
        guard let userData else { return noErr }
        let daemon = Unmanaged<HotkeyDaemon>.fromOpaque(userData).takeUnretainedValue()
        daemon.handleHotKey(eventRef: eventRef)
        return noErr
      }, 1, &eventSpec, UnsafeMutableRawPointer(Unmanaged.passUnretained(self).toOpaque()), nil)
      eventHandlerInstalled = true
    }

    var enabledCount = 0
    var hotKeyRefs: [UInt32: EventHotKeyRef?] = [:]
    var bindingsByID: [UInt32: HotkeyConfigFile.Binding] = [:]
    var invalid: [InvalidBinding] = []

    var nextID: UInt32 = 1
    for b in cfg.bindings {
      if (b.enabled ?? true) == false {
        continue
      }
      enabledCount += 1
      do {
        let parsed = try parseShortcut(b.shortcut)
        var hotKeyRef: EventHotKeyRef? = nil
        let hotKeyID = EventHotKeyID(signature: OSType(0x434C4950), id: nextID) // 'CLIP'
        let st = RegisterEventHotKey(parsed.keyCode, parsed.modifiers, hotKeyID, GetApplicationEventTarget(), 0, &hotKeyRef)
        if st != noErr {
          invalid.append(InvalidBinding(displayName: b.displayName, shortcut: b.shortcut, reason: "RegisterEventHotKey failed: \(st)"))
          continue
        }
        hotKeyRefs[nextID] = hotKeyRef
        bindingsByID[nextID] = b
        log("registered: id=\(nextID) shortcut=\(b.shortcut)")
        nextID += 1
      } catch {
        invalid.append(InvalidBinding(displayName: b.displayName, shortcut: b.shortcut, reason: summarize(error)))
        continue
      }
    }

    let registeredCount = bindingsByID.count
    // 如果用户启用了快捷键但全部注册失败：视为“本次 reload 失败”，由上层决定是否保留旧配置。
    var errSummary = ""
    if enabledCount > 0 && registeredCount == 0 {
      errSummary = "hotkey.json 启用的快捷键均不可用（保留上一次成功配置）"
    } else if !invalid.isEmpty {
      errSummary = "hotkey.json 有部分快捷键不可用（\(invalid.count)）"
    }

    return RegistrationBuild(
      allBindings: cfg.bindings,
      hotKeyRefs: hotKeyRefs,
      bindingsByID: bindingsByID,
      invalidBindings: invalid,
      enabledCount: enabledCount,
      registeredCount: registeredCount,
      errorSummary: errSummary
    )
  }

  private func unregisterAllHotkeys() {
    for (_, ref) in hotKeyRefs {
      if let ref {
        UnregisterEventHotKey(ref)
      }
    }
    hotKeyRefs.removeAll()
    bindingsByID.removeAll()
  }

  private func startConfigWatcher() {
    if configWatchSource != nil { return }
    let dirURL = configURL().deletingLastPathComponent()
    let dirPath = dirURL.path
    let fd = open(dirPath, O_EVTONLY)
    if fd < 0 {
      log("config watcher disabled: cannot open dir: \(dirPath)")
      return
    }
    configWatchFD = fd
    let src = DispatchSource.makeFileSystemObjectSource(fileDescriptor: fd, eventMask: [.write, .rename, .delete], queue: actionQueue)
    src.setEventHandler { [weak self] in
      guard let self else { return }
      self.scheduleAutoReload(reason: "fswatch")
    }
    src.setCancelHandler { [weak self] in
      guard let self else { return }
      if self.configWatchFD >= 0 {
        close(self.configWatchFD)
        self.configWatchFD = -1
      }
    }
    src.resume()
    configWatchSource = src
  }

  private func scheduleAutoReload(reason: String) {
    // debounce：编辑器原子写/中间态会产生多次事件。
    reloadWorkItem?.cancel()
    let item = DispatchWorkItem { [weak self] in
      guard let self else { return }
      // 自动 reload 失败时保留上一份成功配置（见 reloadBindingsAndMenu）。
      DispatchQueue.main.async {
        self.reloadBindingsAndMenu(firstLaunch: false)
      }
    }
    reloadWorkItem = item
    actionQueue.asyncAfter(deadline: .now() + 0.35, execute: item)
    log("schedule reload: \(reason)")
  }

  private func handleHotKey(eventRef: EventRef?) {
    // debounce：避免 key repeat 或多次触发
    let now = Date()
    if now.timeIntervalSince(lastTriggerAt) < 0.2 {
      return
    }
    lastTriggerAt = now

    guard let eventRef else { return }
    var hk = EventHotKeyID()
    var actualSize: Int = 0
    let status = GetEventParameter(
      eventRef,
      EventParamName(kEventParamDirectObject),
      EventParamType(typeEventHotKeyID),
      nil,
      Int(MemoryLayout<EventHotKeyID>.size),
      &actualSize,
      &hk
    )
    if status != noErr {
      log("GetEventParameter failed: \(status)")
      return
    }
    let id = hk.id
    guard let binding = bindingsByID[id] else {
      log("unknown hotkey id=\(id)")
      return
    }
    actionQueue.async {
      self.execute(binding)
    }
  }

  private func execute(_ binding: HotkeyConfigFile.Binding) {
    let action = binding.action
    let type = action.type.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    switch type {
    case "clip2agent":
      var args = action.args ?? []
      // 自动粘贴依赖剪贴板，因此确保包含 --copy。
      if shouldAutoPaste(action) && !args.contains("--copy") {
        args.append("--copy")
      }
      runAndMaybePaste(binding: binding, command: resolvedClip2AgentCommand(action), args: args, extraEnv: action.env)
    case "exec":
      guard let cmd = action.command, !cmd.isEmpty else {
        log("exec action missing command")
        return
      }
      runAndMaybePaste(binding: binding, command: cmd, args: action.args ?? [], extraEnv: action.env)
    default:
      log("unknown action type: \(action.type)")
    }
  }

  private func shouldAutoPaste(_ action: HotkeyConfigFile.Action) -> Bool {
    guard let p = action.post?.paste else { return false }
    return p.enabled ?? false
  }

  private func pasteDelayMs(_ action: HotkeyConfigFile.Action) -> Int {
    guard let p = action.post?.paste else { return 80 }
    let ms = p.delayMs ?? 80
    if ms < 0 { return 0 }
    if ms > 5000 { return 5000 }
    return ms
  }

  private func resolvedClip2AgentCommand(_ action: HotkeyConfigFile.Action) -> String {
    if let cmd = action.command, !cmd.isEmpty {
      return cmd
    }
    if let env = ProcessInfo.processInfo.environment["CLIP2AGENT_BIN"], !env.isEmpty {
      return env
    }
    return "clip2agent"
  }

  private func runAndMaybePaste(binding: HotkeyConfigFile.Binding, command: String, args: [String], extraEnv: [String: String]?) {
    let summary = binding.displayName
    DispatchQueue.main.async {
      self.lastActionSummary = summary
      self.rebuildMenu()
    }

    let (ok, errMsg) = runBlocking(command: command, args: args, extraEnv: extraEnv)
    if !ok {
      DispatchQueue.main.async {
        self.lastErrorSummary = errMsg
        self.rebuildMenu()
      }
      return
    }

    if shouldAutoPaste(binding.action) {
      let ms = pasteDelayMs(binding.action)
      if ms > 0 {
        Thread.sleep(forTimeInterval: Double(ms) / 1000.0)
      }

      if isAccessibilityTrusted() {
        pasteCmdV()
      } else {
        // 降级：只 copy，不粘贴。
        DispatchQueue.main.async {
          self.lastErrorSummary = "未授予辅助功能权限：已复制未粘贴"
          self.rebuildMenu()
        }
      }
    }
  }

  private func runAndUpdateSummary(command: String, args: [String], summary: String) {
    let (ok, errMsg) = runBlocking(command: command, args: args, extraEnv: nil)
    DispatchQueue.main.async {
      self.lastActionSummary = summary
      self.lastErrorSummary = ok ? "" : errMsg
      self.rebuildMenu()
    }
  }

  private func runAndShowOutput(title: String, command: String, args: [String], summary: String) {
    let res = runBlockingCapture(command: command, args: args, extraEnv: nil)
    DispatchQueue.main.async {
      self.lastActionSummary = summary
      self.lastErrorSummary = res.ok ? "" : res.errSummary
      self.rebuildMenu()
      let body = res.ok ? res.stdout : (res.stderr.isEmpty ? res.stdout : res.stderr)
      self.showText(title: "\(title) 输出", message: body)
    }
  }

  private func runBlockingCapture(command: String, args: [String], extraEnv: [String: String]?) -> (ok: Bool, stdout: String, stderr: String, errSummary: String) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    p.arguments = [command] + args
    if let extraEnv {
      var env = ProcessInfo.processInfo.environment
      for (k, v) in extraEnv { env[k] = v }
      p.environment = env
    }
    let outPipe = Pipe()
    let errPipe = Pipe()
    p.standardOutput = outPipe
    p.standardError = errPipe
    do {
      try p.run()
      p.waitUntilExit()
      let outData = outPipe.fileHandleForReading.readDataToEndOfFile()
      let errData = errPipe.fileHandleForReading.readDataToEndOfFile()
      let out = (String(data: outData, encoding: .utf8) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
      let err = (String(data: errData, encoding: .utf8) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
      if p.terminationStatus == 0 {
        return (true, out, err, "")
      }
      let sum = err.isEmpty ? "exit=\(p.terminationStatus)" : summarizeString(err)
      log("command failed: \(command) (status=\(p.terminationStatus)) err=\(err)")
      return (false, out, err, sum)
    } catch {
      let s = summarize(error)
      log("exec error: \(s)")
      return (false, "", "", s)
    }
  }

  private func confirm(title: String, message: String) -> Bool {
    let alert = NSAlert()
    alert.messageText = title
    alert.informativeText = message
    alert.addButton(withTitle: "继续")
    alert.addButton(withTitle: "取消")
    return alert.runModal() == .alertFirstButtonReturn
  }

  private func showText(title: String, message: String) {
    let alert = NSAlert()
    alert.messageText = title
    let trimmed = message.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.isEmpty {
      alert.informativeText = "(no output)"
    } else if trimmed.count <= 1200 {
      alert.informativeText = trimmed
    } else {
      let idx = trimmed.index(trimmed.startIndex, offsetBy: 1200)
      alert.informativeText = String(trimmed[..<idx]) + "…"
    }
    alert.addButton(withTitle: "OK")
    alert.runModal()
  }

  private func backupConfigIfExists() {
    let url = configURL()
    let p = url.path
    if !FileManager.default.fileExists(atPath: p) {
      return
    }
    let df = DateFormatter()
    df.locale = Locale(identifier: "en_US_POSIX")
    df.dateFormat = "yyyyMMdd-HHmmss"
    let ts = df.string(from: Date())
    let bak = url.deletingLastPathComponent().appendingPathComponent("hotkey.json.bak-\(ts)")
    do {
      try FileManager.default.copyItem(at: url, to: bak)
      log("backup config: \(bak.path)")
    } catch {
      log("backup config failed: \(error)")
    }
  }

  private func runBlocking(command: String, args: [String], extraEnv: [String: String]?) -> (ok: Bool, errSummary: String) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    p.arguments = [command] + args
    if let extraEnv {
      var env = ProcessInfo.processInfo.environment
      for (k, v) in extraEnv {
        env[k] = v
      }
      p.environment = env
    }

    let outPipe = Pipe()
    let errPipe = Pipe()
    p.standardOutput = outPipe
    p.standardError = errPipe

    do {
      try p.run()
      p.waitUntilExit()
      if p.terminationStatus == 0 {
        return (true, "")
      }
      let data = errPipe.fileHandleForReading.readDataToEndOfFile()
      let msg = (String(data: data, encoding: .utf8) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
      if msg.isEmpty {
        log("command failed: \(command) \(args.joined(separator: " ")) (status=\(p.terminationStatus))")
        return (false, "exit=\(p.terminationStatus)")
      }
      log("command failed: \(command) (status=\(p.terminationStatus)) err=\(msg)")
      return (false, summarizeString(msg))
    } catch {
      let s = summarize(error)
      log("exec error: \(s)")
      return (false, s)
    }
  }

  private func summarizeString(_ s: String) -> String {
    let trimmed = s.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.count <= 140 { return trimmed }
    let idx = trimmed.index(trimmed.startIndex, offsetBy: 140)
    return String(trimmed[..<idx]) + "…"
  }

  private func isAccessibilityTrusted() -> Bool {
    return AXIsProcessTrusted()
  }

  private func promptAccessibility() -> Bool {
    // kAXTrustedCheckOptionPrompt 是 Unmanaged<CFString>，需要取出实际值。
    let key = kAXTrustedCheckOptionPrompt.takeUnretainedValue() as NSString
    let options = [key: true] as CFDictionary
    return AXIsProcessTrustedWithOptions(options)
  }

  private func openAccessibilitySettings() {
    if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility") {
      NSWorkspace.shared.open(url)
      return
    }
    if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security") {
      NSWorkspace.shared.open(url)
    }
  }

  private func pasteCmdV() {
    // US 布局下 'v' 的 virtual keycode = 9（与 keyCodeMap 一致）。
    let keyCode: CGKeyCode = 9
    guard let src = CGEventSource(stateID: .hidSystemState) else { return }
    let flags: CGEventFlags = [.maskCommand]

    if let down = CGEvent(keyboardEventSource: src, virtualKey: keyCode, keyDown: true),
       let up = CGEvent(keyboardEventSource: src, virtualKey: keyCode, keyDown: false)
    {
      down.flags = flags
      up.flags = flags
      down.post(tap: .cghidEventTap)
      up.post(tap: .cghidEventTap)
    }
  }

  private func configURL() -> URL {
    let env = ProcessInfo.processInfo.environment
    let xdg = env["XDG_CONFIG_HOME"]
    let base: URL
    if let xdg, !xdg.isEmpty {
      base = URL(fileURLWithPath: xdg, isDirectory: true)
    } else {
      base = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".config", isDirectory: true)
    }
    return base
      .appendingPathComponent("clip2agent", isDirectory: true)
      .appendingPathComponent("hotkey.json")
  }

  private func logURL() -> URL {
    // 与 Go 侧 `clip2agent hotkey logs` 一致。
    let home = FileManager.default.homeDirectoryForCurrentUser
    return home
      .appendingPathComponent("Library", isDirectory: true)
      .appendingPathComponent("Logs", isDirectory: true)
      .appendingPathComponent("clip2agent-hotkey.log")
  }

  private func log(_ msg: String) {
    fputs("[clip2agent-hotkey] \(msg)\n", stderr)
  }
}

// MARK: - Entry

func usage() -> String {
  return """
clip2agent-hotkey (macOS)

读取 $XDG_CONFIG_HOME/clip2agent/hotkey.json（默认 ~/.config/clip2agent/hotkey.json），注册全局快捷键并执行配置动作。

配置示例：
{
  "bindings": [
    {
      "id": "coco",
      "name": "Coco 引用",
      "enabled": true,
      "shortcut": "control+option+command+v",
      "action": {
        "type": "clip2agent",
        "args": ["path", "--target", "coco", "--copy"],
        "post": {"paste": {"enabled": false, "delay_ms": 80}}
      }
    },
    {
      "shortcut": "control+option+command+b",
      "action": {"type": "clip2agent", "args": ["base64", "--target", "openai", "--json", "--copy"]}
    }
  ]
}

可选环境变量：
- CLIP2AGENT_BIN：指定 clip2agent 可执行文件（默认从 PATH 解析）
\n说明：
- 本程序为菜单栏常驻（无 Dock 图标）。
- 若启用 action.post.paste.enabled=true，将在执行成功后自动发送 Cmd+V（需要在系统设置授予“辅助功能”权限）。
"""
}

if CommandLine.arguments.contains("--help") || CommandLine.arguments.contains("-h") {
  print(usage())
  exit(0)
}

let app = NSApplication.shared
let delegate = HotkeyDaemon()
app.delegate = delegate
app.run()
