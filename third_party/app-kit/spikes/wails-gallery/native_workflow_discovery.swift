import Foundation
import AppKit
import ApplicationServices

let maxWindows = 2, maxNodes = 512, maxChildren = 64, maxDepth = 32
let maxDisclosureActions = 16, maxDisclosureAttributes = 16
let maxTextBytes = 256, maxOutputBytes = 32 * 1024
let softDeadlineSeconds: TimeInterval = 3

struct DiscoveryNode {
    let window: Int
    let role, title, description, value: String?
    let enabled, settable, press: Bool?
    let unreadableName: Bool
}
struct DisclosureStructure {
    let window, node: Int
    let role, subrole: String?
    let enabled, expanded, expandedSettable: Bool?
    let actions, attributes: [String]
    let childCount: Int?
    let position, size: [Double]?
    let parentMatchesTraversal, windowMatches, recapturedSameElement: Bool?
}
struct DiscoveryInput {
    let windows: Int; let nodes: [DiscoveryNode]; let complete: Bool; let limitations: [String]
    let disclosure: DisclosureStructure?
    init(windows: Int, nodes: [DiscoveryNode], complete: Bool, limitations: [String], disclosure: DisclosureStructure? = nil) {
        self.windows = windows; self.nodes = nodes; self.complete = complete; self.limitations = limitations; self.disclosure = disclosure
    }
}
enum DiscoveryRefusal: Error, Equatable { case windows, nodes, windowIndex, text, disclosure }
private struct Target { let name: String; let roles: Set<String>; let setValue: Bool }
private let disclosureName = "Laboratory controls and evidence limits"
private let knownDisclosureActions: Set<String> = [kAXPressAction as String, kAXShowMenuAction as String, "AXScrollToVisible"]
private let knownDisclosureAttributes: Set<String> = [
    kAXRoleAttribute as String, kAXSubroleAttribute as String, kAXTitleAttribute as String,
    kAXDescriptionAttribute as String, kAXValueAttribute as String, kAXEnabledAttribute as String,
    kAXExpandedAttribute as String, kAXChildrenAttribute as String, kAXParentAttribute as String,
    kAXPositionAttribute as String, kAXSizeAttribute as String, kAXWindowAttribute as String,
]
private let requiredDisclosureAttributes: Set<String> = [
    kAXRoleAttribute as String, kAXEnabledAttribute as String, kAXExpandedAttribute as String, kAXParentAttribute as String,
    kAXPositionAttribute as String, kAXSizeAttribute as String, kAXWindowAttribute as String,
]
private let targets = [
    Target(name: "Display name", roles: [kAXTextFieldRole as String, kAXTextAreaRole as String], setValue: true),
    Target(name: "Save changes", roles: [kAXButtonRole as String], setValue: false),
    Target(name: "Apply reviewed draft", roles: [kAXButtonRole as String], setValue: false),
    Target(name: disclosureName, roles: [kAXDisclosureTriangleRole as String, kAXButtonRole as String], setValue: false),
    Target(name: "Fail the next valid save once", roles: [kAXCheckBoxRole as String], setValue: false),
    Target(name: "Apply fixture", roles: [kAXButtonRole as String], setValue: false),
    Target(name: "Profile", roles: [NSAccessibility.Role.link.rawValue], setValue: false),
    Target(name: "Activity", roles: [NSAccessibility.Role.link.rawValue], setValue: false),
    Target(name: "Connect to updates", roles: [kAXButtonRole as String], setValue: false),
    Target(name: "Stop watching", roles: [kAXButtonRole as String], setValue: false),
]

private func candidate(_ n: DiscoveryNode) -> [String: Any] {
    var out: [String: Any] = [:]
    if let v = n.role { out["role"] = v }; if let v = n.title { out["title"] = v }
    if let v = n.description { out["description"] = v }; if let v = n.value { out["value"] = v }
    if let v = n.enabled { out["enabled"] = v }; if let v = n.settable { out["value_settable"] = v }
    if let v = n.press { out["press_supported"] = v }
    return out
}

