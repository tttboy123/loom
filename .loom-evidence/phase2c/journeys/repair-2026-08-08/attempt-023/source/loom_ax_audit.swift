import AppKit
import ApplicationServices

let bundleID = "com.earendilworks.loom.local"

func textAttribute(_ element: AXUIElement, _ name: String) -> String {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, name as CFString, &value) == .success,
          let value else { return "" }
    if let text = value as? String { return text }
    return String(describing: value)
}

func boolAttribute(_ element: AXUIElement, _ name: String) -> Bool {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, name as CFString, &value) == .success,
          let number = value as? NSNumber else { return false }
    return number.boolValue
}

func elementsAttribute(_ element: AXUIElement, _ name: String) -> [AXUIElement] {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, name as CFString, &value) == .success else { return [] }
    return value as? [AXUIElement] ?? []
}

func elementAttribute(_ element: AXUIElement, _ name: String) -> AXUIElement? {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, name as CFString, &value) == .success,
          let value else { return nil }
    return unsafeBitCast(value, to: AXUIElement.self)
}

func actions(_ element: AXUIElement) -> [String] {
    var names: CFArray?
    guard AXUIElementCopyActionNames(element, &names) == .success else { return [] }
    return names as? [String] ?? []
}

func descendants(_ root: AXUIElement) -> [AXUIElement] {
    var result: [AXUIElement] = []
    var stack = elementsAttribute(root, kAXChildrenAttribute)
    var seen = Set<CFHashCode>()
    while let element = stack.popLast() {
        let key = CFHash(element)
        if seen.contains(key) { continue }
        seen.insert(key)
        result.append(element)
        stack.append(contentsOf: elementsAttribute(element, kAXChildrenAttribute))
    }
    return result
}

func treeDescendants(_ root: AXUIElement, depth: Int = 0) -> [AXUIElement] {
    guard depth < 24 else { return [] }
    return elementsAttribute(root, kAXChildrenAttribute).flatMap { child in
        [child] + treeDescendants(child, depth: depth + 1)
    }
}

func runningApp() -> NSRunningApplication {
    let deadline = Date().addingTimeInterval(10)
    while Date() < deadline {
        if let app = NSRunningApplication.runningApplications(withBundleIdentifier: bundleID).last {
            return app
        }
        Thread.sleep(forTimeInterval: 0.1)
    }
    fputs("Loom app not running\n", stderr)
    exit(2)
}

let app = runningApp()
let appElement = AXUIElementCreateApplication(app.processIdentifier)
let command = CommandLine.arguments.dropFirst().first ?? "dump"

