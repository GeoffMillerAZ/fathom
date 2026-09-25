import Foundation
import AppKit
import ApplicationServices

let workflowMaxWindows = 2, workflowMaxNodes = 512, workflowMaxChildren = 64, workflowMaxDepth = 32
let workflowMaxTextBytes = 256, workflowMaxOutputBytes = 128 * 1024
let workflowDeadline: TimeInterval = 45

struct ControlFact {
    let role: String
    let title: String?
    let description: String?
    let value: String?
    let enabled: Bool?
    let settable: Bool?
    let actions: [String]?
    let focused: Bool?
    let expanded: Bool?
    let expandedSettable: Bool?
}
enum SelectorOperation { case setValue, press, disclosure }
enum WorkflowRefusal: Error, Equatable, CustomStringConvertible {
    case count(String), incomplete(String), missing(String), duplicate(String), role(String), disabled(String), unsupported(String)
    var description: String {
        switch self {
        case .count(let s), .incomplete(let s), .missing(let s), .duplicate(let s), .role(let s), .disabled(let s), .unsupported(let s): return s
        }
    }
}

private let publicDisclosureActions = Set([kAXPressAction as String])
// This public AX action is reported as the literal `AXScrollToVisible`; the
// macOS Swift overlay does not export a kAXScrollToVisibleAction constant.
private let publicScrollAction = "AXScrollToVisible"

// AXScrollToVisible is only a bounded recovery action. It is never evidence that
// the disclosure opened, and it is unavailable whenever the advertised surface
// is ambiguous.
func disclosureScrollFallbackAction(_ fact: ControlFact, name: String) throws -> String? {
    guard fact.enabled == true else { throw WorkflowRefusal.disabled("disabled-or-unreadable:\(name)") }
    let actions = fact.actions ?? []
    let presses = actions.filter { $0 == kAXPressAction as String }
    if presses.count == 1 { return nil } // Prefer the ordinary advertised action.
    guard presses.isEmpty else { throw WorkflowRefusal.unsupported("disclosure-action-count:\(name):\(actions.joined(separator: ","))") }
    let scrolls = actions.filter { $0 == publicScrollAction }
    guard scrolls.count == 1 else { throw WorkflowRefusal.unsupported("disclosure-scroll-action-count:\(name):\(actions.joined(separator: ","))") }
    return publicScrollAction
}

func scrolledDisclosureNeedsPress(_ sameElement: Bool, _ fact: ControlFact, name: String) throws -> Bool {
    guard sameElement else { throw WorkflowRefusal.unsupported("disclosure-changed-after-scroll:\(name)") }
    guard fact.enabled == true else { throw WorkflowRefusal.disabled("disabled-or-unreadable:\(name)") }
    let presses = (fact.actions ?? []).filter { $0 == kAXPressAction as String }
    guard presses.count <= 1 else { throw WorkflowRefusal.unsupported("disclosure-action-count:\(name):\((fact.actions ?? []).joined(separator: ","))") }
    return presses.count == 1
}

func selectExact(_ facts: [ControlFact], name: String, roles: Set<String>, operation: SelectorOperation) throws -> Int {
    let named = facts.indices.filter { facts[$0].title == name || facts[$0].description == name }
    guard !named.isEmpty else { throw WorkflowRefusal.missing("missing:\(name)") }
    let found = named.filter { roles.contains(facts[$0].role) }
    guard !found.isEmpty else { throw WorkflowRefusal.role("role:\(name):\(named.map { facts[$0].role }.joined(separator: ","))") }
    guard found.count == 1 else { throw WorkflowRefusal.duplicate("duplicate:\(name)") }
    let i = found[0], f = facts[i]
    guard f.enabled == true else { throw WorkflowRefusal.disabled("disabled-or-unreadable:\(name)") }
    switch operation {
    case .setValue:
        guard f.settable == true else { throw WorkflowRefusal.unsupported("not-settable:\(name)") }
    case .press:
        guard f.actions?.contains(kAXPressAction as String) == true else {
            throw WorkflowRefusal.unsupported("press-unsupported:\(name):\((f.actions ?? []).joined(separator: ","))")
        }
    case .disclosure:
        let advertised = (f.actions ?? []).filter(publicDisclosureActions.contains)
        guard advertised.count == 1 else {
            throw WorkflowRefusal.unsupported("disclosure-action-count:\(name):\((f.actions ?? []).joined(separator: ","))")
        }
    }
    return i
}

func requireOracle(_ texts: [String], exact: [String] = [], contains: [String] = []) throws -> [String] {
    var selected: [String] = []
    for wanted in exact {
        guard texts.contains(wanted) else { throw WorkflowRefusal.missing("oracle-exact:\(wanted)") }
        selected.append(wanted)
    }
    for wanted in contains {
        guard let match = texts.first(where: { $0.contains(wanted) }) else { throw WorkflowRefusal.missing("oracle-contains:\(wanted)") }
        selected.append(match)
    }
    return selected
}

