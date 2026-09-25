import Foundation
import AppKit
import ApplicationServices
import CoreGraphics

private let maxElements = 256
private let messagingTimeout: Float = 2
private let galleryTitle = "Gallery"
private let newWindowTitle = "New comparison window"
private let browserTitle = "Open in browser"
private let diagnosticEventTag: Int64 = 0x4150504b49544e

private enum ProbeFailure: Error {
    case arguments
    case identity
    case trust
    case activation
    case unavailable
}

private struct Budget {
    var seen = 0

    mutating func add(_ count: Int = 1) throws {
        guard count >= 0, seen <= maxElements - count else { throw ProbeFailure.unavailable }
        seen += count
    }
}

private struct MenuControls {
    let newWindow: AXUIElement
    let browser: AXUIElement
    let newWindowEnabled: Bool
    let browserEnabled: Bool
}

private final class KeyTapObservation {
    var keyDownSeen = false
    var keyUpSeen = false
    var eventShapeMatches = true
}

private func observeKeyEvent(
    proxy: CGEventTapProxy,
    type: CGEventType,
    event: CGEvent,
    userInfo: UnsafeMutableRawPointer?
) -> Unmanaged<CGEvent>? {
    guard let userInfo else { return Unmanaged.passUnretained(event) }
    let observation = Unmanaged<KeyTapObservation>.fromOpaque(userInfo).takeUnretainedValue()
    guard event.getIntegerValueField(.eventSourceUserData) == diagnosticEventTag else {
        return Unmanaged.passUnretained(event)
    }
    let relevantFlags = event.flags.intersection([.maskCommand, .maskShift, .maskControl, .maskAlternate])
    let matches = event.getIntegerValueField(.keyboardEventKeycode) == 45 && relevantFlags == .maskCommand
    if type == .keyDown {
        observation.keyDownSeen = true
        observation.eventShapeMatches = observation.eventShapeMatches && matches
    } else if type == .keyUp {
        observation.keyUpSeen = true
        observation.eventShapeMatches = observation.eventShapeMatches && matches
    }
    return Unmanaged.passUnretained(event)
}

private func copyValue(_ element: AXUIElement, _ attribute: String) throws -> CFTypeRef {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, attribute as CFString, &value) == .success,
          let value else { throw ProbeFailure.unavailable }
    return value
}

private func copyElement(_ element: AXUIElement, _ attribute: String) throws -> AXUIElement {
    let value = try copyValue(element, attribute)
    guard CFGetTypeID(value) == AXUIElementGetTypeID() else { throw ProbeFailure.unavailable }
    return unsafeBitCast(value, to: AXUIElement.self)
}

private func copyElements(_ element: AXUIElement, _ attribute: String) throws -> [AXUIElement] {
    let value = try copyValue(element, attribute)
    guard let values = value as? [AnyObject] else { throw ProbeFailure.unavailable }
    return try values.map { item in
        let value = item as CFTypeRef
        guard CFGetTypeID(value) == AXUIElementGetTypeID() else { throw ProbeFailure.unavailable }
        return unsafeBitCast(value, to: AXUIElement.self)
    }
}

private func optionalChildren(_ element: AXUIElement) throws -> [AXUIElement] {
    var value: CFTypeRef?
    let result = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute as CFString, &value)
    if result == .noValue || result == .attributeUnsupported { return [] }
    guard result == .success, let value, let values = value as? [AnyObject] else {
        throw ProbeFailure.unavailable
    }
    return try values.map { item in
        let value = item as CFTypeRef
        guard CFGetTypeID(value) == AXUIElementGetTypeID() else { throw ProbeFailure.unavailable }
        return unsafeBitCast(value, to: AXUIElement.self)
    }
}

private func copyString(_ element: AXUIElement, _ attribute: String) throws -> String {
    guard let value = try copyValue(element, attribute) as? String else {
        throw ProbeFailure.unavailable
    }
    return value
}

private func copyBool(_ element: AXUIElement, _ attribute: String) throws -> Bool {
    guard let value = try copyValue(element, attribute) as? Bool else {
        throw ProbeFailure.unavailable
    }
    return value
}

private func copyInt(_ element: AXUIElement, _ attribute: String) throws -> Int {
    guard let value = try copyValue(element, attribute) as? NSNumber else {
        throw ProbeFailure.unavailable
    }
    return value.intValue
}

