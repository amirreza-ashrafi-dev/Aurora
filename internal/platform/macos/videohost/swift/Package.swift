// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "VideoHost",
    platforms: [.macOS(.v13)],
    products: [.library(name: "VideoHost", type: .dynamic, targets: ["VideoHost"])],
    dependencies: [],
    targets: [
        .target(name: "VideoHost"),
    ],
    swiftLanguageModes: [.v5]
)
