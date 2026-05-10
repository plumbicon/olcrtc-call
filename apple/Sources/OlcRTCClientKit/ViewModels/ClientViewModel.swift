import Foundation

private enum RunningMode {
    case localProxy
    #if os(iOS)
    case packetTunnel
    #endif
}

@MainActor
public final class ClientViewModel: ObservableObject {
    @Published public private(set) var profiles: [ConnectionProfile]
    @Published public var selectedProfileID: UUID?
    @Published public var draft: ConnectionProfile
    @Published public private(set) var status: ClientStatus = .stopped
    @Published public private(set) var logs: [String] = []
    @Published public var useSystemProxy = true
    @Published public var selectedNetworkService = "Wi-Fi"
    @Published public private(set) var networkServices: [String] = ["Wi-Fi"]
    @Published public private(set) var isImporting = false
    @Published public private(set) var refreshingSubscriptionIDs: Set<UUID> = []

    private let engine: OlcRTCEngine
    private let store: ProfileStore
    private let uriParser: OlcRTCURIParser
    private let subscriptionParser: OlcRTCSubscriptionParser
    private let subscriptionFetcher: SubscriptionFetcher
    private let systemProxyManager: SystemProxyManager
    #if os(iOS)
    private let packetTunnelManager = PacketTunnelManager()
    #endif
    private var eventTask: Task<Void, Never>?
    private var startTask: Task<Void, Never>?
    private var importTask: Task<Void, Never>?
    private var refreshTasks: [UUID: Task<Void, Never>] = [:]
    private var runningMode: RunningMode?

    public init(
        engine: OlcRTCEngine = OlcRTCEngineFactory.makeDefault(),
        store: ProfileStore = ProfileStore(),
        uriParser: OlcRTCURIParser = OlcRTCURIParser(),
        subscriptionParser: OlcRTCSubscriptionParser? = nil,
        subscriptionFetcher: SubscriptionFetcher = SubscriptionFetcher(),
        systemProxyManager: SystemProxyManager = SystemProxyManager()
    ) {
        self.engine = engine
        self.store = store
        self.uriParser = uriParser
        self.subscriptionParser = subscriptionParser ?? OlcRTCSubscriptionParser(uriParser: uriParser)
        self.subscriptionFetcher = subscriptionFetcher
        self.systemProxyManager = systemProxyManager

        let loadedProfiles = store.loadProfiles().map { $0.normalizedForCurrentDefaults() }
        let selected = store.loadSelectedProfileID()
        let initialProfile = loadedProfiles.first(where: { $0.id == selected }) ?? loadedProfiles[0]

        profiles = loadedProfiles
        selectedProfileID = initialProfile.id
        draft = initialProfile

        observeEngineEvents()
        loadNetworkServices()
    }

    deinit {
        eventTask?.cancel()
        startTask?.cancel()
        importTask?.cancel()
        refreshTasks.values.forEach { $0.cancel() }
    }

    public var selectedProfileName: String {
        draft.name.isEmpty ? "Untitled profile" : draft.name
    }

    public var canStart: Bool {
        !status.isRunning && validationMessage == nil
    }

    public var validationMessage: String? {
        validate(profile: draft)
    }

    public func selectProfile(_ id: UUID?) {
        saveDraft()
        guard let id, let profile = profiles.first(where: { $0.id == id }) else {
            return
        }

        selectedProfileID = id
        draft = profile
        store.saveSelectedProfileID(id)
    }

    public func addProfile() {
        saveDraft()

        var profile = ConnectionProfile.empty
        profile.name = "Profile \(profiles.count + 1)"
        profiles.append(profile)
        selectedProfileID = profile.id
        draft = profile
        persistProfiles()
    }

    public func deleteProfiles(at offsets: IndexSet) {
        guard profiles.count > 1 else {
            return
        }

        let removedIDs = offsets.compactMap { profiles.indices.contains($0) ? profiles[$0].id : nil }
        for offset in offsets.sorted(by: >) {
            profiles.remove(at: offset)
        }
        store.deleteSecrets(profileIDs: removedIDs)
        if let selectedProfileID, profiles.contains(where: { $0.id == selectedProfileID }) {
            persistProfiles()
            return
        }

        let profile = profiles[0]
        self.selectedProfileID = profile.id
        draft = profile
        persistProfiles()
    }

