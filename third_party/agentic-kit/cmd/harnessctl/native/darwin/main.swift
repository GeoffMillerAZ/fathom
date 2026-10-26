import AppKit
import Darwin
import Foundation
import LocalAuthentication
import Security

private let planType = "application/vnd.agentic-kit.harness-maintenance-plan.v1+json"
private let planVersion = "harnessctl.maintenance-plan/1"
private let replyVersion = "harnessctl.native-approval/1"
private let stampVersion = "harnessctl.host-stamp/1"
private let planCheckVersion = "harnessctl.native-plan-check/1"
private let maxPlan = 65_536

private enum Failure: Error { case invalid, unavailable, canceled }

private func emit(_ object: [String: Any]) {
    guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]),
          data.count <= 65_536 else { _exit(2) }
    _ = data.withUnsafeBytes { write(STDOUT_FILENO, $0.baseAddress, data.count) }
    _ = write(STDOUT_FILENO, "\n", 1)
}

private func reply(_ status: String, signature: String = "", publicKey: String = "") {
    emit(["version": replyVersion, "status": status, "signature": signature, "publicKey": publicKey])
}

private func exactRead(_ count: Int, lifeline: Lifeline) throws -> Data {
    var result = Data()
    while result.count < count {
        guard !lifeline.isCanceled() else { throw Failure.canceled }
        var ready = pollfd(fd: STDIN_FILENO, events: Int16(POLLIN), revents: 0)
        let polled = poll(&ready, 1, 250)
        if polled == 0 { continue }
        if polled < 0 { throw Failure.canceled }
        var buffer = [UInt8](repeating: 0, count: min(4096, count - result.count))
        let n = read(STDIN_FILENO, &buffer, buffer.count)
        if n <= 0 { throw Failure.canceled }
        result.append(contentsOf: buffer.prefix(n))
    }
    return result
}

private func framedPlan(lifeline: Lifeline) throws -> Data {
    let prefix = try exactRead(4, lifeline: lifeline)
    let size = prefix.reduce(UInt32(0)) { ($0 << 8) | UInt32($1) }
    guard size > 0 && size <= UInt32(maxPlan) else { throw Failure.invalid }
    return try exactRead(Int(size), lifeline: lifeline)
}

// A small closed JSON parser: duplicate keys, floats, nulls, trailing bytes and
// malformed UTF-8 are rejected before a value can reach the review surface.
private struct Parser {
    let bytes: [UInt8]
    var at = 0
    init(_ data: Data) throws {
        guard String(bytes: data, encoding: .utf8) != nil else { throw Failure.invalid }
        bytes = Array(data)
    }
    mutating func spaces() { while at < bytes.count && [9, 10, 13, 32].contains(bytes[at]) { at += 1 } }
    mutating func take(_ b: UInt8) throws { spaces(); guard at < bytes.count && bytes[at] == b else { throw Failure.invalid }; at += 1 }
    mutating func string() throws -> String {
        spaces()
        let start = at
        try take(34)
        var escaped = false
        while at < bytes.count {
            let b = bytes[at]; at += 1
            if b == 34 && !escaped {
                let raw = Data(bytes[start..<at])
                guard let value = try? JSONSerialization.jsonObject(with: raw, options: .fragmentsAllowed) as? String else { throw Failure.invalid }
                return value
            }
            if b < 32 { throw Failure.invalid }
            if b == 92 && !escaped { escaped = true } else { escaped = false }
        }
        throw Failure.invalid
    }
    mutating func value(_ depth: Int = 0) throws -> Any {
        guard depth <= 24 else { throw Failure.invalid }
        spaces(); guard at < bytes.count else { throw Failure.invalid }
        switch bytes[at] {
        case 123:
            at += 1; spaces()
            var object: [String: Any] = [:]
            if at < bytes.count && bytes[at] == 125 { at += 1; return object }
            while true {
                let key = try string()
                guard object[key] == nil else { throw Failure.invalid }
                try take(58)
                object[key] = try value(depth + 1)
                spaces(); guard at < bytes.count else { throw Failure.invalid }
                let separator = bytes[at]; at += 1
                if separator == 125 { return object }
                guard separator == 44 else { throw Failure.invalid }
            }
        case 91:
            at += 1; spaces()
            var array: [Any] = []
            if at < bytes.count && bytes[at] == 93 { at += 1; return array }
            while true {
                array.append(try value(depth + 1))
                spaces(); guard at < bytes.count else { throw Failure.invalid }
                let separator = bytes[at]; at += 1
                if separator == 93 { return array }
                guard separator == 44 else { throw Failure.invalid }
            }
        case 34: return try string()
        case 48...57:
            let start = at
            if bytes[at] == 48 { at += 1 }
            else { while at < bytes.count && (48...57).contains(bytes[at]) { at += 1 } }
            let raw = String(decoding: bytes[start..<at], as: UTF8.self)
            guard let number = Int64(raw) else { throw Failure.invalid }
            return number
        default: throw Failure.invalid
        }
    }
    mutating func document() throws -> [String: Any] {
        guard let result = try value() as? [String: Any] else { throw Failure.invalid }
        spaces(); guard at == bytes.count else { throw Failure.invalid }
        return result
    }
}