private func press(_ element: AXUIElement) throws {
    guard AXUIElementPerformAction(element, kAXPressAction as CFString) == .success else {
        throw ProbeFailure.unavailable
    }
}

private func findGalleryItem(_ application: AXUIElement, budget: inout Budget) throws -> AXUIElement {
    let menuBar = try copyElement(application, kAXMenuBarAttribute)
    try budget.add()
    let items = try copyElements(menuBar, kAXChildrenAttribute)
    try budget.add(items.count)
    let matches = try items.filter { item in
        try copyString(item, kAXRoleAttribute) == (kAXMenuBarItemRole as String) &&
            copyString(item, kAXTitleAttribute) == galleryTitle
    }
    guard matches.count == 1 else { throw ProbeFailure.unavailable }
    return matches[0]
}

private func findControlsUnderGallery(_ gallery: AXUIElement, budget: inout Budget) throws -> MenuControls? {
    var queue = try optionalChildren(gallery)
    try budget.add(queue.count)
    var index = 0
    var newItems: [AXUIElement] = []
    var browserItems: [AXUIElement] = []
    while index < queue.count {
        let element = queue[index]
        index += 1
        let role = try copyString(element, kAXRoleAttribute)
        if role == (kAXMenuItemRole as String) {
            let title = try copyString(element, kAXTitleAttribute)
            if title == newWindowTitle { newItems.append(element) }
            if title == browserTitle { browserItems.append(element) }
        }
        let children = try optionalChildren(element)
        try budget.add(children.count)
        queue.append(contentsOf: children)
    }
    if newItems.isEmpty && browserItems.isEmpty { return nil }
    guard newItems.count == 1, browserItems.count == 1 else { throw ProbeFailure.unavailable }
    return MenuControls(
        newWindow: newItems[0],
        browser: browserItems[0],
        newWindowEnabled: try copyBool(newItems[0], kAXEnabledAttribute),
        browserEnabled: try copyBool(browserItems[0], kAXEnabledAttribute)
    )
}

private func menuControls(_ application: AXUIElement, budget: inout Budget) throws -> MenuControls {
    let gallery = try findGalleryItem(application, budget: &budget)
    if let controls = try findControlsUnderGallery(gallery, budget: &budget) { return controls }
    try press(gallery)
    guard let controls = try findControlsUnderGallery(gallery, budget: &budget) else {
        throw ProbeFailure.unavailable
    }
    return controls
}

private func windowCount(_ application: AXUIElement, budget: inout Budget) throws -> Int {
    let windows = try copyElements(application, kAXWindowsAttribute)
    try budget.add(windows.count)
    return windows.count
}

private func activate(_ application: NSRunningApplication) throws {
    guard application.activate(options: []) else {
        throw ProbeFailure.activation
    }
    let deadline = Date().addingTimeInterval(1)
    while !application.isActive {
        guard Date() < deadline else { throw ProbeFailure.activation }
        RunLoop.current.run(until: Date().addingTimeInterval(0.01))
    }
}

private func postCommandN(to pid: pid_t, diagnosticTag: Int64? = nil) throws {
    if #available(macOS 10.15, *) {
        guard CGPreflightPostEventAccess() else { throw ProbeFailure.trust }
    }
    guard let down = CGEvent(keyboardEventSource: nil, virtualKey: 45, keyDown: true),
          let up = CGEvent(keyboardEventSource: nil, virtualKey: 45, keyDown: false) else {
        throw ProbeFailure.unavailable
    }
    down.flags = .maskCommand
    up.flags = .maskCommand
    var character: UniChar = 110 // Deterministic 'n', independent of keyboard layout.
    down.keyboardSetUnicodeString(stringLength: 1, unicodeString: &character)
    up.keyboardSetUnicodeString(stringLength: 1, unicodeString: &character)
    if let diagnosticTag {
        down.setIntegerValueField(.eventSourceUserData, value: diagnosticTag)
        up.setIntegerValueField(.eventSourceUserData, value: diagnosticTag)
    }
    down.postToPid(pid)
    up.postToPid(pid)
}