    public func deleteProfiles(ids: [UUID]) {
        guard profiles.count > ids.count else {
            return
        }

        profiles.removeAll { ids.contains($0.id) }
        store.deleteSecrets(profileIDs: ids)

        if let selectedProfileID, profiles.contains(where: { $0.id == selectedProfileID }) {
            persistProfiles()
            return
        }

        let profile = profiles[0]
        selectedProfileID = profile.id
        draft = profile
        persistProfiles()
    }

    public func deleteSubscription(_ id: UUID) {
        let ids = profiles.compactMap { profile in
            profile.subscription?.id == id ? profile.id : nil
        }
        deleteProfiles(ids: ids)
    }

    public func refreshSubscription(_ id: UUID) {
        saveDraft()

        guard let metadata = profiles.compactMap(\.subscription).first(where: { $0.id == id }) else {
            appendLog("Subscription refresh failed: subscription was not found.")
            return
        }

        guard let sourceURLValue = metadata.sourceURL, let sourceURL = URL(string: sourceURLValue) else {
            appendLog("Subscription \(metadata.name) does not have a source URL.")
            return
        }

        refreshTasks[id]?.cancel()
        refreshTasks[id] = Task { [weak self] in
            guard let self else { return }
            refreshingSubscriptionIDs.insert(id)
            defer {
                refreshingSubscriptionIDs.remove(id)
                refreshTasks[id] = nil
            }

            do {
                let content = try await fetchSubscription(from: sourceURL)
                try refreshSubscription(content, sourceURL: sourceURL, existingSubscriptionID: id)
            } catch {
                appendLog("Subscription refresh failed: \(error.localizedDescription)")
            }
        }
    }

    public func importValue(_ value: String) {
        let value = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !value.isEmpty else {
            return
        }

        importTask?.cancel()
        importTask = Task { [weak self] in
            guard let self else { return }
            isImporting = true
            defer { isImporting = false }

            do {
                if let url = subscriptionURL(from: value) {
                    let content = try await fetchSubscription(from: url)
                    try importSubscription(content, sourceURL: url)
                    return
                }

                if value.lowercased().hasPrefix("olcrtc://") && !value.contains("\n") {
                    importURI(value)
                    return
                }

                try importSubscription(value, sourceURL: nil)
            } catch {
                appendLog("Import failed: \(error.localizedDescription)")
            }
        }
    }

    public func importURI(_ value: String) {
        do {
            saveDraft()
            var profile = try uriParser.parse(value, into: .empty)
            profile.id = UUID()
            profile.subscription = nil
            replacePlaceholderIfNeeded()
            profiles.append(profile)
            selectedProfileID = profile.id
            draft = profile
            persistProfiles()
            appendLog("Imported olcRTC profile link.")
        } catch {
            appendLog("Import failed: \(error.localizedDescription)")
        }
    }

    public func saveDraft() {
        guard let index = profiles.firstIndex(where: { $0.id == draft.id }) else {
            return
        }

        profiles[index] = draft
        persistProfiles()
    }