func sameWindowSet(_ initial: [Int], _ current: [Int]) -> Bool {
    initial.count == 2 && current.count == 2 && Set(initial).count == 2 && Set(initial) == Set(current)
}

enum ObservationFailureKind: String, Equatable { case transientRead, unreadable, identity, limit }
struct ObservationReadFailure: Error, CustomStringConvertible {
    let kind: ObservationFailureKind
    let operation, attribute: String
    let code: Int32
    let window, nodes: Int?
    var description: String { "observation-\(kind.rawValue):\(operation):\(attribute):\(code)" }
    func located(window: Int, nodes: Int) -> ObservationReadFailure {
        ObservationReadFailure(kind: kind, operation: operation, attribute: attribute, code: code, window: window, nodes: nodes)
    }
}
func classifyObservationFailure(code: Int32, noValueExpected: Bool = false) -> ObservationFailureKind {
    if code == AXError.invalidUIElement.rawValue || code == AXError.cannotComplete.rawValue ||
       (noValueExpected && code == AXError.noValue.rawValue) { return .transientRead }
    return .unreadable
}
func shouldRetryObservation(_ failure: ObservationReadFailure) -> Bool { failure.kind == .transientRead }

#if !WORKFLOW_SELF_TEST
private enum LiveFailure: Error, CustomStringConvertible {
    case arguments, identity, trust, unavailable(String)
    var description: String {
        switch self { case .arguments: return "invalid-arguments"; case .identity: return "native-identity-mismatch"; case .trust: return "accessibility-unavailable"; case .unavailable(let s): return s }
    }
}
private struct LiveNode {
    let element: AXUIElement
    let fact: ControlFact
    let texts: [String]
}
private struct Capture {
    let windows: [[LiveNode]]
    var nodeCount: Int { windows.reduce(0) { $0 + $1.count } }
}

private final class Driver {
    private let pid: pid_t, path: String, app: AXUIElement, windows: [AXUIElement], deadline: Date
    private var steps: [[String: Any]] = []
    private var incompleteCaptures: [[String: Any]] = []
    private let childAttributes = [kAXChildrenAttribute as String, kAXContentsAttribute as String, kAXVisibleChildrenAttribute as String]

    init(pid: pid_t, path: String) throws {
        self.pid = pid; self.path = path; deadline = Date().addingTimeInterval(workflowDeadline)
        guard let running = NSRunningApplication(processIdentifier: pid), !running.isTerminated,
              running.executableURL?.standardizedFileURL.path == path else { throw LiveFailure.identity }
        guard AXIsProcessTrusted() else { throw LiveFailure.trust }
        app = AXUIElementCreateApplication(pid)
        guard AXUIElementSetMessagingTimeout(app, 1) == .success else { throw LiveFailure.unavailable("messaging-timeout") }
        windows = try Driver.readWindows(app)
        guard windows.count == workflowMaxWindows, !CFEqual(windows[0], windows[1]) else { throw LiveFailure.unavailable("window-count-or-duplicate:\(windows.count)") }
    }

    private static func readWindows(_ app: AXUIElement) throws -> [AXUIElement] {
        var value: CFTypeRef?
        let result = AXUIElementCopyAttributeValue(app, kAXWindowsAttribute as CFString, &value)
        guard result == .success else {
            throw ObservationReadFailure(kind: .identity, operation: "copy-windows", attribute: kAXWindowsAttribute as String,
                code: result.rawValue, window: nil, nodes: nil)
        }
        guard let raw = value as? [AnyObject], raw.count <= workflowMaxWindows else { throw LiveFailure.unavailable("windows-type-or-count") }
        return try raw.map {
            let v = $0 as CFTypeRef
            guard CFGetTypeID(v) == AXUIElementGetTypeID() else { throw LiveFailure.unavailable("window-type") }
            return unsafeBitCast(v, to: AXUIElement.self)
        }
    }

    private func checkDeadline() throws { if Date() >= deadline { throw LiveFailure.unavailable("soft-deadline") } }
    private func checkIdentityAndWindows() throws {
        try checkDeadline()
        guard let running = NSRunningApplication(processIdentifier: pid), !running.isTerminated,
              running.executableURL?.standardizedFileURL.path == path else { throw LiveFailure.identity }
        let current = try Driver.readWindows(app)
        guard current.count == 2, !CFEqual(current[0], current[1]), windows.allSatisfy({ old in current.contains(where: { CFEqual(old, $0) }) }),
              current.allSatisfy({ now in windows.contains(where: { CFEqual(now, $0) }) }) else {
            throw LiveFailure.unavailable("window-set-changed")
        }
    }