private func fields(_ obj: [String: Any], _ expected: Set<String>) throws {
    guard Set(obj.keys) == expected else { throw Failure.invalid }
}
private func str(_ obj: [String: Any], _ key: String, max: Int = 512, nonempty: Bool = true) throws -> String {
    guard let value = obj[key] as? String, value.utf8.count <= max,
          (!nonempty || !value.isEmpty),
          !value.unicodeScalars.contains(where: {
              CharacterSet.controlCharacters.contains($0) ||
              $0.properties.generalCategory == .format ||
              $0.properties.generalCategory == .lineSeparator ||
              $0.properties.generalCategory == .paragraphSeparator
          }) else { throw Failure.invalid }
    return value
}
private func num(_ obj: [String: Any], _ key: String, min: Int64 = 0, max: Int64 = Int64.max) throws -> Int64 {
    guard let value = obj[key] as? Int64, value >= min && value <= max else { throw Failure.invalid }
    return value
}
private func id(_ obj: [String: Any], _ key: String) throws -> String {
    let value = try str(obj, key, max: 64)
    guard value.utf8.allSatisfy({ (65...90).contains($0) || (97...122).contains($0) || (48...57).contains($0) || $0 == 45 || $0 == 95 || $0 == 46 }) else { throw Failure.invalid }
    return value
}
private func hex(_ obj: [String: Any], _ key: String) throws {
    let value = try str(obj, key, max: 64)
    guard value.utf8.count == 64 && value.utf8.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) }) else { throw Failure.invalid }
}
private func utcSecond(_ value: String) throws -> Int64 {
    guard value.utf8.count == 20,
          value.range(of: #"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"#, options: .regularExpression) != nil else { throw Failure.invalid }
    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(secondsFromGMT: 0)
    formatter.dateFormat = "yyyy-MM-dd'T'HH:mm:ss'Z'"
    formatter.isLenient = false
    guard let date = formatter.date(from: value), formatter.string(from: date) == value,
          date.timeIntervalSince1970 >= 1,
          date.timeIntervalSince1970 <= Double(Int64.max) else { throw Failure.invalid }
    return Int64(date.timeIntervalSince1970)
}

private struct Review {
    let purpose: String
    let project: String
    let principal: String
    let approver: String
    let reason: String
    let duration: Int64
    let approvalExpiry: String
    let leaseExpiry: String
    let changes: [String]
}

private func validate(_ raw: Data, tag: String?, uid: String?) throws -> Review {
    var parser = try Parser(raw)
    let plan = try parser.document()
    try fields(plan, Set(["version", "purpose", "instanceId", "authorizationEpoch", "nonce", "controllerEpoch", "controllerSHA256", "registrationEpoch", "registrationSHA256", "snapshotSHA256", "rollbackSnapshotSHA256", "hostId", "projectId", "principalId", "approverId", "approverPrincipalId", "keyTag", "candidateSHA256", "bootId", "preparedUtcMicros", "preparedContinuousNanos", "issuedAt", "approvalExpiresAt", "leaseExpiresAt", "deadlineContinuousNanos", "durationSeconds", "reason", "controls"]))
    guard try str(plan, "version") == planVersion else { throw Failure.invalid }
    let purpose = try str(plan, "purpose")
    guard purpose == "maintenance" || purpose == "rearm-baseline" else { throw Failure.invalid }
    for key in ["instanceId", "nonce", "controllerSHA256", "registrationSHA256", "snapshotSHA256", "rollbackSnapshotSHA256", "candidateSHA256"] { try hex(plan, key) }
    for key in ["hostId", "projectId", "principalId", "approverId", "approverPrincipalId", "keyTag"] { _ = try id(plan, key) }
    let boot = try str(plan, "bootId", max: 128)
    guard boot.utf8.allSatisfy({ (33...126).contains($0) }) else { throw Failure.invalid }
    if let tag, try id(plan, "keyTag") != tag { throw Failure.invalid }
    if let uid, uid != String(getuid()) { throw Failure.invalid }
    _ = try num(plan, "authorizationEpoch", min: 1)
    _ = try num(plan, "controllerEpoch", min: 1, max: 2_147_483_647)
    _ = try num(plan, "registrationEpoch", min: 1, max: 2_147_483_647)
    let preparedContinuous = try num(plan, "preparedContinuousNanos")
    let preparedMicros = try num(plan, "preparedUtcMicros", min: 1)
    let issuedText = try str(plan, "issuedAt", max: 20)
    let issued = try utcSecond(issuedText)
    guard preparedMicros / 1_000_000 == issued else { throw Failure.invalid }
    let approvalText = try str(plan, "approvalExpiresAt", max: 20)
    let approval = try utcSecond(approvalText)
    guard approval > issued, approval - issued <= 300 else { throw Failure.invalid }
    let leaseText = try str(plan, "leaseExpiresAt", max: 20, nonempty: false)
    let lease = leaseText.isEmpty ? Int64(0) : try utcSecond(leaseText)
    let deadline = try num(plan, "deadlineContinuousNanos")
    let duration = try num(plan, "durationSeconds", max: 3600)
    let reason = try str(plan, "reason", max: 512)
    guard !reason.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { throw Failure.invalid }
    guard let controls = plan["controls"] as? [[String: Any]], controls.count <= 8 else { throw Failure.invalid }
    var changes: [String] = []
    var previous = ""
    for control in controls {
        try fields(control, ["id", "oldSamplePPM", "samplePPM"])
        let name = try id(control, "id")
        guard name > previous else { throw Failure.invalid }; previous = name
        let old = try num(control, "oldSamplePPM", max: 1_000_000)
        let new = try num(control, "samplePPM", max: 1_000_000)
        changes.append("\(name): \(old) → \(new) sample PPM")
    }
    if purpose == "maintenance" {
        guard duration > 0, !changes.isEmpty, lease >= approval,
              lease - issued <= duration, deadline > 0 else { throw Failure.invalid }
        let (leaseMicros, overflow1) = lease.multipliedReportingOverflow(by: 1_000_000)
        guard !overflow1 else { throw Failure.invalid }
        let (remainingMicros, overflow2) = leaseMicros.subtractingReportingOverflow(preparedMicros)
        guard !overflow2, remainingMicros > 0 else { throw Failure.invalid }
        let (remainingNanos, overflow3) = remainingMicros.multipliedReportingOverflow(by: 1_000)
        guard !overflow3 else { throw Failure.invalid }
        let (expectedDeadline, overflow4) = preparedContinuous.addingReportingOverflow(remainingNanos)
        guard !overflow4, deadline == expectedDeadline else { throw Failure.invalid }
    } else {
        guard duration == 0, changes.isEmpty, leaseText.isEmpty, deadline == 0 else { throw Failure.invalid }
    }
    return Review(purpose: purpose, project: try id(plan, "projectId"), principal: try id(plan, "principalId"), approver: try id(plan, "approverId"), reason: reason, duration: duration, approvalExpiry: approvalText, leaseExpiry: leaseText, changes: changes)
}

private final class Lifeline: NSObject {
    private let lock = NSLock()
    private var stopped = false
    private var canceled = false
    private var context: LAContext?
    private var alert: NSAlert?
    private var modalActive = false
    func deadline() {
        DispatchQueue.global().asyncAfter(deadline: .now() + 120) {
            self.cancel()
            _ = close(STDIN_FILENO)
        }
    }
    func start() {
        DispatchQueue.global(qos: .userInitiated).async {
            var byte: UInt8 = 0
            // The parent must hold stdin open until the complete response.
            let n = read(STDIN_FILENO, &byte, 1)
            if n >= 0 { self.cancel() }
        }
    }
    func bind(_ context: LAContext) { lock.lock(); self.context = context; let bad = canceled; lock.unlock(); if bad { context.invalidate() } }
    @objc private func endModalReview(_ alert: NSAlert) {
        lock.lock()
        let active = canceled && modalActive && self.alert === alert
        lock.unlock()
        let app = NSApplication.shared
        if active { app.abortModal() }
        alert.window.close()
    }
    private func scheduleModalEnd(_ alert: NSAlert) {
        performSelector(onMainThread: #selector(endModalReview(_:)), with: alert,
                        waitUntilDone: false,
                        modes: [RunLoop.Mode.modalPanel.rawValue, RunLoop.Mode.default.rawValue])
    }
    func bind(_ alert: NSAlert) { lock.lock(); self.alert = alert; let bad = canceled; lock.unlock(); if bad { scheduleModalEnd(alert) } }
    func beginModal(_ alert: NSAlert) -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard !canceled && self.alert === alert else { return false }
        modalActive = true
        return true
    }
    func unbind(_ alert: NSAlert) { lock.lock(); if self.alert === alert { modalActive = false; self.alert = nil }; lock.unlock() }
    func isCanceled() -> Bool { lock.lock(); defer { lock.unlock() }; return canceled }
    func finish() { lock.lock(); stopped = true; lock.unlock() }
    private func cancel() {
        lock.lock()
        if stopped { lock.unlock(); return }
        canceled = true
        let c = context; let a = alert
        lock.unlock()
        if let a { scheduleModalEnd(a) }
        c?.invalidate()
    }
}