    public func start() {
        saveDraft()
        startTask?.cancel()

        var profileToStart = draft.normalizedForCurrentDefaults()
        let availableSocksPort = PortAvailability.nextAvailableTCPPort(startingAt: profileToStart.socksPort)
        if availableSocksPort != profileToStart.socksPort {
            appendLog("SOCKS port \(profileToStart.socksPort) is busy; using \(availableSocksPort).")
            profileToStart.socksPort = availableSocksPort
        }
        if profileToStart != draft {
            draft = profileToStart
            saveDraft()
        }

        if let validationMessage = validate(profile: profileToStart) {
            status = .failed(validationMessage)
            appendLog("Profile is incomplete: \(validationMessage)")
            return
        }

        #if os(iOS)
        if useSystemProxy {
            startPacketTunnel(profile: profileToStart)
            return
        }
        #endif

        let options = OlcRTCStartOptions(profile: profileToStart)
        status = .starting
        runningMode = .localProxy
        appendLog("Starting \(selectedProfileName).")

        startTask = Task { [weak self] in
            guard let self else { return }

            do {
                try await engine.start(options: options)
                try await engine.waitReady(
                    timeoutMillis: max(options.startTimeoutMillis, ConnectionProfile.defaultStartTimeoutMillis)
                )
                let activePort = await engine.activeSocksPort ?? options.socksPort
                if activePort != draft.socksPort {
                    draft.socksPort = activePort
                    saveDraft()
                }
                status = .ready
                appendLog("SOCKS proxy is ready at 127.0.0.1:\(activePort).")
                await enableSystemProxyIfNeeded(port: activePort)
            } catch {
                runningMode = nil
                status = .failed(error.localizedDescription)
                appendLog("Start failed: \(error.localizedDescription)")
                await engine.stop()
            }
        }
    }

    public func stop() {
        startTask?.cancel()
        status = .stopping
        appendLog("Stopping \(selectedProfileName).")

        Task { [weak self] in
            guard let self else { return }
            switch runningMode {
            #if os(iOS)
            case .packetTunnel:
                await packetTunnelManager.stop()
                appendLog("iOS VPN tunnel stopped.")
            #endif
            case .localProxy, nil:
                await disableSystemProxyIfNeeded()
                await engine.stop()
            }
            runningMode = nil
            status = .stopped
        }
    }

    public func clearLogs() {
        logs.removeAll()
    }

    private func persistProfiles() {
        store.saveProfiles(profiles)
        store.saveSelectedProfileID(selectedProfileID)
    }

    private func importSubscription(_ value: String, sourceURL: URL?) throws {
        saveDraft()

        let imported = try subscriptionParser.parse(value, sourceURL: sourceURL)
        let existingIDs: Set<UUID>
        if let sourceURL {
            existingIDs = Set(
                profiles.compactMap { profile in
                    profile.subscription?.sourceURL == sourceURL.absoluteString ? profile.id : nil
                }
            )
        } else {
            existingIDs = []
        }

        if !existingIDs.isEmpty {
            profiles.removeAll { existingIDs.contains($0.id) }
            store.deleteSecrets(profileIDs: Array(existingIDs))
        }

        replacePlaceholderIfNeeded()
        profiles.append(contentsOf: imported.profiles)
        if let firstProfile = imported.profiles.first {
            selectedProfileID = firstProfile.id
            draft = firstProfile
        }
        persistProfiles()
        appendLog("Imported subscription \(imported.name) with \(imported.profiles.count) server(s).")
    }

    private func refreshSubscription(_ value: String, sourceURL: URL, existingSubscriptionID: UUID) throws {
        saveDraft()

        let imported = try subscriptionParser.parse(value, sourceURL: sourceURL)
        let existingIndices = profiles.indices.filter { index in
            profiles[index].subscription?.id == existingSubscriptionID
        }
        let existingProfiles = existingIndices.map { profiles[$0] }
        guard let insertionIndex = existingIndices.min() else {
            try importSubscription(value, sourceURL: sourceURL)
            return
        }

        var existingByKey: [String: ConnectionProfile] = [:]
        for profile in existingProfiles {
            for key in subscriptionProfileKeys(profile) where existingByKey[key] == nil {
                existingByKey[key] = profile
            }
        }

        var matchedExistingIDs: Set<UUID> = []
        var refreshedProfiles: [ConnectionProfile] = []
        for importedProfile in imported.profiles {
            var profile = importedProfile
            profile.subscription?.id = existingSubscriptionID

            if let existingProfile = subscriptionProfileKeys(importedProfile).compactMap({ existingByKey[$0] }).first {
                profile = mergeImportedSubscriptionProfile(profile, preservingLocalSettingsFrom: existingProfile)
                matchedExistingIDs.insert(existingProfile.id)
            }

            refreshedProfiles.append(profile)
        }

        let deletedIDs = existingProfiles
            .map(\.id)
            .filter { !matchedExistingIDs.contains($0) }

        profiles.removeAll { profile in
            profile.subscription?.id == existingSubscriptionID
        }
        profiles.insert(contentsOf: refreshedProfiles, at: min(insertionIndex, profiles.count))

        if !deletedIDs.isEmpty {
            store.deleteSecrets(profileIDs: deletedIDs)
        }

        if let selectedProfileID, let selectedProfile = profiles.first(where: { $0.id == selectedProfileID }) {
            draft = selectedProfile
        } else if let firstProfile = refreshedProfiles.first {
            selectedProfileID = firstProfile.id
            draft = firstProfile
        }

        persistProfiles()
        appendLog(
            "Refreshed subscription \(imported.name): " +
            "\(matchedExistingIDs.count) updated, " +
            "\(refreshedProfiles.count - matchedExistingIDs.count) added, " +
            "\(deletedIDs.count) removed."
        )
    }