    private func raw(_ element: AXUIElement, _ attribute: String) throws -> CFTypeRef? {
        try checkDeadline(); var value: CFTypeRef?
        let result = AXUIElementCopyAttributeValue(element, attribute as CFString, &value)
        if result == .attributeUnsupported || result == .noValue { return nil }
        guard result == .success else {
            throw ObservationReadFailure(kind: classifyObservationFailure(code: result.rawValue),
                operation: "copy-attribute", attribute: attribute, code: result.rawValue, window: nil, nodes: nil)
        }
        return value
    }
    private func text(_ element: AXUIElement, _ attribute: String) throws -> String? {
        guard let value = try raw(element, attribute) else { return nil }
        let string: String?
        if let s = value as? String { string = s }
        else if let n = value as? NSNumber { string = n.stringValue }
        else { return nil }
        guard let string, string.utf8.count <= workflowMaxTextBytes else { throw LiveFailure.unavailable("text-limit:\(attribute)") }
        return string
    }
    private func bool(_ element: AXUIElement, _ attribute: String) throws -> Bool? {
        guard let value = try raw(element, attribute) else { return nil }
        guard let number = value as? NSNumber else { throw LiveFailure.unavailable("boolean-type:\(attribute)") }
        return number.boolValue
    }
    private func actions(_ element: AXUIElement) throws -> [String]? {
        try checkDeadline(); var value: CFArray?
        let result = AXUIElementCopyActionNames(element, &value)
        if result == .actionUnsupported || result == .notImplemented { return [] }
        guard result == .success else {
            throw ObservationReadFailure(kind: classifyObservationFailure(code: result.rawValue, noValueExpected: true),
                operation: "copy-actions", attribute: "AXActionNames", code: result.rawValue, window: nil, nodes: nil)
        }
        guard value != nil else {
            throw ObservationReadFailure(kind: .transientRead, operation: "copy-actions-nil-success",
                attribute: "AXActionNames", code: result.rawValue, window: nil, nodes: nil)
        }
        guard let names = value as? [String] else {
            throw ObservationReadFailure(kind: .unreadable, operation: "copy-actions-type",
                attribute: "AXActionNames", code: result.rawValue, window: nil, nodes: nil)
        }
        guard names.count <= workflowMaxChildren else { throw LiveFailure.unavailable("actions-limit") }
        guard names.allSatisfy({ $0.utf8.count <= workflowMaxTextBytes }) else { throw LiveFailure.unavailable("action-text-limit") }
        return names.sorted()
    }
    private func children(_ element: AXUIElement) throws -> [AXUIElement] {
        var result: [AXUIElement] = []
        for attribute in childAttributes {
            try checkDeadline(); var count = 0
            let countResult = AXUIElementGetAttributeValueCount(element, attribute as CFString, &count)
            if countResult == .attributeUnsupported || countResult == .noValue { continue }
            guard countResult == .success else {
                throw ObservationReadFailure(kind: classifyObservationFailure(code: countResult.rawValue),
                    operation: "children-count", attribute: attribute, code: countResult.rawValue, window: nil, nodes: nil)
            }
            guard count >= 0, count <= workflowMaxChildren else { throw LiveFailure.unavailable("children-count-limit:\(attribute):\(count)") }
            if count == 0 { continue }
            var copied: CFArray?
            let copyResult = AXUIElementCopyAttributeValues(element, attribute as CFString, 0, count, &copied)
            guard copyResult == .success else {
                throw ObservationReadFailure(kind: classifyObservationFailure(code: copyResult.rawValue),
                    operation: "copy-children", attribute: attribute, code: copyResult.rawValue, window: nil, nodes: nil)
            }
            guard let values = copied as? [AnyObject], values.count == count else {
                throw ObservationReadFailure(kind: .unreadable, operation: "copy-children-type",
                    attribute: attribute, code: copyResult.rawValue, window: nil, nodes: nil)
            }
            for object in values {
                let raw = object as CFTypeRef
                guard CFGetTypeID(raw) == AXUIElementGetTypeID() else {
                    throw ObservationReadFailure(kind: .unreadable, operation: "child-type", attribute: attribute,
                        code: AXError.success.rawValue, window: nil, nodes: nil)
                }
                let child = unsafeBitCast(raw, to: AXUIElement.self)
                if !result.contains(where: { CFEqual($0, child) }) {
                    guard result.count < workflowMaxChildren else { throw LiveFailure.unavailable("children-union-limit:\(attribute)") }
                    result.append(child)
                }
            }
        }
        return result
    }
    private func capture() throws -> Capture {
        try checkIdentityAndWindows()
        var all: [[LiveNode]] = [], total = 0
        for window in windows {
            do {
              var queue: [(AXUIElement, Int)] = [(window, 0)], seen: [AXUIElement] = [], nodes: [LiveNode] = [], cursor = 0
              while cursor < queue.count {
                try checkDeadline(); let (element, depth) = queue[cursor]; cursor += 1
                if seen.contains(where: { CFEqual($0, element) }) { continue }
                guard depth <= workflowMaxDepth else { throw LiveFailure.unavailable("depth-limit") }
                seen.append(element); total += 1
                guard total <= workflowMaxNodes else { throw LiveFailure.unavailable("node-limit") }
                guard let role = try text(element, kAXRoleAttribute as String) else { throw LiveFailure.unavailable("role-unreadable") }
                let title = try text(element, kAXTitleAttribute as String)
                let description = try text(element, kAXDescriptionAttribute as String)
                let value = try text(element, kAXValueAttribute as String)
                var settable = DarwinBoolean(false)
                let setResult = AXUIElementIsAttributeSettable(element, kAXValueAttribute as CFString, &settable)
                let canSet = setResult == .success ? settable.boolValue : nil
                var expandedSettable = DarwinBoolean(false)
                let expandedSetResult = AXUIElementIsAttributeSettable(element, kAXExpandedAttribute as CFString, &expandedSettable)
                let actionNames = try actions(element)
                let fact = ControlFact(role: role, title: title, description: description, value: value,
                                       enabled: try bool(element, kAXEnabledAttribute as String), settable: canSet, actions: actionNames,
                                       focused: try bool(element, kAXFocusedAttribute as String),
                                       expanded: try bool(element, kAXExpandedAttribute as String),
                                       expandedSettable: expandedSetResult == .success ? expandedSettable.boolValue : nil)
                nodes.append(LiveNode(element: element, fact: fact, texts: [title, description, value].compactMap { $0 }))
                queue += try children(element).map { ($0, depth + 1) }
              }
              all.append(nodes)
            } catch let error as ObservationReadFailure {
                throw error.located(window: all.count, nodes: total)
            }
        }
        return Capture(windows: all)
    }

