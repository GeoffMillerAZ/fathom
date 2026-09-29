import Foundation

let defaultTimeout = 30
var (minValue, maxValue) = (0, 100)

public func topLevel(_ x: Int) -> Int { x }

macro stringify<T>(_ value: T) -> (T, String) = #externalMacro(module: "M", type: "T")

@discardableResult
public func configure<T: Hashable>(_ items: [T],
                                   using key: String = "id",
                                   completion: @escaping (T) -> Void) async throws -> [T]
    where T: Codable {
    func local() {}
    let scratch = 1
    return items
}

struct Vector: Equatable {
    var x: Double, y: Double
    private(set) var count = 0
    let (p, q) = (1, 2)
    static let zero = Vector(x: 0, y: 0)
    lazy var cache: [String: Int] = [:]

    var length: Double {
        get { sqrt(x * x + y * y) }
        set { x = newValue }
    }

    var observed: Int = 0 {
        didSet { notify() }
    }

    init(x: Double, y: Double) {
        self.x = x
        self.y = y
    }

    init?(raw: String) { return nil }

    subscript(i: Int) -> Double {
        get { i == 0 ? x : y }
    }

    static func == (lhs: Vector, rhs: Vector) -> Bool { lhs.x == rhs.x }
    static prefix func - (v: Vector) -> Vector { v }
    func `default`() {}
    mutating func scale(by factor: Double) -> Vector { self }
}

final class Node {
    deinit { release() }
    @objc func hook() {}
}

enum Level {
    case low, high
    var label: String { "level" }
    func next() -> Level { .high }
    static var all: [Level] { [.low, .high] }
}