func evaluateDiscovery(_ input: DiscoveryInput) throws -> [String: Any] {
    guard (1...maxWindows).contains(input.windows) else { throw DiscoveryRefusal.windows }
    guard input.nodes.count <= maxNodes else { throw DiscoveryRefusal.nodes }
    for n in input.nodes {
        guard (0..<input.windows).contains(n.window) else { throw DiscoveryRefusal.windowIndex }
        for v in [n.role, n.title, n.description, n.value].compactMap({ $0 }) {
            guard v.utf8.count <= maxTextBytes else { throw DiscoveryRefusal.text }
        }
    }
    if let d = input.disclosure {
        guard (0..<input.windows).contains(d.window), (0..<input.nodes.count).contains(d.node),
              input.nodes[d.node].window == d.window,
              d.actions.count <= maxDisclosureActions, d.attributes.count <= maxDisclosureAttributes,
              d.actions.allSatisfy(knownDisclosureActions.contains),
              d.attributes.allSatisfy(knownDisclosureAttributes.contains),
              requiredDisclosureAttributes.isSubset(of: Set(d.attributes)),
              !Set(d.attributes).isDisjoint(with: [kAXTitleAttribute as String, kAXDescriptionAttribute as String]),
              d.actions.allSatisfy({ $0.utf8.count <= maxTextBytes }),
              d.attributes.allSatisfy({ $0.utf8.count <= maxTextBytes }),
              d.position.map({ $0.count == 2 && $0.allSatisfy(\.isFinite) }) ?? true,
              d.size.map({ $0.count == 2 && $0.allSatisfy({ $0.isFinite && $0 >= 0 }) }) ?? true else {
            throw DiscoveryRefusal.disclosure
        }
    }
    let complete = input.complete && input.limitations.isEmpty
    let controls: [[String: Any]] = targets.map { target in
        let windows: [[String: Any]] = (0..<input.windows).map { window in
            let all = input.nodes.filter { $0.window == window }
            let found = all.filter { $0.title == target.name || $0.description == target.name }
            let status: String
            if !complete { status = "incomplete-observation" }
            else if all.contains(where: { $0.unreadableName }) { status = "inconclusive-unreadable" }
            else if found.isEmpty { status = "not-exposed" }
            else if found.count > 1 { status = "ambiguous" }
            else if found[0].role.map(target.roles.contains) != true { status = "unsupported-role" }
            else if found[0].enabled == nil { status = "unreadable-enabled" }
            else if found[0].enabled == false { status = "disabled" }
            else if target.setValue { status = found[0].settable == nil ? "unreadable-settable" : (found[0].settable == true ? "unique-actionable" : "not-settable") }
            else { status = found[0].press == nil ? "unreadable-actions" : (found[0].press == true ? "unique-actionable" : "press-unsupported") }
            return ["window": window, "status": status, "candidates": found.map(candidate)]
        }
        return ["name": target.name, "windows": windows]
    }
    let disclosure: [String: Any]
    if !complete {
        disclosure = ["status": "incomplete-observation"]
    } else if let d = input.disclosure {
        let identityStatus: String
        if d.parentMatchesTraversal == nil || d.windowMatches == nil || d.recapturedSameElement == nil { identityStatus = "identity-unreadable" }
        else if d.parentMatchesTraversal == false || d.windowMatches == false || d.recapturedSameElement == false { identityStatus = "identity-mismatch" }
        else { identityStatus = "observed" }
        var observed: [String: Any] = ["status": identityStatus, "window": d.window, "node": d.node,
            "actions": d.actions, "known_attributes": d.attributes]
        if let v = d.role { observed["role"] = v }; if let v = d.subrole { observed["subrole"] = v }
        if let v = d.enabled { observed["enabled"] = v }; if let v = d.expanded { observed["expanded"] = v }
        if let v = d.expandedSettable { observed["expanded_settable"] = v }
        if let v = d.childCount { observed["child_count"] = v }; if let v = d.position { observed["position"] = v }
        if let v = d.size { observed["size"] = v }
        if let v = d.parentMatchesTraversal { observed["parent_matches_traversal"] = v }
        if let v = d.windowMatches { observed["window_matches"] = v }
        if let v = d.recapturedSameElement { observed["recaptured_same_element"] = v }
        disclosure = observed
    } else {
        disclosure = ["status": "not-observed"]
    }
    return ["schema": "appkit-native-workflow-discovery-v1", "observation_only": true, "complete": complete,
            "deadline_scope": "soft-between-AX-calls", "outer_timeout_required": true,
            "window_count": input.windows, "node_count": input.nodes.count,
            "limitations": input.limitations, "controls": controls, "laboratory_disclosure": disclosure]
}