    private func facts(_ capture: Capture, _ window: Int) -> [ControlFact] { capture.windows[window].map(\.fact) }
    private func selected(_ capture: Capture, _ window: Int, name: String, roles: Set<String>, operation: SelectorOperation) throws -> LiveNode {
        let index = try selectExact(facts(capture, window), name: name, roles: roles, operation: operation)
        return capture.windows[window][index]
    }
    private func namedRole(_ capture: Capture, _ window: Int, name: String, roles: Set<String>) throws -> LiveNode {
        let found = capture.windows[window].filter { ($0.fact.title == name || $0.fact.description == name) && roles.contains($0.fact.role) }
        guard found.count == 1 else { throw found.isEmpty ? WorkflowRefusal.missing("missing:\(name)") : WorkflowRefusal.duplicate("duplicate:\(name)") }
        return found[0]
    }
    private func allTexts(_ capture: Capture, _ window: Int) -> [String] { capture.windows[window].flatMap(\.texts) }
    private func observe(_ label: String, _ capture: Capture, selected: [String], controls: [[String: Any]] = []) {
        steps.append(["step": label, "node_count": capture.nodeCount, "selected_oracle_text": selected, "controls": controls])
    }
    private func command(_ label: String, window: Int, name: String, action: String, result: AXError? = nil) {
        var item: [String: Any] = ["step": label, "command": ["window": window, "name": name, "action": action, "status": result == nil ? "attempted" : "returned"]]
        if let result { var c = item["command"] as! [String: Any]; c["ax_return_code"] = result.rawValue; item["command"] = c }
        steps.append(item)
    }
    private func wait(_ label: String, timeout: TimeInterval = 4, predicate: (Capture) throws -> ([String], [[String: Any]])) throws -> Capture {
        let local = min(deadline, Date().addingTimeInterval(timeout)); var last = "state-not-ready", lastCapture: Capture?
        while Date() < local {
            do {
                let capture = try self.capture(); lastCapture = capture; let result = try predicate(capture)
                observe(label, capture, selected: result.0, controls: result.1); return capture
            } catch let error as WorkflowRefusal { last = error.description }
              catch let error as ObservationReadFailure {
                guard shouldRetryObservation(error) else { throw error }
                guard incompleteCaptures.count < 63 else { throw LiveFailure.unavailable("incomplete-capture-limit") }
                incompleteCaptures.append(["wait": label, "kind": error.kind.rawValue, "operation": error.operation,
                    "attribute": error.attribute, "ax_return_code": error.code,
                    "window": error.window ?? -1, "nodes_before_failure": error.nodes ?? -1])
                last = error.description
            }
            RunLoop.current.run(until: Date().addingTimeInterval(0.05))
        }
        if let lastCapture {
            var controls: [[String: Any]] = []
            let names: [(String, Set<String>)] = [
                ("Display name", [kAXTextFieldRole as String, kAXTextAreaRole as String]),
                ("Save changes", [kAXButtonRole as String]), ("Apply reviewed draft", [kAXButtonRole as String]),
                ("Laboratory controls and evidence limits", [kAXDisclosureTriangleRole as String, kAXButtonRole as String]),
            ]
            for window in lastCapture.windows.indices {
                for (name, roles) in names {
                    let found = lastCapture.windows[window].filter { ($0.fact.title == name || $0.fact.description == name) && roles.contains($0.fact.role) }
                    for node in found.prefix(2) { controls.append(controlJSON(node, name).merging(["window": window]) { _, new in new }) }
                }
            }
            observe("\(label)-last-observed", lastCapture, selected: [], controls: controls)
        }
        throw LiveFailure.unavailable("wait:\(label):\(last)")
    }
    private func controlJSON(_ node: LiveNode, _ name: String) -> [String: Any] {
        var out: [String: Any] = ["name": name, "role": node.fact.role]
        if let value = node.fact.value { out["value"] = value }
        if let enabled = node.fact.enabled { out["enabled"] = enabled }
        if let actions = node.fact.actions { out["actions"] = actions }
        if let focused = node.fact.focused { out["focused"] = focused }
        if let expanded = node.fact.expanded { out["expanded"] = expanded }
        if let settable = node.fact.expandedSettable { out["expanded_settable"] = settable }
        return out
    }
    private func profile(_ capture: Capture, _ window: Int, value: String, saveName: String = "Save changes", saveEnabled: Bool) throws -> ([String], [[String: Any]]) {
        let field = try selected(capture, window, name: "Display name", roles: [kAXTextFieldRole as String, kAXTextAreaRole as String], operation: .setValue)
        guard field.fact.value == value else { throw WorkflowRefusal.missing("field-value:\(value)") }
        let saveFacts = facts(capture, window), indices = saveFacts.indices.filter { saveFacts[$0].title == saveName || saveFacts[$0].description == saveName }
        guard indices.count == 1 else { throw WorkflowRefusal.missing("save-control:\(saveName):\(indices.count)") }
        let save = capture.windows[window][indices[0]]
        guard save.fact.role == (kAXButtonRole as String), save.fact.enabled == saveEnabled else { throw WorkflowRefusal.disabled("save-enabled:\(saveName):\(saveEnabled)") }
        let oracle = try requireOracle(allTexts(capture, window), exact: [value])
        return (oracle, [controlJSON(field, "Display name"), controlJSON(save, saveName)])
    }
    private func setField(_ window: Int, _ value: String, label: String) throws {
        let roles: Set<String> = [kAXTextFieldRole as String, kAXTextAreaRole as String]
        let before = try self.capture(), focusTarget = try selected(before, window, name: "Display name", roles: roles, operation: .press)
        observe("\(label)-focus-before", before, selected: try requireOracle(allTexts(before, window), exact: ["Display name"]),
                controls: [controlJSON(focusTarget, "Display name")])
        try checkIdentityAndWindows(); command("\(label)-focus-command", window: window, name: "Display name", action: kAXPressAction as String)
        let pressResult = AXUIElementPerformAction(focusTarget.element, kAXPressAction as CFString)
        command("\(label)-focus-result", window: window, name: "Display name", action: kAXPressAction as String, result: pressResult)
        guard pressResult == .success else { throw LiveFailure.unavailable("field-press-failed:\(label):\(pressResult.rawValue)") }
        let capture = try self.capture(), node = try selected(capture, window, name: "Display name", roles: roles, operation: .setValue)
        guard CFEqual(focusTarget.element, node.element) else { throw LiveFailure.unavailable("field-changed-after-focus:\(label)") }
        observe("\(label)-focus-after", capture, selected: try requireOracle(allTexts(capture, window), exact: ["Display name"]),
                controls: [controlJSON(node, "Display name")])
        try checkIdentityAndWindows(); command("\(label)-set-command", window: window, name: "Display name", action: "AXSetValue")
        let result = AXUIElementSetAttributeValue(node.element, kAXValueAttribute as CFString, value as CFString)
        command("\(label)-set-result", window: window, name: "Display name", action: "AXSetValue", result: result)
        guard result == .success else { throw LiveFailure.unavailable("set-value-failed:\(label):\(result.rawValue)") }
        _ = try wait("\(label)-input") { try self.profile($0, window, value: value, saveEnabled: true) }
    }
    private func press(_ window: Int, _ name: String, roles: Set<String>, label: String) throws {
        let capture = try self.capture(), node = try selected(capture, window, name: name, roles: roles, operation: .press)
        try checkIdentityAndWindows(); command(label, window: window, name: name, action: kAXPressAction as String)
        let result = AXUIElementPerformAction(node.element, kAXPressAction as CFString)
        command("\(label)-result", window: window, name: name, action: kAXPressAction as String, result: result)
        guard result == .success else { throw LiveFailure.unavailable("press-failed:\(label):\(result.rawValue)") }
    }
    private func openDisclosure(_ window: Int) throws {
        let name = "Laboratory controls and evidence limits", capture = try self.capture()
        let diagnostic = try namedRole(capture, window, name: name, roles: [kAXDisclosureTriangleRole as String, kAXButtonRole as String])
        observe("disclosure-metadata", capture, selected: try requireOracle(allTexts(capture, window), exact: [name]), controls: [controlJSON(diagnostic, name)])
        var current = capture, node = diagnostic, needsPress = true
        if try disclosureScrollFallbackAction(node.fact, name: name) != nil {
            try checkIdentityAndWindows(); command("disclosure-scroll-command", window: window, name: name, action: publicScrollAction)
            let result = AXUIElementPerformAction(node.element, publicScrollAction as CFString)
            command("disclosure-scroll-result", window: window, name: name, action: publicScrollAction, result: result)
            guard result == .success else { throw LiveFailure.unavailable("disclosure-scroll-failed:\(result.rawValue)") }
            current = try self.capture()
            let after = try namedRole(current, window, name: name, roles: [kAXDisclosureTriangleRole as String, kAXButtonRole as String])
            needsPress = try scrolledDisclosureNeedsPress(CFEqual(node.element, after.element), after.fact, name: name)
            node = after
            observe("disclosure-scroll-after", current, selected: try requireOracle(allTexts(current, window), exact: [name]), controls: [controlJSON(node, name)])
        }
        // A scroll result is never opening evidence. The caller's exact enabled
        // fixture-control wait is the only allowed proof for this no-Press case.
        guard needsPress else { return }
        let selectedNode = try selected(current, window, name: name, roles: [kAXDisclosureTriangleRole as String, kAXButtonRole as String], operation: .disclosure)
        let allowed = (selectedNode.fact.actions ?? []).filter(publicDisclosureActions.contains)
        guard allowed.count == 1 else { throw LiveFailure.unavailable("disclosure-action-count:\((node.fact.actions ?? []).joined(separator: ","))") }
        try checkIdentityAndWindows(); command("disclosure-open-command", window: window, name: name, action: allowed[0])
        let result = AXUIElementPerformAction(selectedNode.element, allowed[0] as CFString)
        command("disclosure-open-result", window: window, name: name, action: allowed[0], result: result)
        guard result == .success else { throw LiveFailure.unavailable("disclosure-action-failed:\(allowed[0]):\(result.rawValue)") }
    }