switch command {
case "audit-json":
    let standardSubroles = Set(["AXCloseButton", "AXMinimizeButton", "AXFullScreenButton", "AXZoomButton"])
    var controls: [[String: Any]] = []
    for element in treeDescendants(appElement) {
        let role = textAttribute(element, kAXRoleAttribute)
        let elementActions = actions(element)
        guard role == "AXButton", elementActions.contains("AXPress"),
              boolAttribute(element, kAXEnabledAttribute) else { continue }
        let subrole = textAttribute(element, kAXSubroleAttribute)
        let parent = elementAttribute(element, kAXParentAttribute)
        let parentRole = parent.map { textAttribute($0, kAXRoleAttribute) } ?? ""
        let excluded = standardSubroles.contains(subrole) || parentRole == "AXScrollBar"
        let description = textAttribute(element, kAXDescriptionAttribute)
        let help = textAttribute(element, kAXHelpAttribute)
        let title = textAttribute(element, kAXTitleAttribute)
        let identifier = textAttribute(element, kAXIdentifierAttribute)
        controls.append([
            "role": role,
            "subrole": subrole,
            "parent_role": parentRole,
            "description": description,
            "help": help,
            "title": title,
            "identifier": identifier,
            "actions": elementActions,
            "enabled": true,
            "excluded_standard_chrome": excluded,
            "labeled": excluded || ![description, help, title, identifier].allSatisfy {
                $0.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            }
        ])
    }
    let product = controls.filter { !($0["excluded_standard_chrome"] as? Bool ?? false) }
    let recent = product.filter {
        ($0["description"] as? String ?? "").hasPrefix("Open recent task ")
    }
    let recentDescriptions = recent.map { $0["description"] as? String ?? "" }
    let recentHelpMatches = recent.allSatisfy {
        ($0["description"] as? String ?? "") == ($0["help"] as? String ?? "")
    }
    let summary: [String: Any] = [
        "product_control_count": product.count,
        "unlabeled_product_control_count": product.filter { !($0["labeled"] as? Bool ?? false) }.count,
        "recent_control_count": recent.count,
        "recent_unique_label_count": Set(recentDescriptions).count,
        "recent_help_matches_label": recentHelpMatches,
        "recent_labels": recentDescriptions
    ]
    let output: [String: Any] = [
        "schema_version": 1,
        "bundle_id": bundleID,
        "controls": controls,
        "summary": summary
    ]
    let data = try JSONSerialization.data(withJSONObject: output, options: [.prettyPrinted, .sortedKeys])
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write(Data("\n".utf8))
    let pass = (summary["unlabeled_product_control_count"] as? Int == 0) &&
        recent.count == 4 && Set(recentDescriptions).count == 4 && recentHelpMatches
    exit(pass ? 0 : 12)

case "dump":
    let standardSubroles = Set(["AXCloseButton", "AXMinimizeButton", "AXFullScreenButton", "AXZoomButton"])
    var productCount = 0
    var emptyHelpCount = 0
    for element in descendants(appElement) {
        let role = textAttribute(element, kAXRoleAttribute)
        let elementActions = actions(element)
        guard role == "AXButton", elementActions.contains("AXPress"),
              boolAttribute(element, kAXEnabledAttribute) else { continue }
        let subrole = textAttribute(element, kAXSubroleAttribute)
        let parent = elementAttribute(element, kAXParentAttribute)
        let parentRole = parent.map { textAttribute($0, kAXRoleAttribute) } ?? ""
        let excluded = standardSubroles.contains(subrole) || parentRole == "AXScrollBar"
        let help = textAttribute(element, kAXHelpAttribute)
        let title = textAttribute(element, kAXTitleAttribute)
        let value = textAttribute(element, kAXValueAttribute)
        let identifier = textAttribute(element, kAXIdentifierAttribute)
        print("excluded=\(excluded)|help=\(help)|title=\(title)|value=\(value)|identifier=\(identifier)|subrole=\(subrole)|parent_role=\(parentRole)|actions=\(elementActions.joined(separator: ","))")
        if !excluded {
            productCount += 1
            if help.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines).isEmpty { emptyHelpCount += 1 }
        }
    }
    print("SUMMARY product=\(productCount) empty_help=\(emptyHelpCount)")
    exit(emptyHelpCount == 0 ? 0 : 3)

case "focused":
    var focused: CFTypeRef?
    let appFocused = AXUIElementCopyAttributeValue(
        appElement,
        kAXFocusedUIElementAttribute as CFString,
        &focused
    )
    if appFocused != .success || focused == nil {
        let system = AXUIElementCreateSystemWide()
        _ = AXUIElementCopyAttributeValue(
            system,
            kAXFocusedUIElementAttribute as CFString,
            &focused
        )
    }
    guard let element = focused else {
        print("FOCUSED none")
        exit(4)
    }
    let ax = unsafeBitCast(element, to: AXUIElement.self)
    print("FOCUSED role=\(textAttribute(ax, kAXRoleAttribute))|help=\(textAttribute(ax, kAXHelpAttribute))|title=\(textAttribute(ax, kAXTitleAttribute))|value=\(textAttribute(ax, kAXValueAttribute))|identifier=\(textAttribute(ax, kAXIdentifierAttribute))")

case "press-help":
    guard CommandLine.arguments.count == 3 else { exit(5) }
    let target = CommandLine.arguments[2]
    guard let element = descendants(appElement).first(where: {
        textAttribute($0, kAXHelpAttribute) == target && actions($0).contains("AXPress")
    }) else {
        fputs("help target not found: \(target)\n", stderr)
        exit(6)
    }
    let result = AXUIElementPerformAction(element, kAXPressAction as CFString)
    print("PRESSED help=\(target) result=\(result.rawValue)")
    exit(result == .success ? 0 : 7)