private func review(_ plan: Review, preview: Bool, lifeline: Lifeline?) throws -> Bool {
    let app = NSApplication.shared
    app.setActivationPolicy(.accessory)
    let alert = NSAlert()
    alert.alertStyle = .warning
    alert.messageText = preview ? "DEMO PREVIEW — NO AUTHORIZATION" : "Review maintenance authorization"
    let changes = plan.changes.isEmpty ? "No sample-rate change. Restore the signed baseline only." : plan.changes.joined(separator: "\n")
    alert.informativeText = "Purpose: \(plan.purpose)\nProject: \(plan.project)\nPrincipal: \(plan.principal)\nApprover: \(plan.approver)\n\nExact changes:\n\(changes)\n\nDuration: \(plan.duration) seconds\nApproval expires: \(plan.approvalExpiry)\nLease expires: \(plan.leaseExpiry.isEmpty ? "none (baseline rearm)" : plan.leaseExpiry)\n\nMandatory checks and signed baseline remain unchanged. This proposal changes only the selected harnessctl receiver. It does not install or change command hooks.\n\nReason: \(plan.reason)"
    let detail = NSTextView(frame: NSRect(x: 0, y: 0, width: 520, height: 330))
    detail.string = alert.informativeText
    detail.isEditable = false
    detail.isSelectable = true
    detail.font = NSFont.monospacedSystemFont(ofSize: 12, weight: .regular)
    let scroll = NSScrollView(frame: detail.frame)
    scroll.documentView = detail
    scroll.hasVerticalScroller = true
    alert.informativeText = preview ? "Synthetic review only. This preview has no effects." : "Review every field before approving."
    alert.accessoryView = scroll
    if preview { alert.addButton(withTitle: "Close") }
    else { alert.addButton(withTitle: "Cancel"); alert.addButton(withTitle: "Approve") }
    if preview { alert.buttons[0].keyEquivalent = "\u{1b}" }
    else { alert.buttons[0].keyEquivalent = "\u{1b}"; alert.buttons[1].keyEquivalent = "" }
    lifeline?.bind(alert)
    guard lifeline?.isCanceled() != true else { throw Failure.canceled }
    app.activate(ignoringOtherApps: true)
    guard lifeline?.beginModal(alert) ?? true else { throw Failure.canceled }
    let answer = alert.runModal()
    lifeline?.unbind(alert)
    if lifeline?.isCanceled() == true { throw Failure.canceled }
    return !preview && answer == .alertSecondButtonReturn
}