    func run() throws -> [String: Any] {
        let button: Set<String> = [kAXButtonRole as String], link: Set<String> = [NSAccessibility.Role.link.rawValue]
        let baseline = try wait("baseline") { c in
            let a = try self.profile(c, 0, value: "Alex Morgan", saveEnabled: false), b = try self.profile(c, 1, value: "Alex Morgan", saveEnabled: false)
            let h0 = try requireOracle(self.allTexts(c, 0), exact: ["Your profile"]), h1 = try requireOracle(self.allTexts(c, 1), exact: ["Your profile"])
            return (a.0 + b.0 + h0 + h1, a.1 + b.1)
        }
        guard baseline.windows.count == 2 else { throw LiveFailure.unavailable("baseline-window-count") }
        try setField(0, String(repeating: "x", count: 81), label: "invalid")
        try press(0, "Save changes", roles: button, label: "invalid-save-command")
        _ = try wait("invalid-retained") { c in
            let base = try self.profile(c, 0, value: String(repeating: "x", count: 81), saveEnabled: true)
            let oracle = try requireOracle(self.allTexts(c, 0), exact: ["Changes need attention", "Some fields need attention. Your submitted draft is retained; nothing was saved.", "Enter a display name between 1 and 80 characters."])
            return (base.0 + oracle, base.1)
        }
        try setField(0, "Newer Workflow Casey", label: "newer")
        try press(0, "Save changes", roles: button, label: "newer-save-command")
        _ = try wait("newer-saved") { c in
            let base = try self.profile(c, 0, value: "Newer Workflow Casey", saveEnabled: false)
            return (base.0 + (try requireOracle(self.allTexts(c, 0), exact: ["Update", "Profile saved in this synthetic session only."])), base.1)
        }
        try setField(1, "Reviewed Workflow Avery", label: "reviewed-stale")
        try press(1, "Save changes", roles: button, label: "stale-save-command")
        _ = try wait("stale-conflict") { c in
            let base = try self.profile(c, 1, value: "Reviewed Workflow Avery", saveName: "Apply reviewed draft", saveEnabled: true)
            let oracle = try requireOracle(self.allTexts(c, 1), exact: ["Review a newer saved version", "Saved values changed after this form was opened. Your draft is retained. Review the current saved values before applying the draft again.", "Currently saved values", "Newer Workflow Casey"])
            return (base.0 + oracle, base.1)
        }
        try press(1, "Apply reviewed draft", roles: button, label: "apply-reviewed-command")
        _ = try wait("reviewed-saved") { c in
            let base = try self.profile(c, 1, value: "Reviewed Workflow Avery", saveEnabled: false)
            return (base.0 + (try requireOracle(self.allTexts(c, 1), exact: ["Update", "Profile saved in this synthetic session only."])), base.1)
        }
        try openDisclosure(1)
        _ = try wait("fixtures-exposed") { c in
            let check = try self.selected(c, 1, name: "Fail the next valid save once", roles: [kAXCheckBoxRole as String], operation: .press)
            let apply = try self.selected(c, 1, name: "Apply fixture", roles: button, operation: .press)
            return (try requireOracle(self.allTexts(c, 1), exact: ["Laboratory controls and evidence limits"]), [self.controlJSON(check, "Fail the next valid save once"), self.controlJSON(apply, "Apply fixture")])
        }
        try press(1, "Fail the next valid save once", roles: [kAXCheckBoxRole as String], label: "arm-failure-command")
        _ = try wait("failure-armed") { c in
            let check = try self.selected(c, 1, name: "Fail the next valid save once", roles: [kAXCheckBoxRole as String], operation: .press)
            guard check.fact.value == "1" || check.fact.value == "true" else { throw WorkflowRefusal.missing("checkbox-not-armed") }
            return (try requireOracle(self.allTexts(c, 1), exact: ["Fail the next valid save once"]), [self.controlJSON(check, "Fail the next valid save once")])
        }
        try press(1, "Apply fixture", roles: button, label: "apply-fixture-command")
        _ = try wait("fixture-applied") { c in
            let base = try self.profile(c, 1, value: "Reviewed Workflow Avery", saveEnabled: false)
            return (base.0 + (try requireOracle(self.allTexts(c, 1), exact: ["Simulation controls applied. This does not change real authorization or service availability."])), base.1)
        }
        try setField(1, "Retried Workflow Avery", label: "retry-draft")
        try press(1, "Save changes", roles: button, label: "simulated-failure-command")
        _ = try wait("simulated-failure-retained") { c in
            let base = try self.profile(c, 1, value: "Retried Workflow Avery", saveEnabled: true)
            let oracle = try requireOracle(self.allTexts(c, 1), exact: ["Changes need attention", "The one-shot save failure was simulated. Nothing was saved; retry the same draft to complete the save.", "No unsaved changes. You can retry the failed submission."])
            return (base.0 + oracle, base.1)
        }
        try press(1, "Save changes", roles: button, label: "retry-save-command")
        _ = try wait("retry-saved") { c in
            let base = try self.profile(c, 1, value: "Retried Workflow Avery", saveEnabled: false)
            return (base.0 + (try requireOracle(self.allTexts(c, 1), exact: ["Update", "Profile saved in this synthetic session only."])), base.1)
        }
        try press(0, "Activity", roles: link, label: "activity-command")
        _ = try wait("activity-page") { c in
            let connect = try self.selected(c, 0, name: "Connect to updates", roles: button, operation: .press)
            let stop = try self.namedRole(c, 0, name: "Stop watching", roles: button)
            guard stop.fact.enabled == false else { throw WorkflowRefusal.disabled("stop-expected-disabled") }
            let oracle = try requireOracle(self.allTexts(c, 0), exact: ["Watch this session", "Not watching. Showing the page-load snapshot. Connect or reload to check for changes."], contains: ["Page-load version 5 · Retried Workflow Avery"])
            return (oracle, [self.controlJSON(connect, "Connect to updates"), self.controlJSON(stop, "Stop watching")])
        }
        try press(0, "Connect to updates", roles: button, label: "connect-command")
        _ = try wait("watching-retried") { c in
            let stop = try self.selected(c, 0, name: "Stop watching", roles: button, operation: .press)
            let connect = try self.namedRole(c, 0, name: "Connect to updates", roles: button)
            guard connect.fact.enabled == false else { throw WorkflowRefusal.disabled("connect-expected-disabled") }
            let oracle = try requireOracle(self.allTexts(c, 0), exact: ["Watching for changes. This version is the last observed snapshot, not a freshness guarantee."], contains: ["Last observed version 5 · Retried Workflow Avery"])
            return (oracle, [self.controlJSON(connect, "Connect to updates"), self.controlJSON(stop, "Stop watching")])
        }
        try setField(1, "Live Workflow Avery", label: "live-draft")
        try press(1, "Save changes", roles: button, label: "live-save-command")
        _ = try wait("live-observed") { c in
            let profile = try self.profile(c, 1, value: "Live Workflow Avery", saveEnabled: false)
            let oracle = try requireOracle(self.allTexts(c, 0), exact: ["Watching for changes. This version is the last observed snapshot, not a freshness guarantee."], contains: ["Last observed version 6 · Live Workflow Avery"])
            return (profile.0 + oracle, profile.1)
        }
        try setField(1, "Unsubmitted workflow draft", label: "unsubmitted")
        try press(0, "Stop watching", roles: button, label: "stop-command")
        _ = try wait("watching-paused") { c in
            let profile = try self.profile(c, 1, value: "Unsubmitted workflow draft", saveEnabled: true)
            let connect = try self.selected(c, 0, name: "Connect to updates", roles: button, operation: .press)
            let stop = try self.namedRole(c, 0, name: "Stop watching", roles: button)
            guard stop.fact.enabled == false else { throw WorkflowRefusal.disabled("stop-expected-disabled") }
            return (profile.0 + (try requireOracle(self.allTexts(c, 0), exact: ["Watching paused. Displayed data may be stale. Connect again to request a new snapshot."])), profile.1 + [self.controlJSON(connect, "Connect to updates"), self.controlJSON(stop, "Stop watching")])
        }
        try press(0, "Connect to updates", roles: button, label: "reconnect-command")
        _ = try wait("watching-reconnected") { c in
            let profile = try self.profile(c, 1, value: "Unsubmitted workflow draft", saveEnabled: true)
            let stop = try self.selected(c, 0, name: "Stop watching", roles: button, operation: .press)
            let connect = try self.namedRole(c, 0, name: "Connect to updates", roles: button)
            guard connect.fact.enabled == false else { throw WorkflowRefusal.disabled("connect-expected-disabled") }
            let oracle = try requireOracle(self.allTexts(c, 0), exact: ["Watching for changes. This version is the last observed snapshot, not a freshness guarantee."], contains: ["Last observed version 6 · Live Workflow Avery"])
            return (profile.0 + oracle, profile.1 + [self.controlJSON(connect, "Connect to updates"), self.controlJSON(stop, "Stop watching")])
        }
        return ["schema": "appkit-native-workflow-ax-v1", "workflow_passed": true,
                "pid": Int(pid), "observed_executable": path, "window_count": 2,
                "limits": ["deadline_seconds": 45, "nodes": workflowMaxNodes, "children": workflowMaxChildren, "depth": workflowMaxDepth, "text_bytes": workflowMaxTextBytes],
                "steps": steps, "incomplete_capture_attempts": incompleteCaptures]
    }
    func failureReport(_ error: Error) -> [String: Any] {
        var report: [String: Any] = ["schema": "appkit-native-workflow-ax-v1", "workflow_passed": false, "pid": Int(pid),
         "observed_executable": path, "error": String(describing: error), "completed_and_attempted_steps": steps,
         "incomplete_capture_attempts": incompleteCaptures]
        if let failure = error as? ObservationReadFailure {
            var detail: [String: Any] = ["kind": failure.kind.rawValue, "operation": failure.operation,
                "attribute": failure.attribute, "ax_return_code": failure.code]
            if let window = failure.window { detail["window"] = window }
            if let nodes = failure.nodes { detail["nodes_before_failure"] = nodes }
            report["observation_failure"] = detail
        }
        return report
    }
}

