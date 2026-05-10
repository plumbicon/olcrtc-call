// swift-tools-version: 5.9

import PackageDescription
import Foundation

let packageDirectory = URL(fileURLWithPath: #filePath).deletingLastPathComponent().path
let mobileFrameworkPath = "\(packageDirectory)/Frameworks/Mobile.xcframework"
let hasMobileFramework = FileManager.default.fileExists(atPath: mobileFrameworkPath)
let kitDependencies: [Target.Dependency] = hasMobileFramework
    ? [.target(name: "Mobile", condition: .when(platforms: [.iOS]))]
    : []
let mobileTargets: [Target] = hasMobileFramework
    ? [.binaryTarget(name: "Mobile", path: "Frameworks/Mobile.xcframework")]
    : []

let package = Package(
    name: "OlcRTCApple",
    platforms: [
        .iOS(.v16),
        .macOS(.v13),
    ],
    products: [
        .library(
            name: "OlcRTCClientKit",
            targets: ["OlcRTCClientKit"]
        ),
        .executable(
            name: "OlcRTCClientMac",
            targets: ["OlcRTCClientMac"]
        ),
    ],
    targets: [
        .target(
            name: "OlcRTCClientKit",
            dependencies: kitDependencies,
            linkerSettings: hasMobileFramework
                ? [.linkedLibrary("resolv", .when(platforms: [.iOS]))]
                : []
        ),
        .executableTarget(
            name: "OlcRTCClientMac",
            dependencies: ["OlcRTCClientKit"]
        ),
    ] + mobileTargets
)
