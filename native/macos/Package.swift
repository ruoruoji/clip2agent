// swift-tools-version: 5.9
import PackageDescription

let package = Package(
  name: "clip2agent-macos",
  platforms: [
    .macOS(.v12)
  ],
  products: [
    .executable(name: "clip2agent-macos", targets: ["clip2agent-macos"]),
    .executable(name: "clip2agent-hotkey", targets: ["clip2agent-hotkey"])
  ],
  targets: [
    .executableTarget(
      name: "clip2agent-macos",
      path: "Sources"
    ),
    .executableTarget(
      name: "clip2agent-hotkey",
      path: "SourcesHotkey"
    )
  ]
)
