@main
struct App {
    @State private var count = 0
    @Published var name = ""
    static func main() { run() }
}

protocol Loader {
    func load() async throws
}

extension Loader {
    func reload() async throws { try await load() }
}

open class Base {
    public required init() {}
    open func work() {}
    class func make() -> Base { Base() }
}

func generic<T, U>(_ a: T, _ b: U) -> (T, U) where T: Equatable { (a, b) }
let 名前 = "n"
func 挨拶() {}

extension UnsafeMutablePointer<CInterop.FTSEnt> {
    var path: Int { count() }
}
extension [Header] {
    func headers() {}
}
extension Foo.Bar<Int> {
    static func make() {}
}

func == (lhs: Bar, rhs: Bar) -> Bool { true }
prefix func ! (value: Bar) -> Bar { value }