private func protectedSignature(_ payload: Data, tag: String, expectedPoint: Data, approvalExpiry: String, lifeline: Lifeline) throws -> Data {
    let context = LAContext()
    context.touchIDAuthenticationAllowableReuseDuration = 0
    context.localizedCancelTitle = "Cancel"
    lifeline.bind(context)
    guard !lifeline.isCanceled(), context.canEvaluatePolicy(.deviceOwnerAuthentication, error: nil) else { throw Failure.unavailable }
    let sem = DispatchSemaphore(value: 0)
    var authenticated = false
    context.evaluatePolicy(.deviceOwnerAuthentication, localizedReason: "Sign the exact maintenance plan") { ok, _ in
        authenticated = ok
        sem.signal()
    }
    guard sem.wait(timeout: .now() + 120) == .success, !lifeline.isCanceled() else { context.invalidate(); throw Failure.canceled }
    guard authenticated else { throw Failure.canceled }
    guard try approvalWindowOpen(approvalExpiry) else { throw Failure.invalid }
    let query: [String: Any] = [kSecClass as String: kSecClassKey, kSecAttrApplicationTag as String: Data(tag.utf8), kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom, kSecReturnRef as String: true, kSecMatchLimit as String: kSecMatchLimitOne, kSecUseAuthenticationContext as String: context]
    var item: CFTypeRef?
    guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess, let item,
          CFGetTypeID(item) == SecKeyGetTypeID() else { throw Failure.unavailable }
    let key = item as! SecKey
    guard let attrs = SecKeyCopyAttributes(key) as? [String: Any],
          (attrs[kSecAttrTokenID as String] as? String) == (kSecAttrTokenIDSecureEnclave as String),
          (attrs[kSecAttrKeyClass as String] as? String) == (kSecAttrKeyClassPrivate as String),
          (attrs[kSecAttrKeyType as String] as? String) == (kSecAttrKeyTypeECSECPrimeRandom as String),
          (attrs[kSecAttrKeySizeInBits as String] as? Int) == 256,
          let pub = SecKeyCopyPublicKey(key),
          let point = SecKeyCopyExternalRepresentation(pub, nil) as Data?, point == expectedPoint else { throw Failure.unavailable }
    guard !lifeline.isCanceled(), SecKeyIsAlgorithmSupported(key, .sign, .ecdsaSignatureMessageX962SHA256) else { throw Failure.unavailable }
    let type = Data(planType.utf8)
    var pae = Data("DSSEv1 \(type.count) ".utf8)
    pae.append(type)
    pae.append(Data(" \(payload.count) ".utf8))
    pae.append(payload)
    guard try approvalWindowOpen(approvalExpiry) else { throw Failure.invalid }
    guard let signature = SecKeyCreateSignature(key, .ecdsaSignatureMessageX962SHA256, pae as CFData, nil) as Data? else { throw Failure.unavailable }
    guard !lifeline.isCanceled() else { throw Failure.canceled }
    return signature
}