case "press-description":
    guard CommandLine.arguments.count == 3 else { exit(13) }
    let target = CommandLine.arguments[2]
    guard let element = treeDescendants(appElement).first(where: {
        textAttribute($0, kAXDescriptionAttribute) == target && actions($0).contains("AXPress")
    }) else {
        fputs("description target not found: \(target)\n", stderr)
        exit(14)
    }
    let result = AXUIElementPerformAction(element, kAXPressAction as CFString)
    print("PRESSED description=\(target) result=\(result.rawValue)")
    exit(result == .success ? 0 : 15)

case "set-size":
    guard CommandLine.arguments.count == 4,
          let width = Double(CommandLine.arguments[2]),
          let height = Double(CommandLine.arguments[3]),
          let window = elementsAttribute(appElement, kAXWindowsAttribute).first else { exit(8) }
    var position = CGPoint(x: 36, y: 48)
    var size = CGSize(width: width, height: height)
    let positionValue = AXValueCreate(.cgPoint, &position)!
    let sizeValue = AXValueCreate(.cgSize, &size)!
    let p = AXUIElementSetAttributeValue(window, kAXPositionAttribute as CFString, positionValue)
    let s = AXUIElementSetAttributeValue(window, kAXSizeAttribute as CFString, sizeValue)
    print("SET_SIZE width=\(Int(width)) height=\(Int(height)) position=\(p.rawValue) size=\(s.rawValue)")
    exit(p == .success && s == .success ? 0 : 9)

case "window-id":
    let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
    let info = CGWindowListCopyWindowInfo(options, kCGNullWindowID) as? [[String: Any]] ?? []
    let candidates = info.filter { ($0[kCGWindowOwnerPID as String] as? pid_t) == app.processIdentifier && ($0[kCGWindowLayer as String] as? Int) == 0 }
    guard let window = candidates.max(by: {
        let a = ($0[kCGWindowBounds as String] as? [String: CGFloat]) ?? [:]
        let b = ($1[kCGWindowBounds as String] as? [String: CGFloat]) ?? [:]
        return (a["Width", default: 0] * a["Height", default: 0]) < (b["Width", default: 0] * b["Height", default: 0])
    }), let id = window[kCGWindowNumber as String] as? CGWindowID else { exit(10) }
    print(id)

case "windows":
    let info = CGWindowListCopyWindowInfo([.optionAll], kCGNullWindowID) as? [[String: Any]] ?? []
    for window in info where (window[kCGWindowOwnerPID as String] as? pid_t) == app.processIdentifier {
        let id = window[kCGWindowNumber as String] ?? ""
        let layer = window[kCGWindowLayer as String] ?? ""
        let name = window[kCGWindowName as String] ?? ""
        let bounds = window[kCGWindowBounds as String] ?? ""
        let onscreen = window[kCGWindowIsOnscreen as String] ?? ""
        print("WINDOW id=\(id)|layer=\(layer)|name=\(name)|bounds=\(bounds)|onscreen=\(onscreen)")
    }

case "new-window":
    let source = CGEventSource(stateID: .combinedSessionState)
    let down = CGEvent(keyboardEventSource: source, virtualKey: 45, keyDown: true)!
    let up = CGEvent(keyboardEventSource: source, virtualKey: 45, keyDown: false)!
    down.flags = [.maskCommand]
    up.flags = [.maskCommand]
    down.postToPid(app.processIdentifier)
    up.postToPid(app.processIdentifier)
    print("NEW_WINDOW pid=\(app.processIdentifier)")

case "activate":
    let activated = app.activate(options: [.activateAllWindows, .activateIgnoringOtherApps])
    print("ACTIVATE pid=\(app.processIdentifier) result=\(activated)")

default:
    fputs("unknown command\n", stderr)
    exit(11)
}
