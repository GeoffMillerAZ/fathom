import Foundation

public protocol Shape: AnyObject {
    associatedtype Unit
    var area: Double { get }
    func draw(in context: Context) throws
    init(side: Int)
    subscript(index: Int) -> Int { get }
    static func unit() -> Self
}

public final class Box<T>: Container, Sendable where T: Hashable {
    typealias Item = T
    struct Inner {
        let raw: Int
    }
    enum Mode: String, CaseIterable {
        case fast, slow = "S"
        case custom(Int, label: String)
        indirect case chain
    }
}

actor Counter {
    var value = 0
    func bump() { value += 1 }
}

struct Point: Equatable {
    var x: Int
}

extension Box.Inner: Codable {
    func encoded() -> Data { Data() }
}

extension Array where Element == Int {
    var total: Int { reduce(0, +) }
}

#if os(iOS)
class Platform {}
#else
class Platform {}
#endif

typealias Handler = (Int) -> Void
