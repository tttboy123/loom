// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "LoomLocalApp",
    platforms: [
        .macOS(.v14),
    ],
    products: [
        .library(name: "LoomLocalAppCore", targets: ["LoomLocalAppCore"]),
        .library(name: "LoomLocalAppUI", targets: ["LoomLocalAppUI"]),
        .executable(name: "LoomLocalApp", targets: ["LoomLocalApp"]),
        .executable(
            name: "LoomLocalAppContractProbe",
            targets: ["LoomLocalAppContractProbe"]
        ),
    ],
    targets: [
        .target(name: "LoomLocalAppCore"),
        .target(
            name: "LoomLocalAppUI",
            dependencies: ["LoomLocalAppCore"]
        ),
        .executableTarget(
            name: "LoomLocalApp",
            dependencies: ["LoomLocalAppCore", "LoomLocalAppUI"]
        ),
        .executableTarget(
            name: "LoomLocalAppContractProbe",
            dependencies: ["LoomLocalAppCore"]
        ),
        .testTarget(
            name: "LoomLocalAppTests",
            dependencies: ["LoomLocalAppCore", "LoomLocalAppUI"]
        ),
    ],
    swiftLanguageModes: [.v5]
)