private func stamp() throws {
    var size = 0
    guard sysctlbyname("kern.bootsessionuuid", nil, &size, nil, 0) == 0, size > 1, size <= 129 else { throw Failure.unavailable }
    var boot = [CChar](repeating: 0, count: size)
    guard sysctlbyname("kern.bootsessionuuid", &boot, &size, nil, 0) == 0 else { throw Failure.unavailable }
    guard boot.contains(0) else { throw Failure.unavailable }
    let bootID = String(cString: boot)
    guard !bootID.isEmpty, bootID.utf8.count <= 128, bootID.utf8.allSatisfy({ (33...126).contains($0) }) else { throw Failure.unavailable }
    var tv = timeval()
    guard gettimeofday(&tv, nil) == 0 else { throw Failure.unavailable }
    let (secNanos, secOverflow) = Int64(tv.tv_sec).multipliedReportingOverflow(by: 1_000_000)
    let (micros, addOverflow) = secNanos.addingReportingOverflow(Int64(tv.tv_usec))
    guard !secOverflow, !addOverflow, micros > 0 else { throw Failure.unavailable }
    var info = mach_timebase_info_data_t()
    guard mach_timebase_info(&info) == KERN_SUCCESS, info.denom != 0 else { throw Failure.unavailable }
    let ticks = mach_continuous_time()
    let (scaled, overflow) = ticks.multipliedReportingOverflow(by: UInt64(info.numer))
    guard !overflow else { throw Failure.unavailable }
    let nanos = scaled / UInt64(info.denom)
    guard nanos <= UInt64(Int64.max) else { throw Failure.unavailable }
    emit(["version": stampVersion, "os": "darwin", "osUserId": String(getuid()), "bootId": bootID, "utcMicros": micros, "continuousNanos": Int64(nanos)])
}

