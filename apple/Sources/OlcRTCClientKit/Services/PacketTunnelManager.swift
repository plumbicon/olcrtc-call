import Foundation

#if os(iOS)
import NetworkExtension

public enum PacketTunnelManagerError: LocalizedError {
    case providerBundleIdentifierMissing
    case providerDisconnected
    case startTimedOut

    public var errorDescription: String? {
        switch self {
        case .providerBundleIdentifierMissing:
            "Packet tunnel provider bundle identifier is missing."
        case .providerDisconnected:
            "The iOS VPN tunnel stopped before it reported a connection."
        case .startTimedOut:
            "Timed out while waiting for the iOS VPN tunnel to connect."
        }
    }
}

public final class PacketTunnelManager {
    private let providerBundleIdentifier: String
    private let localizedDescription: String

    public init(
        providerBundleIdentifier: String? = nil,
        localizedDescription: String = "Godwit"
    ) {
        self.providerBundleIdentifier = providerBundleIdentifier
            ?? Bundle.main.bundleIdentifier.map { "\($0).PacketTunnel" }
            ?? "community.openlibre.olcrtc.ios.PacketTunnel"
        self.localizedDescription = localizedDescription
    }

    public func start(
        profile: ConnectionProfile,
        eventHandler: ((String) async -> Void)? = nil
    ) async throws {
        let configuration = PacketTunnelConfiguration(profile: profile.normalizedForCurrentDefaults())
        await eventHandler?("Preparing iOS VPN configuration.")
        let manager = try await loadOrCreateManager()
        await eventHandler?("Saving iOS VPN configuration.")
        try await configure(manager: manager, configuration: configuration)
        await eventHandler?("Requesting iOS VPN tunnel start.")
        try manager.connection.startVPNTunnel(options: configuration.providerConfiguration)
        await eventHandler?("Waiting up to \(configuration.startTimeoutMillis / 1_000)s for iOS VPN tunnel.")
        try await waitUntilConnected(
            manager.connection,
            timeoutMillis: configuration.startTimeoutMillis,
            eventHandler: eventHandler
        )
    }

    public func stop() async {
        do {
            let managers = try await Self.loadAllManagers()
            managers
                .filter { manager in
                    (manager.protocolConfiguration as? NETunnelProviderProtocol)?
                        .providerBundleIdentifier == providerBundleIdentifier
                }
                .forEach { $0.connection.stopVPNTunnel() }
        } catch {
            return
        }
    }

    private func loadOrCreateManager() async throws -> NETunnelProviderManager {
        let managers = try await Self.loadAllManagers()
        if let manager = managers.first(where: { manager in
            (manager.protocolConfiguration as? NETunnelProviderProtocol)?
                .providerBundleIdentifier == providerBundleIdentifier
        }) {
            return manager
        }
        return NETunnelProviderManager()
    }

    private func configure(
        manager: NETunnelProviderManager,
        configuration: PacketTunnelConfiguration
    ) async throws {
        let tunnelProtocol = NETunnelProviderProtocol()
        tunnelProtocol.providerBundleIdentifier = providerBundleIdentifier
        tunnelProtocol.serverAddress = configuration.carrierName
        tunnelProtocol.providerConfiguration = configuration.providerMetadata
        tunnelProtocol.includeAllNetworks = true
        tunnelProtocol.excludeLocalNetworks = true
        tunnelProtocol.enforceRoutes = true

        manager.localizedDescription = localizedDescription
        manager.protocolConfiguration = tunnelProtocol
        manager.isEnabled = true

        try await save(manager)
        try await load(manager)
    }

    private func waitUntilConnected(
        _ connection: NEVPNConnection,
        timeoutMillis: Int,
        eventHandler: ((String) async -> Void)?
    ) async throws {
        let timeout = UInt64(max(timeoutMillis, 10_000)) * 1_000_000
        let startedAt = ContinuousClock.now
        let deadline = startedAt.advanced(by: .nanoseconds(timeout))
        let disconnectedGraceDeadline = startedAt.advanced(by: .seconds(5))
        var lastStatus: NEVPNStatus?
        var sawConnectionAttempt = false

        while ContinuousClock.now < deadline {
            let status = connection.status
            if status != lastStatus {
                await eventHandler?("iOS VPN status: \(Self.statusDescription(status)).")
                lastStatus = status
            }

            switch status {
            case .connected:
                return
            case .invalid:
                throw PacketTunnelManagerError.providerBundleIdentifierMissing
            case .disconnected:
                if sawConnectionAttempt || ContinuousClock.now >= disconnectedGraceDeadline {
                    throw PacketTunnelManagerError.providerDisconnected
                }
            case .disconnecting:
                if sawConnectionAttempt {
                    throw PacketTunnelManagerError.providerDisconnected
                }
            case .connecting, .reasserting:
                sawConnectionAttempt = true
            @unknown default:
                break
            }
            try await Task.sleep(nanoseconds: 250_000_000)
        }

        throw PacketTunnelManagerError.startTimedOut
    }

    private static func statusDescription(_ status: NEVPNStatus) -> String {
        switch status {
        case .invalid:
            "invalid"
        case .disconnected:
            "disconnected"
        case .connecting:
            "connecting"
        case .connected:
            "connected"
        case .reasserting:
            "reasserting"
        case .disconnecting:
            "disconnecting"
        @unknown default:
            "unknown"
        }
    }

    private static func loadAllManagers() async throws -> [NETunnelProviderManager] {
        try await withCheckedThrowingContinuation { continuation in
            NETunnelProviderManager.loadAllFromPreferences { managers, error in
                if let error {
                    continuation.resume(throwing: error)
                    return
                }
                continuation.resume(returning: managers ?? [])
            }
        }
    }

    private func save(_ manager: NETunnelProviderManager) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            manager.saveToPreferences { error in
                if let error {
                    continuation.resume(throwing: error)
                    return
                }
                continuation.resume(returning: ())
            }
        }
    }

    private func load(_ manager: NETunnelProviderManager) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            manager.loadFromPreferences { error in
                if let error {
                    continuation.resume(throwing: error)
                    return
                }
                continuation.resume(returning: ())
            }
        }
    }
}
#endif