private func postCommandNChord(to pid: pid_t) throws {
    if #available(macOS 10.15, *) {
        guard CGPreflightPostEventAccess() else { throw ProbeFailure.trust }
    }
    guard let source = CGEventSource(stateID: .privateState),
          let commandDown = CGEvent(keyboardEventSource: source, virtualKey: 55, keyDown: true),
          let nDown = CGEvent(keyboardEventSource: source, virtualKey: 45, keyDown: true),
          let nUp = CGEvent(keyboardEventSource: source, virtualKey: 45, keyDown: false),
          let commandUp = CGEvent(keyboardEventSource: source, virtualKey: 55, keyDown: false) else {
        throw ProbeFailure.unavailable
    }
    commandDown.type = .flagsChanged
    commandDown.flags = .maskCommand
    nDown.flags = .maskCommand
    nUp.flags = .maskCommand
    commandUp.type = .flagsChanged
    commandUp.flags = []
    var character: UniChar = 110
    nDown.keyboardSetUnicodeString(stringLength: 1, unicodeString: &character)
    nUp.keyboardSetUnicodeString(stringLength: 1, unicodeString: &character)

    // Keep each modifier/key transition distinct without extending the probe's
    // one-second activation bound or relying on the global event stream.
    for event in [commandDown, nDown, nUp, commandUp] {
        event.postToPid(pid)
        Thread.sleep(forTimeInterval: 0.01)
    }
}

private func focusedWindowFact(_ application: AXUIElement) -> (available: Bool, owned: Bool) {
    guard let focused = try? copyElement(application, kAXFocusedWindowAttribute),
          let windows = try? copyElements(application, kAXWindowsAttribute) else {
        return (false, false)
    }
    return (true, windows.contains { window in CFEqual(focused, window) })
}

private func diagnoseCommandN(
    application: AXUIElement,
    running: NSRunningApplication,
    controls: MenuControls,
    pid: pid_t,
    windows: Int
) throws {
    try activate(running)
    let focused = focusedWindowFact(application)
    let commandCharacter = try? copyString(controls.newWindow, kAXMenuItemCmdCharAttribute)
    let commandModifiers = try? copyInt(controls.newWindow, kAXMenuItemCmdModifiersAttribute)
    let commandVirtualKey = try? copyInt(controls.newWindow, kAXMenuItemCmdVirtualKeyAttribute)
    let characterMatches = commandCharacter?.lowercased() == "n"
    // AX encodes Command as the default: zero means Command with no extras.
    let modifiersMatch = commandModifiers == 0
    let virtualKeyMatches = commandVirtualKey == 45
    var receipt: [String: Any] = [
        "ok": true,
        "operation": "diagnose-key-new-window",
        "pid": Int(pid),
        "windows": windows,
        "focused_window_available": focused.available,
        "focused_window_owned": focused.owned,
        "shortcut_character_available": commandCharacter != nil,
        "shortcut_character_matches": characterMatches,
        "shortcut_modifiers_available": commandModifiers != nil,
        "shortcut_modifiers_match": modifiersMatch,
        "shortcut_virtual_key_available": commandVirtualKey != nil,
        "shortcut_virtual_key_matches": virtualKeyMatches,
        "listen_available": false,
        "key_down_seen": false,
        "key_up_seen": false,
        "event_shape_matches": false,
    ]

    guard CGPreflightListenEventAccess() else {
        receipt["category"] = "listen-permission-unavailable"
        emit(receipt)
        return
    }
    receipt["listen_available"] = true

    let observation = KeyTapObservation()
    let mask = (CGEventMask(1) << CGEventType.keyDown.rawValue) |
        (CGEventMask(1) << CGEventType.keyUp.rawValue)
    guard let tap = CGEvent.tapCreateForPid(
        pid: pid,
        place: .headInsertEventTap,
        options: .listenOnly,
        eventsOfInterest: mask,
        callback: observeKeyEvent,
        userInfo: Unmanaged.passUnretained(observation).toOpaque()
    ) else {
        receipt["category"] = "event-tap-unavailable"
        emit(receipt)
        return
    }
    guard let source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0) else {
        receipt["category"] = "event-source-unavailable"
        emit(receipt)
        return
    }
    CFRunLoopAddSource(CFRunLoopGetCurrent(), source, .defaultMode)
    defer {
        CGEvent.tapEnable(tap: tap, enable: false)
        CFRunLoopRemoveSource(CFRunLoopGetCurrent(), source, .defaultMode)
    }
    CGEvent.tapEnable(tap: tap, enable: true)
    try postCommandN(to: pid, diagnosticTag: diagnosticEventTag)
    let deadline = Date().addingTimeInterval(1)
    while !(observation.keyDownSeen && observation.keyUpSeen) && Date() < deadline {
        RunLoop.current.run(until: Date().addingTimeInterval(0.01))
    }
    receipt["category"] = observation.keyDownSeen && observation.keyUpSeen ? "events-observed" : "events-not-observed"
    receipt["key_down_seen"] = observation.keyDownSeen
    receipt["key_up_seen"] = observation.keyUpSeen
    receipt["event_shape_matches"] = observation.keyDownSeen && observation.keyUpSeen && observation.eventShapeMatches
    emit(receipt)
}

