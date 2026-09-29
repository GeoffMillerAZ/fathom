// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "Fixture",
    targets: [
        .executableTarget(name: "App", dependencies: ["Core", "Net"]),
        .target(name: "Core"),
        .target(name: "Net"),
    ]
)