    private func mergeImportedSubscriptionProfile(
        _ importedProfile: ConnectionProfile,
        preservingLocalSettingsFrom existingProfile: ConnectionProfile
    ) -> ConnectionProfile {
        var profile = importedProfile
        profile.id = existingProfile.id
        profile.socksPort = existingProfile.socksPort
        profile.socksUser = existingProfile.socksUser
        profile.socksPass = existingProfile.socksPass
        profile.dnsServer = existingProfile.dnsServer
        profile.debugLogging = existingProfile.debugLogging
        profile.startTimeoutMillis = existingProfile.startTimeoutMillis
        return profile
    }

    private func subscriptionProfileKeys(_ profile: ConnectionProfile) -> [String] {
        let nodeURI = profile.subscription?.nodeURI?.trimmingCharacters(in: .whitespacesAndNewlines)
        let fullConnectionKey = [
            profile.carrier.rawValue,
            profile.transport.rawValue,
            profile.roomID,
            profile.clientID,
            profile.keyHex,
        ].joined(separator: "|")
        let connectionKey = [
            profile.carrier.rawValue,
            profile.transport.rawValue,
            profile.roomID,
            profile.clientID,
        ].joined(separator: "|")

        var keys: [String] = []
        if let nodeURI, !nodeURI.isEmpty {
            keys.append("uri:\(nodeURI)")
        }
        keys.append("connection-full:\(fullConnectionKey)")
        keys.append("connection:\(connectionKey)")
        return keys
    }

    private func replacePlaceholderIfNeeded() {
        guard profiles.count == 1, isEmptyPlaceholder(profiles[0]) else {
            return
        }

        let profileID = profiles[0].id
        profiles.removeAll()
        store.deleteSecrets(profileIDs: [profileID])
    }

    private func isEmptyPlaceholder(_ profile: ConnectionProfile) -> Bool {
        let empty = ConnectionProfile.empty
        var comparable = profile
        comparable.id = empty.id
        return comparable == empty
    }

    private func subscriptionURL(from value: String) -> URL? {
        if let url = URL(string: value),
           let scheme = url.scheme?.lowercased(),
           ["http", "https"].contains(scheme),
           url.host(percentEncoded: false) != nil {
            return url
        }

        guard !value.contains("\n"),
              !value.lowercased().hasPrefix("olcrtc://"),
              value.contains("."),
              let url = URL(string: "https://\(value)"),
              url.host(percentEncoded: false) != nil else {
            return nil
        }
        return url
    }

    private func fetchSubscription(from url: URL) async throws -> String {
        appendLog("Loading subscription from \(url.absoluteString).")
        do {
            return try await subscriptionFetcher.fetchWithURLSession(from: url)
        } catch {
            guard subscriptionFetcher.shouldRetryThroughResolvedEndpoint(error),
                  let host = url.host(percentEncoded: false),
                  url.scheme?.lowercased() == "https" else {
                throw error
            }

            appendLog("DNS lookup failed for \(host); retrying with DNS-over-HTTPS.")
            do {
                return try await subscriptionFetcher.fetchThroughResolvedEndpoint(from: url)
            } catch {
                appendLog("DNS-over-HTTPS retry failed: \(error.localizedDescription)")
                throw error
            }
        }
    }