private func emit(_ object: [String: Any]) {
    guard JSONSerialization.isValidJSONObject(object),
          let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]) else {
        return
    }
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write(Data([0x0a]))
}

private func fail(_ category: String, code: Int32 = 1) -> Never {
    emit(["ok": false, "error": category])
    exit(code)
}

private func run() throws {
    let arguments = CommandLine.arguments
    guard arguments.count == 4,
          let rawPID = Int32(arguments[1]), rawPID > 1 else { throw ProbeFailure.arguments }
    let suppliedPath = arguments[2]
    let suppliedURL = URL(fileURLWithPath: suppliedPath)
    let standardized = suppliedURL.standardizedFileURL.path
    let directoryName = suppliedURL.deletingLastPathComponent().lastPathComponent
    guard suppliedPath.hasPrefix("/"), suppliedPath == standardized,
          suppliedURL.lastPathComponent == "gallery-native",
          directoryName.hasPrefix("appkit-native-companion-") else {
        throw ProbeFailure.identity
    }
    let pid = pid_t(rawPID)
    guard let running = NSRunningApplication(processIdentifier: pid),
          !running.isTerminated,
          let executable = running.executableURL?.standardizedFileURL.path,
          executable == suppliedPath else { throw ProbeFailure.identity }
    guard AXIsProcessTrusted() else { throw ProbeFailure.trust }

    let operation = arguments[3]
    guard ["inspect", "new-window", "open-browser", "close-window", "key-new-window", "key-new-window-chord", "diagnose-key-new-window"].contains(operation) else {
        throw ProbeFailure.arguments
    }
    let application = AXUIElementCreateApplication(pid)
    guard AXUIElementSetMessagingTimeout(application, messagingTimeout) == .success else {
        throw ProbeFailure.unavailable
    }
    var budget = Budget()
    let controls = try menuControls(application, budget: &budget)
    let before = try windowCount(application, budget: &budget)

    switch operation {
    case "inspect":
        break
    case "new-window":
        guard controls.newWindowEnabled else { throw ProbeFailure.unavailable }
        try activate(running)
        try press(controls.newWindow)
    case "open-browser":
        guard controls.browserEnabled else { throw ProbeFailure.unavailable }
        try activate(running)
        try press(controls.browser)
    case "close-window":
        try activate(running)
        let windows = try copyElements(application, kAXWindowsAttribute)
        try budget.add(windows.count)
        guard let first = windows.first else { throw ProbeFailure.unavailable }
        try press(copyElement(first, kAXCloseButtonAttribute))
    case "key-new-window":
        // A person can press Cmd+N while the menu is disabled. Deliver the
        // event to this exact PID so the test can prove native refusal too.
        try activate(running)
        try postCommandN(to: pid)
    case "key-new-window-chord":
        try activate(running)
        try postCommandNChord(to: pid)
    case "diagnose-key-new-window":
        try diagnoseCommandN(
            application: application,
            running: running,
            controls: controls,
            pid: pid,
            windows: before
        )
        return
    default:
        throw ProbeFailure.arguments
    }

    emit([
        "ok": true,
        "operation": operation,
        "pid": Int(pid),
        "windows": before,
        "new_window_enabled": controls.newWindowEnabled,
        "browser_enabled": controls.browserEnabled,
    ])
}

do {
    try run()
} catch ProbeFailure.arguments {
    fail("invalid-arguments", code: 2)
} catch ProbeFailure.identity {
    fail("native-identity-mismatch")
} catch ProbeFailure.trust {
    fail("accessibility-unavailable")
} catch ProbeFailure.activation {
    fail("activation-unavailable")
} catch {
    fail("native-ui-unavailable")
}