private func approvalWindowOpen(_ text: String) throws -> Bool {
    let seconds = try utcSecond(text)
    let (expiresMicros, overflow) = seconds.multipliedReportingOverflow(by: 1_000_000)
    guard !overflow else { throw Failure.invalid }
    var now = timeval()
    guard gettimeofday(&now, nil) == 0 else { throw Failure.unavailable }
    let (currentMicros, mulOverflow) = Int64(now.tv_sec).multipliedReportingOverflow(by: 1_000_000)
    let (wallMicros, addOverflow) = currentMicros.addingReportingOverflow(Int64(now.tv_usec))
    guard !mulOverflow, !addOverflow else { throw Failure.unavailable }
    return wallMicros < expiresMicros
}

private func run() throws {
    let args = Array(CommandLine.arguments.dropFirst())
    guard let verb = args.first else { throw Failure.invalid }
    if verb == "stamp" { guard args.count == 1 else { throw Failure.invalid }; try stamp(); return }
    if verb == "plan-check" {
        guard args.count == 1 else { throw Failure.invalid }
        let lifeline = Lifeline()
        lifeline.deadline()
        var received = 0
        var status = "invalid"
        if let raw = try? framedPlan(lifeline: lifeline) {
            received = raw.count
            if (try? validate(raw, tag: nil, uid: nil)) != nil { status = "valid" }
        }
        lifeline.finish()
        emit(["version": planCheckVersion, "status": status, "receivedBytes": received, "noAuthority": true])
        return
    }
    guard verb == "approve" || verb == "preview" else { throw Failure.invalid }
    let preview = verb == "preview"
    var tag: String?; var uid: String?; var point: Data?
    if preview { guard args.count == 1 else { throw Failure.invalid } }
    else {
        guard args.count == 7, args[1] == "--key-tag", args[3] == "--os-user-id", args[5] == "--public-key" else { throw Failure.invalid }
        tag = args[2]; uid = args[4]
        guard uid == String(getuid()), let raw = Data(base64Encoded: args[6]), raw.count == 65,
              raw.first == 4, raw.base64EncodedString() == args[6], !args[2].isEmpty else { throw Failure.invalid }
        point = raw
    }
    let lifeline = Lifeline()
    lifeline.deadline()
    let raw = try framedPlan(lifeline: lifeline)
    lifeline.start()
    let plan = try validate(raw, tag: tag, uid: uid)
    guard !lifeline.isCanceled() else { throw Failure.canceled }
    if !preview {
        guard try approvalWindowOpen(plan.approvalExpiry) else { throw Failure.invalid }
    }
    let approved = try review(plan, preview: preview, lifeline: lifeline)
    if preview { lifeline.finish(); reply("canceled"); return }
    guard approved else { lifeline.finish(); reply("denied"); return }
    guard let tag, let point else { throw Failure.invalid }
    guard try approvalWindowOpen(plan.approvalExpiry) else { throw Failure.invalid }
    let signature = try protectedSignature(raw, tag: tag, expectedPoint: point, approvalExpiry: plan.approvalExpiry, lifeline: lifeline)
    guard !lifeline.isCanceled() else { throw Failure.canceled }
    lifeline.finish()
    reply("signed", signature: signature.base64EncodedString(), publicKey: point.base64EncodedString())
}

do { try run() }
catch Failure.canceled { reply("canceled") }
catch Failure.unavailable { reply("unavailable") }
catch { reply("invalid") }
