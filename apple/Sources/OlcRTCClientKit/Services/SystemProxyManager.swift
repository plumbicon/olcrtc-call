import Foundation

public enum SystemProxyError: LocalizedError {
    case commandFailed(String)

    public var errorDescription: String? {
        switch self {
        case .commandFailed(let message):
            message
        }
    }
}

public final class SystemProxyManager {
    public init() {}

    public func networkServices() async -> [String] {
        #if os(macOS)
        do {
            let output = try await runNetworkSetup(["-listallnetworkservices"])
            return output
                .split(separator: "\n")
                .dropFirst()
                .map { line in
                    line.trimmingCharacters(in: .whitespacesAndNewlines)
                }
                .filter { !$0.isEmpty }
                .map { service in
                    service.hasPrefix("*") ? String(service.dropFirst()) : service
                }
        } catch {
            return ["Wi-Fi"]
        }
        #else
        return []
        #endif
    }

    public func enable(service: String, host: String, port: Int) async throws {
        #if os(macOS)
        try await runNetworkSetup(["-setsocksfirewallproxy", service, host, "\(port)"])
        try await runNetworkSetup(["-setsocksfirewallproxystate", service, "on"])
        #else
        _ = service
        _ = host
        _ = port
        #endif
    }

    public func disable(service: String) async throws {
        #if os(macOS)
        try await runNetworkSetup(["-setsocksfirewallproxystate", service, "off"])
        #else
        _ = service
        #endif
    }

    @discardableResult
    private func runNetworkSetup(_ arguments: [String]) async throws -> String {
        #if os(macOS)
        try await Task.detached {
            let process = Process()
            let output = Pipe()
            process.executableURL = URL(fileURLWithPath: "/usr/sbin/networksetup")
            process.arguments = arguments
            process.standardOutput = output
            process.standardError = output

            try process.run()
            process.waitUntilExit()

            let data = output.fileHandleForReading.readDataToEndOfFile()
            let message = String(data: data, encoding: .utf8) ?? ""
            guard process.terminationStatus == 0 else {
                throw SystemProxyError.commandFailed(message.trimmingCharacters(in: .whitespacesAndNewlines))
            }

            return message
        }.value
        #else
        return ""
        #endif
    }
}