    private func observeEngineEvents() {
        eventTask = Task { [weak self] in
            guard let self else { return }
            for await message in engine.events {
                appendLog(message)
            }
        }
    }

    private func appendLog(_ message: String) {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"
        logs.append("[\(formatter.string(from: Date()))] \(message)")
        if logs.count > 300 {
            logs.removeFirst(logs.count - 300)
        }
    }

    private func loadNetworkServices() {
        Task { [weak self] in
            guard let self else { return }
            let services = await systemProxyManager.networkServices()
            networkServices = services.isEmpty ? ["Wi-Fi"] : services
            if !networkServices.contains(selectedNetworkService) {
                selectedNetworkService = networkServices.first ?? "Wi-Fi"
            }
        }
    }

    #if os(iOS)
    private func startPacketTunnel(profile: ConnectionProfile) {
        status = .starting
        runningMode = .packetTunnel
        appendLog("Starting \(selectedProfileName) with iOS VPN.")

        startTask = Task { [weak self] in
            guard let self else { return }

            do {
                try await packetTunnelManager.start(profile: profile)
                status = .ready
                appendLog("iOS VPN tunnel is connected. System traffic is routed through olcRTC.")
            } catch {
                runningMode = nil
                status = .failed(error.localizedDescription)
                appendLog("VPN start failed: \(vpnStartFailureMessage(error))")
                await packetTunnelManager.stop()
            }
        }
    }

    private func vpnStartFailureMessage(_ error: Error) -> String {
        let message = error.localizedDescription
        guard message.localizedCaseInsensitiveContains("IPC failed") else {
            return message
        }

        #if targetEnvironment(simulator)
        return "\(message). Rebuild the simulator app with signing enabled so the Packet Tunnel extension gets simulated entitlements."
        #else
        return "\(message). Check that the app and Packet Tunnel extension profiles include the Network Extension packet-tunnel-provider entitlement."
        #endif
    }
    #endif

    private func enableSystemProxyIfNeeded(port: Int) async {
        #if os(macOS)
        guard useSystemProxy else {
            appendLog("System SOCKS proxy is off. Configure apps manually to use 127.0.0.1:\(port).")
            return
        }

        do {
            try await systemProxyManager.enable(service: selectedNetworkService, host: "127.0.0.1", port: port)
            appendLog("System SOCKS proxy enabled for \(selectedNetworkService) on 127.0.0.1:\(port).")
        } catch {
            appendLog("System proxy setup failed: \(error.localizedDescription)")
        }
        #else
        appendLog("iOS system traffic is not routed automatically. Configure apps manually to use 127.0.0.1:\(port).")
        #endif
    }

    private func disableSystemProxyIfNeeded() async {
        #if os(macOS)
        guard useSystemProxy else {
            return
        }

        do {
            try await systemProxyManager.disable(service: selectedNetworkService)
            appendLog("System SOCKS proxy disabled for \(selectedNetworkService).")
        } catch {
            appendLog("System proxy cleanup failed: \(error.localizedDescription)")
        }
        #endif
    }

    private func validate(profile: ConnectionProfile) -> String? {
        if profile.clientID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return "Client ID is required."
        }
        if profile.keyHex.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return "Encryption key is required."
        }
        if profile.keyHex.count != 64 || !profile.keyHex.allSatisfy(\.isHexDigit) {
            return "Encryption key must be 64 hexadecimal characters."
        }
        if profile.carrier != .jazz && profile.roomID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return "Room ID is required for this carrier."
        }
        if !(1...65_535).contains(profile.socksPort) {
            return "SOCKS port must be between 1 and 65535."
        }

        return nil
    }
}
