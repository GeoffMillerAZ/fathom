import Foundation
import AppKit
import CoreGraphics

private let diagnosticEventTag: Int64 = 0x4150504b49544e
private let windowTitle = "App Kit Gallery (experimental)"
private let newWindowTitle = "New comparison window"
private let browserTitle = "Open in browser"

private func emit(_ values: [String: Any]) {
    guard JSONSerialization.isValidJSONObject(values),
          let data = try? JSONSerialization.data(withJSONObject: values, options: [.sortedKeys]) else {
        return
    }
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write(Data([0x0a]))
}

private func isDiagnosticEvent(_ event: NSEvent) -> Bool {
    event.cgEvent?.getIntegerValueField(.eventSourceUserData) == diagnosticEventTag
}

private final class DiagnosticMenu: NSMenu {
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        let handled = super.performKeyEquivalent(with: event)
        if isDiagnosticEvent(event) {
            emit([
                "event": "menu_equivalent",
                "handled": handled,
                "gallery": title == "Gallery",
            ])
        }
        return handled
    }
}

private final class DiagnosticWindow: NSWindow {
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        let handled = super.performKeyEquivalent(with: event)
        if isDiagnosticEvent(event) {
            emit(["event": "window_equivalent", "handled": handled])
        }
        return handled
    }
}

private final class DiagnosticApplication: NSApplication {
    override func sendEvent(_ event: NSEvent) {
        if let cgEvent = event.cgEvent,
           cgEvent.getIntegerValueField(.eventSourceUserData) == diagnosticEventTag,
           event.type == .keyDown || event.type == .keyUp {
            let owner = delegate as? ControlDelegate
            emit([
                "event": "tagged_key",
                "key_down": event.type == .keyDown,
                "key_up": event.type == .keyUp,
                "n_characters": event.characters == "n" && event.charactersIgnoringModifiers == "n",
                "command": event.modifierFlags.contains(.command),
                "window_belongs": owner?.owns(event.window) ?? false,
                "menu_owned": NSRunningApplication.current.ownsMenuBar,
                "app_active": NSApp.isActive,
                "new_enabled": owner?.newEnabled ?? false,
            ])
        }
        super.sendEvent(event)
    }
}

private final class ControlDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, NSMenuDelegate {
    private var windows: [NSWindow] = []
    private var createdCount = 0
    private var menuActionCount = 0
    private var finishedLaunching = false
    private var newWindowItem: NSMenuItem!
    var newEnabled: Bool { newWindowItem?.isEnabled ?? false }

    func applicationDidFinishLaunching(_ notification: Notification) {
        finishedLaunching = true
        installMenu()
        createWindow()
        // Match the pinned Wails regular-app activation path for this control.
        NSApp.activate(ignoringOtherApps: true)
        DispatchQueue.main.async {
            emit([
                "event": "ready",
                "count": self.windows.count,
                "is_active": NSApp.isActive,
                "finished_launching": self.finishedLaunching,
                "key_window_owned": self.owns(NSApp.keyWindow),
            ])
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 20) {
            emit(["event": "timeout", "count": 1])
            exit(124)
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }

    func windowWillClose(_ notification: Notification) {
        guard let closing = notification.object as? NSWindow else { return }
        windows.removeAll { $0 === closing }
        updateNewWindowEnabled()
    }

    func owns(_ window: NSWindow?) -> Bool {
        guard let window else { return false }
        return windows.contains { $0 === window }
    }

    func menuWillOpen(_ menu: NSMenu) {
        emit(["event": "menu_open", "count": windows.count])
    }

    func menuDidClose(_ menu: NSMenu) {
        emit(["event": "menu_closed", "count": windows.count])
    }

    @objc private func newComparisonWindow(_ sender: Any?) {
        menuActionCount += 1
        emit(["event": "menu_action", "count": menuActionCount])
        guard windows.count < 2 else {
            updateNewWindowEnabled()
            return
        }
        createWindow()
    }

    @objc private func browserPlaceholder(_ sender: Any?) {
        emit(["event": "browser_placeholder", "count": 1])
    }

    @objc private func quit(_ sender: Any?) {
        NSApp.terminate(nil)
    }

    private func installMenu() {
        let mainMenu = DiagnosticMenu()

        let applicationItem = NSMenuItem()
        mainMenu.addItem(applicationItem)
        let applicationMenu = NSMenu(title: "Application")
        applicationMenu.addItem(withTitle: "Quit", action: #selector(quit(_:)), keyEquivalent: "q").target = self
        applicationItem.submenu = applicationMenu

        let galleryItem = NSMenuItem()
        galleryItem.title = "Gallery"
        mainMenu.addItem(galleryItem)
        let galleryMenu = DiagnosticMenu(title: "Gallery")
        galleryMenu.delegate = self
        newWindowItem = galleryMenu.addItem(
            withTitle: newWindowTitle,
            action: #selector(newComparisonWindow(_:)),
            keyEquivalent: "n"
        )
        newWindowItem.keyEquivalentModifierMask = [.command]
        newWindowItem.target = self
        let browserItem = galleryMenu.addItem(
            withTitle: browserTitle,
            action: #selector(browserPlaceholder(_:)),
            keyEquivalent: ""
        )
        browserItem.target = self
        galleryItem.submenu = galleryMenu
        NSApp.mainMenu = mainMenu
    }

    private func createWindow() {
        let offset = CGFloat(createdCount * 28)
        let window = DiagnosticWindow(
            contentRect: NSRect(x: 160 + offset, y: 160 + offset, width: 720, height: 480),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = windowTitle
        window.isReleasedWhenClosed = false
        window.delegate = self
        let label = NSTextField(labelWithString: "Native UI diagnostic control")
        label.alignment = .center
        label.frame = window.contentView?.bounds ?? .zero
        label.autoresizingMask = [.width, .height]
        window.contentView?.addSubview(label)
        windows.append(window)
        createdCount += 1
        window.makeKeyAndOrderFront(nil)
        updateNewWindowEnabled()
        emit(["event": "created", "count": createdCount])
    }

    private func updateNewWindowEnabled() {
        newWindowItem?.isEnabled = windows.count < 2
    }
}

private let application = DiagnosticApplication.shared
private let control = ControlDelegate()
application.delegate = control
application.setActivationPolicy(.regular)
application.run()
