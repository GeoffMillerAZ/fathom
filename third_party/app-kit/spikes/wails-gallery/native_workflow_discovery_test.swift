#if DISCOVERY_SELF_TEST
import Foundation
import AppKit
import ApplicationServices

private enum Failure: Error { case assertion(String) }
private func require(_ condition: @autoclosure () -> Bool, _ message: String) throws { if !condition() { throw Failure.assertion(message) } }
private func node(_ name: String, _ role: String = kAXButtonRole as String, window: Int = 0,
                  enabled: Bool? = true, settable: Bool? = false, press: Bool? = true,
                  unreadable: Bool = false) -> DiscoveryNode {
    DiscoveryNode(window: window, role: role, title: unreadable ? nil : name, description: nil,
                  value: role == (kAXTextFieldRole as String) ? "Alex Morgan" : nil,
                  enabled: enabled, settable: settable, press: press, unreadableName: unreadable)
}
private func valid(_ window: Int = 0) -> [DiscoveryNode] { [
    node("Display name", kAXTextFieldRole as String, window: window, settable: true, press: false),
    node("Save changes", window: window), node("Apply reviewed draft", window: window),
    node("Laboratory controls and evidence limits", kAXDisclosureTriangleRole as String, window: window),
    node("Fail the next valid save once", kAXCheckBoxRole as String, window: window), node("Apply fixture", window: window),
    node("Profile", NSAccessibility.Role.link.rawValue, window: window), node("Activity", NSAccessibility.Role.link.rawValue, window: window),
    node("Connect to updates", window: window), node("Stop watching", window: window),
] }
private func disclosure(window: Int = 0, node: Int = 3, actions: [String] = ["AXScrollToVisible", kAXShowMenuAction as String],
                        attributes: [String]? = nil, parent: Bool? = true, boundWindow: Bool? = true,
                        recaptured: Bool? = true, position: [Double]? = [24, 320], size: [Double]? = [480, 44]) -> DisclosureStructure {
    let required = [kAXRoleAttribute, kAXTitleAttribute, kAXEnabledAttribute, kAXExpandedAttribute,
                    kAXChildrenAttribute, kAXParentAttribute, kAXPositionAttribute, kAXSizeAttribute, kAXWindowAttribute].map { $0 as String }
    return DisclosureStructure(window: window, node: node, role: kAXDisclosureTriangleRole as String, subrole: nil,
        enabled: true, expanded: false, expandedSettable: false, actions: actions, attributes: attributes ?? required,
        childCount: 0, position: position, size: size, parentMatchesTraversal: parent,
        windowMatches: boundWindow, recapturedSameElement: recaptured)
}
private func status(_ report: [String: Any], _ name: String, _ window: Int = 0) throws -> String {
    let controls = report["controls"] as? [[String: Any]] ?? [], control = controls.first { $0["name"] as? String == name }
    let windows = control?["windows"] as? [[String: Any]] ?? []
    guard let result = windows.first(where: { $0["window"] as? Int == window })?["status"] as? String else { throw Failure.assertion("missing \(name)") }
    return result
}
private func disclosureFact(_ report: [String: Any], _ name: String) throws -> Any {
    guard let structure = report["laboratory_disclosure"] as? [String: Any], let value = structure[name] else {
        throw Failure.assertion("missing disclosure fact \(name)")
    }
    return value
}
private func refused(_ expected: DiscoveryRefusal, _ body: () throws -> Void) throws {
    do { try body(); throw Failure.assertion("missing refusal") } catch let error as DiscoveryRefusal { try require(error == expected, "wrong refusal") }
}
private func tests() throws -> Int {
    var checks = 0
    func check(_ value: @autoclosure () throws -> Bool, _ message: String) throws {
        let passed = try value(); if !passed { throw Failure.assertion(message) }; checks += 1
    }
    let good = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure()))
    try check(good["laboratory_disclosure"] != nil, "disclosure structure")
    try check(try disclosureFact(good, "status") as? String == "observed", "disclosure identity")
    try check(try disclosureFact(good, "window") as? Int == 0 && disclosureFact(good, "node") as? Int == 3, "disclosure location")
    try check(try disclosureFact(good, "role") as? String == kAXDisclosureTriangleRole as String, "disclosure role")
    try check(try disclosureFact(good, "actions") as? [String] == ["AXScrollToVisible", kAXShowMenuAction as String], "disclosure actions")
    try check(try disclosureFact(good, "expanded") as? Bool == false && disclosureFact(good, "expanded_settable") as? Bool == false, "disclosure state")
    try check(try disclosureFact(good, "child_count") as? Int == 0, "disclosure children")
    try check(try disclosureFact(good, "position") as? [Double] == [24, 320] && disclosureFact(good, "size") as? [Double] == [480, 44], "disclosure geometry")
    let mismatch = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(recaptured: false)))
    try check(try disclosureFact(mismatch, "status") as? String == "identity-mismatch", "disclosure mismatch")
    let unreadableIdentity = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(parent: nil)))
    try check(try disclosureFact(unreadableIdentity, "status") as? String == "identity-unreadable", "disclosure identity unreadable")
    let missing = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: []))
    try check(try disclosureFact(missing, "status") as? String == "not-observed", "disclosure absent")
    let partial = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: false, limitations: ["time-limit"], disclosure: disclosure()))
    try check(try disclosureFact(partial, "status") as? String == "incomplete-observation", "disclosure incomplete")
    for name in ["Display name", "Save changes", "Apply reviewed draft", "Laboratory controls and evidence limits", "Fail the next valid save once", "Apply fixture", "Profile", "Activity", "Connect to updates", "Stop watching"] {
        try check(try status(good, name) == "unique-actionable", name)
    }
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid() + [node("Save changes")], complete: true, limitations: [])), "Save changes") == "ambiguous", "duplicate")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid().filter { $0.title != "Apply reviewed draft" }, complete: true, limitations: [])), "Apply reviewed draft") == "not-exposed", "hidden")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("", unreadable: true)], complete: true, limitations: [])), "Save changes") == "inconclusive-unreadable", "unreadable")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("Save changes"), node("", unreadable: true)], complete: true, limitations: [])), "Save changes") == "inconclusive-unreadable", "unreadable peer")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("Stop watching", enabled: false)], complete: true, limitations: [])), "Stop watching") == "disabled", "disabled")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("Display name", kAXTextFieldRole as String, settable: nil)], complete: true, limitations: [])), "Display name") == "unreadable-settable", "settable unreadable")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("Save changes", press: nil)], complete: true, limitations: [])), "Save changes") == "unreadable-actions", "actions unreadable")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("Profile", kAXStaticTextRole as String)], complete: true, limitations: [])), "Profile") == "unsupported-role", "role")
    try check(try status(evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: false, limitations: ["node-limit"])), "Save changes") == "incomplete-observation", "bounds")
    let two = try evaluateDiscovery(DiscoveryInput(windows: 2, nodes: valid(0) + valid(1), complete: true, limitations: []))
    try check(try status(two, "Profile", 0) == "unique-actionable" && status(two, "Profile", 1) == "unique-actionable", "windows")
    try refused(.windows) { _ = try evaluateDiscovery(DiscoveryInput(windows: 0, nodes: [], complete: true, limitations: [])) }; checks += 1
    try refused(.windows) { _ = try evaluateDiscovery(DiscoveryInput(windows: 3, nodes: [], complete: true, limitations: [])) }; checks += 1
    try refused(.nodes) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: Array(repeating: valid()[0], count: maxNodes + 1), complete: true, limitations: [])) }; checks += 1
    try refused(.windowIndex) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node("Save changes", window: 1)], complete: true, limitations: [])) }; checks += 1
    try refused(.text) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: [node(String(repeating: "x", count: maxTextBytes + 1))], complete: true, limitations: [])) }; checks += 1
    try refused(.disclosure) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(node: maxNodes))) }; checks += 1
    try refused(.disclosure) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(actions: ["AXInventedOpen"]))) }; checks += 1
    try refused(.disclosure) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(attributes: [kAXRoleAttribute as String]))) }; checks += 1
    let noName = [kAXRoleAttribute, kAXEnabledAttribute, kAXExpandedAttribute, kAXParentAttribute,
                  kAXPositionAttribute, kAXSizeAttribute, kAXWindowAttribute].map { $0 as String }
    try refused(.disclosure) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(attributes: noName))) }; checks += 1
    try refused(.disclosure) { _ = try evaluateDiscovery(DiscoveryInput(windows: 1, nodes: valid(), complete: true, limitations: [], disclosure: disclosure(position: [Double.nan, 1]))) }; checks += 1
    let data = try JSONSerialization.data(withJSONObject: good, options: [.sortedKeys])
    try check(data.count <= maxOutputBytes, "output"); try check(!String(decoding: data, as: UTF8.self).contains("launch"), "credential")
    return checks
}
@main struct NativeWorkflowDiscoveryTests {
    static func main() {
        do { let data = try JSONSerialization.data(withJSONObject: ["checks": try tests(), "passed": true], options: [.sortedKeys]); FileHandle.standardOutput.write(data); FileHandle.standardOutput.write(Data([10])) }
        catch { FileHandle.standardError.write(Data("self-test failed: \(error)\n".utf8)); exit(1) }
    }
}
#endif