private func emit(_ object: [String: Any]) throws {
    let data = try JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
    guard data.count + 1 <= workflowMaxOutputBytes else { throw LiveFailure.unavailable("output-limit") }
    FileHandle.standardOutput.write(data); FileHandle.standardOutput.write(Data([10]))
}
private func fail(_ message: String, code: Int32 = 1) -> Never {
    try? emit(["schema": "appkit-native-workflow-ax-v1", "workflow_passed": false, "error": message]); exit(code)
}
private func workflowMain() throws {
    guard CommandLine.arguments.count == 3, let rawPID = Int32(CommandLine.arguments[1]), rawPID > 1 else { throw LiveFailure.arguments }
    let path = CommandLine.arguments[2], url = URL(fileURLWithPath: path)
    guard path.hasPrefix("/"), url.standardizedFileURL.path == path, url.lastPathComponent == "gallery-native",
          url.deletingLastPathComponent().lastPathComponent.hasPrefix("appkit-native-companion-") else { throw LiveFailure.identity }
    let driver = try Driver(pid: pid_t(rawPID), path: path)
    do { try emit(driver.run()) } catch { try emit(driver.failureReport(error)); exit(1) }
}
@main struct NativeWorkflowDriverMain {
    static func main() {
        do { try workflowMain() }
        catch let error as LiveFailure { fail(error.description, code: error.description == "invalid-arguments" ? 2 : 1) }
        catch let error as WorkflowRefusal { fail(error.description) }
        catch { fail("workflow-unavailable") }
    }
}
#endif
