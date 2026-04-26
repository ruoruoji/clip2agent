import AppKit
import Foundation

struct HelperError: Codable {
  let code: String
  let message: String
}

struct HelperOK: Codable {
  let mime_type: String
  let width: Int
  let height: Int
  let source_tool: String
}

func writeErr(_ code: String, _ message: String) -> Never {
  let err = HelperError(code: code, message: message)
  if let data = try? JSONEncoder().encode(err), let s = String(data: data, encoding: .utf8) {
    FileHandle.standardError.write(Data(s.utf8))
  } else {
    FileHandle.standardError.write(Data("{\"code\":\"\(code)\",\"message\":\"\(message)\"}".utf8))
  }
  exit(1)
}

func parseArgs() -> String {
  var outPath: String? = nil
  var i = 1
  while i < CommandLine.arguments.count {
    let a = CommandLine.arguments[i]
    if a == "--out" {
      i += 1
      if i < CommandLine.arguments.count {
        outPath = CommandLine.arguments[i]
      }
    }
    i += 1
  }
  guard let p = outPath, !p.isEmpty else {
    writeErr("E006", "missing --out")
  }
  return p
}

let outPath = parseArgs()

let pb = NSPasteboard.general
let items = pb.pasteboardItems ?? []
if items.count == 0 {
  writeErr("E001", "剪贴板为空")
}

guard let objs = pb.readObjects(forClasses: [NSImage.self], options: nil) as? [NSImage], let img = objs.first else {
  writeErr("E002", "剪贴板中无图片")
}

guard let tiff = img.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let pngData = bitmap.representation(using: .png, properties: [:]) else {
  writeErr("E004", "无法转换为 PNG")
}

do {
  try pngData.write(to: URL(fileURLWithPath: outPath), options: .atomic)
} catch {
  writeErr("E007", "写入输出文件失败")
}

let ok = HelperOK(
  mime_type: "image/png",
  width: bitmap.pixelsWide,
  height: bitmap.pixelsHigh,
  source_tool: "nspasteboard"
)

if let data = try? JSONEncoder().encode(ok), let s = String(data: data, encoding: .utf8) {
  FileHandle.standardOutput.write(Data(s.utf8))
} else {
  FileHandle.standardOutput.write(Data("{\"mime_type\":\"image/png\",\"width\":\(bitmap.pixelsWide),\"height\":\(bitmap.pixelsHigh),\"source_tool\":\"nspasteboard\"}".utf8))
}

