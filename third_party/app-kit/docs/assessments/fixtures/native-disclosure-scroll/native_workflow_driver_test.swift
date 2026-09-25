#if WORKFLOW_SELF_TEST
import Foundation
import AppKit
import ApplicationServices

private enum TestFailure: Error { case assertion(String) }
private func expect(_ condition: Bool, _ message: String) throws { if !condition { throw TestFailure.assertion(message) } }
private func fact(_ name: String, role: String = kAXButtonRole as String, enabled: Bool? = true,
                  settable: Bool? = false, actions: [String]? = [kAXPressAction as String], value: String? = nil) -> ControlFact {
    ControlFact(role: role, title: name, description: nil, value: value, enabled: enabled, settable: settable,
                actions: actions, focused: nil, expanded: nil, expandedSettable: nil)
}
private func refuses(_ expected: String, _ body: () throws -> Void) throws {
    do { try body(); throw TestFailure.assertion("missing refusal") }
    catch let error as WorkflowRefusal { try expect(error.description.hasPrefix(expected), "wrong refusal: \(error)") }
}
private func runTests() throws -> Int {
    var checks = 0
    func check(_ body: @autoclosure () throws -> Bool, _ message: String) throws { try expect(try body(), message); checks += 1 }
    let field = fact("Display name", role: kAXTextFieldRole as String, settable: true, actions: [], value: "Alex Morgan")
    try check(try selectExact([field], name: "Display name", roles: [kAXTextFieldRole as String], operation: .setValue) == 0, "set selector")
    try check(try selectExact([fact("Save changes")], name: "Save changes", roles: [kAXButtonRole as String], operation: .press) == 0, "press selector")
    try refuses("press-unsupported:") { _ = try selectExact([fact("Display name", role: kAXTextFieldRole as String, settable: true, actions: [])], name: "Display name", roles: [kAXTextFieldRole as String], operation: .press) }; checks += 1
    let disclosure = fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: [kAXPressAction as String])
    try check(try selectExact([disclosure], name: "Laboratory controls and evidence limits", roles: [kAXDisclosureTriangleRole as String], operation: .disclosure) == 0, "advertised disclosure")
    try check(try disclosureScrollFallbackAction(fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: ["AXScrollToVisible"]), name: "Laboratory controls and evidence limits") == "AXScrollToVisible", "scroll-only fallback")
    try check(try disclosureScrollFallbackAction(disclosure, name: "Laboratory controls and evidence limits") == nil, "press preferred over scroll")
    try refuses("disclosure-scroll-action-count:") { _ = try disclosureScrollFallbackAction(fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: []), name: "Laboratory controls and evidence limits") }; checks += 1
    try refuses("disclosure-scroll-action-count:") { _ = try disclosureScrollFallbackAction(fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: ["AXScrollToVisible", "AXScrollToVisible"]), name: "Laboratory controls and evidence limits") }; checks += 1
    try refuses("disabled-or-unreadable:") { _ = try disclosureScrollFallbackAction(fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, enabled: false, actions: ["AXScrollToVisible"]), name: "Laboratory controls and evidence limits") }; checks += 1
    try refuses("disclosure-changed-after-scroll:") { _ = try scrolledDisclosureNeedsPress(false, disclosure, name: "Laboratory controls and evidence limits") }; checks += 1
    try check(try scrolledDisclosureNeedsPress(true, fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: ["AXScrollToVisible"]), name: "Laboratory controls and evidence limits") == false, "post-scroll no-press needs fixture proof")
    try refuses("disclosure-action-count:") { _ = try scrolledDisclosureNeedsPress(true, fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: [kAXPressAction as String, kAXPressAction as String]), name: "Laboratory controls and evidence limits") }; checks += 1
    try refuses("missing:") { _ = try selectExact([], name: "Save changes", roles: [kAXButtonRole as String], operation: .press) }; checks += 1
    try refuses("duplicate:") { _ = try selectExact([fact("Activity", role: NSAccessibility.Role.link.rawValue), fact("Activity", role: NSAccessibility.Role.link.rawValue)], name: "Activity", roles: [NSAccessibility.Role.link.rawValue], operation: .press) }; checks += 1
    try check(try selectExact([fact("Profile", role: kAXHeadingRole as String), fact("Profile", role: NSAccessibility.Role.link.rawValue)], name: "Profile", roles: [NSAccessibility.Role.link.rawValue], operation: .press) == 1, "heading plus link")
    try refuses("duplicate:") { _ = try selectExact([fact("Profile", role: NSAccessibility.Role.link.rawValue), fact("Profile", role: NSAccessibility.Role.link.rawValue)], name: "Profile", roles: [NSAccessibility.Role.link.rawValue], operation: .press) }; checks += 1
    try refuses("role:") { _ = try selectExact([fact("Activity")], name: "Activity", roles: [NSAccessibility.Role.link.rawValue], operation: .press) }; checks += 1
    try refuses("disabled-or-unreadable:") { _ = try selectExact([fact("Save changes", enabled: false)], name: "Save changes", roles: [kAXButtonRole as String], operation: .press) }; checks += 1
    try refuses("not-settable:") { _ = try selectExact([fact("Display name", role: kAXTextFieldRole as String, settable: false, actions: [])], name: "Display name", roles: [kAXTextFieldRole as String], operation: .setValue) }; checks += 1
    try refuses("press-unsupported:") { _ = try selectExact([fact("Save changes", actions: [])], name: "Save changes", roles: [kAXButtonRole as String], operation: .press) }; checks += 1
    try refuses("disclosure-action-count:") { _ = try selectExact([fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: [])], name: "Laboratory controls and evidence limits", roles: [kAXDisclosureTriangleRole as String], operation: .disclosure) }; checks += 1
    try check(try selectExact([fact("Laboratory controls and evidence limits", role: kAXDisclosureTriangleRole as String, actions: [kAXPressAction as String, kAXConfirmAction as String])], name: "Laboratory controls and evidence limits", roles: [kAXDisclosureTriangleRole as String], operation: .disclosure) == 0, "one allowed advertised action")
    let invalid = ["Changes need attention", "Some fields need attention. Your submitted draft is retained; nothing was saved.", "Enter a display name between 1 and 80 characters."]
    try check(try requireOracle(invalid, exact: invalid) == invalid, "invalid oracle")
    try check(try requireOracle(["Last observed version 6 · Live Workflow Avery · editor"], contains: ["Last observed version 6 · Live Workflow Avery"]).count == 1, "live oracle")
    try refuses("oracle-exact:") { _ = try requireOracle(["Profile saved in this synthetic session only."], exact: ["The one-shot save failure was simulated. Nothing was saved; retry the same draft to complete the save."]) }; checks += 1
    try check(sameWindowSet([11, 22], [22, 11]), "window reorder")
    try check(!sameWindowSet([11, 22], [11, 33]), "window replacement")
    try check(!sameWindowSet([11, 11], [11, 11]), "window duplicate")
    try check(classifyObservationFailure(code: AXError.invalidUIElement.rawValue) == .transientRead, "invalid element transient")
    try check(classifyObservationFailure(code: AXError.cannotComplete.rawValue) == .transientRead, "cannot complete transient")
    try check(classifyObservationFailure(code: AXError.noValue.rawValue, noValueExpected: true) == .transientRead, "expected no value transient")
    try check(classifyObservationFailure(code: AXError.noValue.rawValue) == .unreadable, "unexpected no value unreadable")
    try check(!shouldRetryObservation(ObservationReadFailure(kind: .identity, operation: "identity", attribute: "process", code: 0, window: nil, nodes: nil)), "identity not retryable")
    try check(!shouldRetryObservation(ObservationReadFailure(kind: .limit, operation: "limit", attribute: "nodes", code: 0, window: nil, nodes: nil)), "limit not retryable")
    try check(!shouldRetryObservation(ObservationReadFailure(kind: .unreadable, operation: "copy-actions-type", attribute: "AXActionNames", code: 0, window: nil, nodes: nil)), "unreadable not success")
    // Planted false-pass: success text alone cannot satisfy the failed-save oracle.
    try refuses("oracle-exact:") { _ = try requireOracle(["Update", "Profile saved in this synthetic session only."], exact: ["Changes need attention", "The one-shot save failure was simulated. Nothing was saved; retry the same draft to complete the save."]) }; checks += 1
    return checks
}
@main struct NativeWorkflowDriverTests {
    static func main() {
        do {
            let result: [String: Any] = ["schema": "appkit-native-workflow-driver-self-test-v1", "passed": true, "checks": try runTests()]
            let data = try JSONSerialization.data(withJSONObject: result, options: [.sortedKeys]); FileHandle.standardOutput.write(data); FileHandle.standardOutput.write(Data([10]))
        } catch { FileHandle.standardError.write(Data("self-test failed: \(error)\n".utf8)); exit(1) }
    }
}
#endif