#if !DISCOVERY_SELF_TEST
private enum LiveFailure: Error { case arguments, identity, trust, unavailable(String) }
private final class Traversal {
    private struct Item { let element: AXUIElement; let parent: AXUIElement?; let window, depth: Int }
    private var visited: [AXUIElement] = [], limits: [String] = []
    private let deadline = Date().addingTimeInterval(softDeadlineSeconds)
    private func raw(_ e: AXUIElement, _ a: String) -> (CFTypeRef?, Bool) {
        var v: CFTypeRef?; let r = AXUIElementCopyAttributeValue(e, a as CFString, &v)
        if r == .noValue || r == .attributeUnsupported { return (nil, false) }
        return r == .success && v != nil ? (v, false) : (nil, true)
    }
    private func text(_ e: AXUIElement, _ a: String) -> (String?, Bool) {
        let (v, bad) = raw(e, a); guard !bad else { return (nil, true) }; guard let v else { return (nil, false) }
        guard let s = v as? String, s.utf8.count <= maxTextBytes else { limits.append("text-limit"); return (nil, true) }
        return (s, false)
    }
    private func bool(_ e: AXUIElement, _ a: String) -> Bool? { let (v, bad) = raw(e, a); return bad ? nil : (v as? NSNumber)?.boolValue }
    private func settable(_ e: AXUIElement) -> Bool? {
        var v: DarwinBoolean = false; return AXUIElementIsAttributeSettable(e, kAXValueAttribute as CFString, &v) == .success ? v.boolValue : nil
    }
    private func settable(_ e: AXUIElement, _ attribute: String) -> Bool? {
        var v: DarwinBoolean = false
        return AXUIElementIsAttributeSettable(e, attribute as CFString, &v) == .success ? v.boolValue : nil
    }
    private func press(_ e: AXUIElement) -> Bool? {
        var v: CFArray?; guard AXUIElementCopyActionNames(e, &v) == .success, let names = v as? [String] else { return nil }
        return names.contains(kAXPressAction as String)
    }
    private func safeValue(_ e: AXUIElement, _ role: String?) -> String? {
        guard role == (kAXTextFieldRole as String) || role == (kAXTextAreaRole as String) || role == (kAXCheckBoxRole as String) else { return nil }
        let (v, bad) = raw(e, kAXValueAttribute as String); if bad { return nil }
        let s = (v as? String) ?? (v as? NSNumber)?.stringValue
        guard let s, s.utf8.count <= maxTextBytes else { if v != nil { limits.append("value-limit") }; return nil }; return s
    }
    private func children(_ e: AXUIElement) -> [AXUIElement]? {
        var count = 0; let r = AXUIElementGetAttributeValueCount(e, kAXChildrenAttribute as CFString, &count)
        if r == .noValue || r == .attributeUnsupported { return [] }
        guard r == .success, count >= 0, count <= maxChildren else { limits.append(count > maxChildren ? "children-limit" : "children-unreadable"); return nil }
        if count == 0 { return [] }
        var v: CFArray?; guard AXUIElementCopyAttributeValues(e, kAXChildrenAttribute as CFString, 0, count, &v) == .success,
                              let values = v as? [AnyObject] else { limits.append("children-unreadable"); return nil }
        var out: [AXUIElement] = []
        for value in values { let raw = value as CFTypeRef; guard CFGetTypeID(raw) == AXUIElementGetTypeID() else { limits.append("child-type"); return nil }; out.append(unsafeBitCast(raw, to: AXUIElement.self)) }
        return out
    }
    private func childCount(_ e: AXUIElement) -> Int? {
        var count = 0; let result = AXUIElementGetAttributeValueCount(e, kAXChildrenAttribute as CFString, &count)
        if result == .noValue || result == .attributeUnsupported { return nil }
        guard result == .success, count >= 0, count <= maxChildren else { limits.append(count > maxChildren ? "children-limit" : "children-unreadable"); return nil }
        return count
    }
    private func element(_ e: AXUIElement, _ attribute: String) -> AXUIElement? {
        let (value, bad) = raw(e, attribute); guard !bad, let value, CFGetTypeID(value) == AXUIElementGetTypeID() else { return nil }
        return unsafeBitCast(value, to: AXUIElement.self)
    }
    private func point(_ e: AXUIElement, _ attribute: String, _ type: AXValueType) -> [Double]? {
        let (value, bad) = raw(e, attribute); guard !bad, let value, CFGetTypeID(value) == AXValueGetTypeID() else { return nil }
        let ax = unsafeBitCast(value, to: AXValue.self)
        if type == .cgPoint { var p = CGPoint.zero; guard AXValueGetType(ax) == type, AXValueGetValue(ax, type, &p) else { return nil }; return [p.x, p.y] }
        var s = CGSize.zero; guard AXValueGetType(ax) == type, AXValueGetValue(ax, type, &s) else { return nil }; return [s.width, s.height]
    }
    private func names(_ e: AXUIElement, actions: Bool) -> [String]? {
        var values: CFArray?
        let result = actions ? AXUIElementCopyActionNames(e, &values) : AXUIElementCopyAttributeNames(e, &values)
        guard result == .success, let names = values as? [String] else { return nil }
        let bound = actions ? maxDisclosureActions : 128
        guard names.count <= bound, names.allSatisfy({ $0.utf8.count <= maxTextBytes }) else { limits.append(actions ? "disclosure-actions-limit" : "disclosure-attributes-limit"); return nil }
        return names
    }
    private func recapture(_ expected: AXUIElement, window: AXUIElement) -> Bool? {
        var queue = [Item(element: window, parent: nil, window: 0, depth: 0)], seen: [AXUIElement] = [], matches: [AXUIElement] = []; var index = 0
        let recaptureDeadline = Date().addingTimeInterval(softDeadlineSeconds)
        while index < queue.count {
            guard Date() < recaptureDeadline, seen.count < maxNodes else { limits.append("disclosure-recapture-limit"); return nil }
            let item = queue[index]; index += 1
            if seen.contains(where: { CFEqual($0, item.element) }) { continue }; seen.append(item.element)
            if item.depth > maxDepth { limits.append("disclosure-recapture-depth"); return nil }
            let role = text(item.element, kAXRoleAttribute as String).0
            let title = text(item.element, kAXTitleAttribute as String).0
            let description = text(item.element, kAXDescriptionAttribute as String).0
            if (title == disclosureName || description == disclosureName) && [kAXDisclosureTriangleRole as String, kAXButtonRole as String].contains(role ?? "") { matches.append(item.element) }
            guard let kids = children(item.element) else { return nil }
            queue += kids.map { Item(element: $0, parent: item.element, window: 0, depth: item.depth + 1) }
        }
        guard matches.count == 1 else { limits.append(matches.isEmpty ? "disclosure-recapture-missing" : "disclosure-recapture-ambiguous"); return nil }
        return CFEqual(matches[0], expected)
    }
    private func disclosure(_ item: Item, node: Int, windows: [AXUIElement]) -> DisclosureStructure? {
        guard let actionNames = names(item.element, actions: true), let attributeNames = names(item.element, actions: false) else { limits.append("disclosure-names-unreadable"); return nil }
        let unknownActions = actionNames.filter { !knownDisclosureActions.contains($0) }
        if !unknownActions.isEmpty { limits.append("disclosure-unknown-action"); return nil }
        let attributes = attributeNames.filter(knownDisclosureAttributes.contains).sorted()
        guard attributes.count <= maxDisclosureAttributes else { limits.append("disclosure-attributes-limit"); return nil }
        let role = text(item.element, kAXRoleAttribute as String).0
        let subrole = text(item.element, kAXSubroleAttribute as String).0
        let childCount = childCount(item.element)
        let parentMatches = item.parent.flatMap { parent in element(item.element, kAXParentAttribute as String).map { CFEqual($0, parent) } }
        let windowMatches = element(item.element, kAXWindowAttribute as String).map { CFEqual($0, windows[item.window]) }
        return DisclosureStructure(window: item.window, node: node, role: role, subrole: subrole,
            enabled: bool(item.element, kAXEnabledAttribute as String), expanded: bool(item.element, kAXExpandedAttribute as String),
            expandedSettable: settable(item.element, kAXExpandedAttribute as String), actions: actionNames.sorted(), attributes: attributes,
            childCount: childCount, position: point(item.element, kAXPositionAttribute as String, .cgPoint), size: point(item.element, kAXSizeAttribute as String, .cgSize),
            parentMatchesTraversal: parentMatches, windowMatches: windowMatches,
            recapturedSameElement: recapture(item.element, window: windows[item.window]))
    }
    func capture(_ windows: [AXUIElement]) -> DiscoveryInput {
        var queue = windows.enumerated().map { Item(element: $0.element, parent: nil, window: $0.offset, depth: 0) }, nodes: [DiscoveryNode] = []; var index = 0
        var disclosureItem: (Item, Int)?
        while index < queue.count {
            if Date() >= deadline { limits.append("time-limit"); break }; if nodes.count >= maxNodes { limits.append("node-limit"); break }
            let item = queue[index]; index += 1
            if visited.contains(where: { CFEqual($0, item.element) }) { continue }; visited.append(item.element)
            if item.depth > maxDepth { limits.append("depth-limit"); continue }
            let role = text(item.element, kAXRoleAttribute as String).0, title = text(item.element, kAXTitleAttribute as String), description = text(item.element, kAXDescriptionAttribute as String)
            let node = nodes.count
            nodes.append(DiscoveryNode(window: item.window, role: role, title: title.0, description: description.0,
                value: safeValue(item.element, role), enabled: bool(item.element, kAXEnabledAttribute as String),
                settable: settable(item.element), press: press(item.element), unreadableName: title.1 || description.1))
            if (title.0 == disclosureName || description.0 == disclosureName) && [kAXDisclosureTriangleRole as String, kAXButtonRole as String].contains(role ?? "") {
                if disclosureItem == nil { disclosureItem = (item, node) } else { limits.append("disclosure-ambiguous") }
            }
            if let kids = children(item.element) { queue += kids.map { Item(element: $0, parent: item.element, window: item.window, depth: item.depth + 1) } }
        }
        let structure = disclosureItem.flatMap { disclosure($0.0, node: $0.1, windows: windows) }
        let unique = Array(Set(limits)).sorted()
        return DiscoveryInput(windows: windows.count, nodes: nodes, complete: unique.isEmpty && index == queue.count, limitations: unique, disclosure: structure)
    }
}
private func emit(_ object: [String: Any]) throws {
    let data = try JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]); guard data.count <= maxOutputBytes else { throw LiveFailure.unavailable("output-limit") }
    FileHandle.standardOutput.write(data); FileHandle.standardOutput.write(Data([10]))
}
private func fail(_ error: String, _ code: Int32 = 1) -> Never { try? emit(["schema": "appkit-native-workflow-discovery-v1", "observation_only": true, "complete": false, "error": error]); exit(code) }
private func run() throws {
    guard CommandLine.arguments.count == 3, let rawPID = Int32(CommandLine.arguments[1]), rawPID > 1 else { throw LiveFailure.arguments }
    let path = CommandLine.arguments[2], url = URL(fileURLWithPath: path)
    guard path.hasPrefix("/"), url.standardizedFileURL.path == path,
          url.lastPathComponent == "gallery-native",
          url.deletingLastPathComponent().lastPathComponent.hasPrefix("appkit-native-companion-") else { throw LiveFailure.identity }
    let pid = pid_t(rawPID); guard let running = NSRunningApplication(processIdentifier: pid), !running.isTerminated, running.executableURL?.standardizedFileURL.path == path else { throw LiveFailure.identity }
    guard AXIsProcessTrusted() else { throw LiveFailure.trust }
    let app = AXUIElementCreateApplication(pid); guard AXUIElementSetMessagingTimeout(app, 1) == .success else { throw LiveFailure.unavailable("messaging-timeout") }
    var value: CFTypeRef?; guard AXUIElementCopyAttributeValue(app, kAXWindowsAttribute as CFString, &value) == .success,
                                let rawWindows = value as? [AnyObject], (1...maxWindows).contains(rawWindows.count) else { throw LiveFailure.unavailable("window-count") }
    let windows: [AXUIElement] = try rawWindows.map { let raw = $0 as CFTypeRef; guard CFGetTypeID(raw) == AXUIElementGetTypeID() else { throw LiveFailure.unavailable("window-type") }; return unsafeBitCast(raw, to: AXUIElement.self) }
    let input = Traversal().capture(windows)
    guard let rechecked = NSRunningApplication(processIdentifier: pid), !rechecked.isTerminated,
          rechecked.executableURL?.standardizedFileURL.path == path else { throw LiveFailure.identity }
    var report = try evaluateDiscovery(input)
    report["process_id"] = Int(pid); report["native_path"] = path; report["native_identity_rechecked"] = true
    try emit(report)
}
@main struct NativeWorkflowDiscoveryMain {
    static func main() {
        do { try run() } catch LiveFailure.arguments { fail("invalid-arguments", 2) } catch LiveFailure.identity { fail("native-identity-mismatch") }
        catch LiveFailure.trust { fail("accessibility-unavailable") } catch LiveFailure.unavailable(let reason) { fail(reason) }
        catch DiscoveryRefusal.windows { fail("window-count") } catch DiscoveryRefusal.nodes { fail("node-limit") }
        catch DiscoveryRefusal.windowIndex { fail("window-index") } catch DiscoveryRefusal.text { fail("text-limit") }
        catch DiscoveryRefusal.disclosure { fail("disclosure-structure") }
        catch { fail("discovery-unavailable") }
    }
}
#endif
