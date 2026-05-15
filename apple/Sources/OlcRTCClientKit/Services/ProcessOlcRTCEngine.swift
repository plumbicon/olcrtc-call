import Foundation

#if os(macOS)
public final class ProcessOlcRTCEngine: OlcRTCEngine {
    private let eventPair = AsyncStream<String>.makeStream(of: String.self)
    private let lock = NSLock()
    private var process: Process?
    private var outputPipe: Pipe?
    private var outputBuffer = Data()
    private var ready = false
    private var stopping = false
    private var portConflictDetected = false
    private var retryCount = 0
    private var activePort: Int?
    private var lastOptions: OlcRTCStartOptions?
    private var lastSupportRoot: URL?
    private var lastCliURL: URL?
    private let maxPortRetries = 20

    public init() {}

    public var events: AsyncStream<String> {
        eventPair.stream
    }

    public var isRunning: Bool {
        get async {
            withLock { process?.isRunning == true }
        }
    }

    public var activeSocksPort: Int? {
        get async {
            withLock { activePort }
        }
    }

    public func start(options: OlcRTCStartOptions) async throws {
        try validate(options)

        let alreadyRunning = withLock { process?.isRunning == true }
        if alreadyRunning {
            throw OlcRTCEngineError.invalidProfile("olcRTC is already running.")
        }

        guard let supportRoot = supportRoot() else {
            throw OlcRTCEngineError.cliMissing("olcRTC support files were not found. Set OLCRTC_REPO_ROOT.")
        }
        guard let cliURL = cliURL(supportRoot: supportRoot) else {
            throw OlcRTCEngineError.cliMissing(
                "macOS CLI binary was not found. Run ./apple/Scripts/build-macos-cli.sh."
            )
        }

        withLock {
            lastOptions = options
            lastSupportRoot = supportRoot
            lastCliURL = cliURL
            activePort = options.socksPort
            retryCount = 0
        }

        try launchProcess(options: options, supportRoot: supportRoot, cliURL: cliURL, socksPort: options.socksPort)
    }

    public func waitReady(timeoutMillis: Int) async throws {
        let deadline = Date().addingTimeInterval(Double(timeoutMillis) / 1_000)
        while Date() < deadline {
            let state = withLock {
                (
                    isReady: ready,
                    isRunning: process?.isRunning == true,
                    portConflict: portConflictDetected,
                    activePort: activePort
                )
            }

            if state.isReady {
                return
            }
            if !state.isRunning {
                if state.portConflict,
                   let port = state.activePort,
                   try await retryAfterPortConflict(currentPort: port) {
                    continue
                }
                throw OlcRTCEngineError.invalidProfile("olcRTC exited before SOCKS became ready.")
            }

            try await Task.sleep(nanoseconds: 200_000_000)
        }

        throw OlcRTCEngineError.invalidProfile("olcRTC start timed out.")
    }

    public func stop() async {
        let task = withLock {
            stopping = true
            return process
        }

        guard let task, task.isRunning else {
            clearProcess()
            return
        }

        emit("Terminating olcRTC process.")
        task.terminate()

        await Task.detached {
            task.waitUntilExit()
        }.value

        clearProcess()
    }

    private func validate(_ options: OlcRTCStartOptions) throws {
        if !options.socksUser.isEmpty || !options.socksPass.isEmpty {
            throw OlcRTCEngineError.unsupportedOption("SOCKS username/password are available only through Mobile.xcframework.")
        }
        if options.clientID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            throw OlcRTCEngineError.invalidProfile("Client ID is required.")
        }
        if options.keyHex.count != 64 || !options.keyHex.allSatisfy(\.isHexDigit) {
            throw OlcRTCEngineError.invalidProfile("Encryption key must be 64 hexadecimal characters.")
        }
        if options.carrierName != "jazz" && options.roomID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            throw OlcRTCEngineError.invalidProfile("Room ID is required for this carrier.")
        }
    }

    private func arguments(options: OlcRTCStartOptions, supportRoot: URL, socksPort: Int) -> [String] {
        var args = [
            "-mode", "cnc",
            "-link", "direct",
            "-transport", options.transportName,
            "-carrier", options.carrierName,
            "-id", options.roomID,
            "-client-id", options.clientID,
            "-key", options.keyHex,
            "-socks-host", "127.0.0.1",
            "-socks-port", "\(socksPort)",
            "-dns", options.dnsServer,
            "-data", supportRoot.appendingPathComponent("data").path,
            "-vp8-fps", "\(options.vp8FPS)",
            "-vp8-batch", "\(options.vp8BatchSize)",
            "-fps", "\(options.seiFPS)",
            "-batch", "\(options.seiBatchSize)",
            "-frag", "\(options.seiFragmentSize)",
            "-ack-ms", "\(options.seiAckTimeoutMillis)",
            "-video-codec", options.videoCodec,
            "-video-w", "\(options.videoWidth)",
            "-video-h", "\(options.videoHeight)",
            "-video-fps", "\(options.videoFPS)",
            "-video-bitrate", options.videoBitrate,
            "-video-hw", options.videoHardwareAcceleration,
            "-video-qr-recovery", options.videoQRRecovery,
            "-video-qr-size", "\(options.videoQRSize)",
            "-video-tile-module", "\(options.videoTileModule)",
            "-video-tile-rs", "\(options.videoTileRS)",
        ]
        if options.debugLogging {
            args.append("-debug")
        }
        return args
    }

    private func launchProcess(options: OlcRTCStartOptions, supportRoot: URL, cliURL: URL, socksPort: Int) throws {
        let task = Process()
        let pipe = Pipe()
        task.executableURL = cliURL
        task.currentDirectoryURL = supportRoot
        task.arguments = arguments(options: options, supportRoot: supportRoot, socksPort: socksPort)
        task.standardOutput = pipe
        task.standardError = pipe

        pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            self?.handleOutput(data)
        }

        task.terminationHandler = { [weak self] process in
            self?.handleTermination(status: process.terminationStatus)
        }

        withLock {
            process = task
            outputPipe = pipe
            outputBuffer.removeAll(keepingCapacity: true)
            ready = false
            stopping = false
            portConflictDetected = false
            activePort = socksPort
        }

        emit("Launching \(cliURL.path)")
        emit(task.arguments?.joined(separator: " ") ?? "")
        try task.run()
    }

    private func retryAfterPortConflict(currentPort: Int) async throws -> Bool {
        let state = withLock {
            (
                options: lastOptions,
                supportRoot: lastSupportRoot,
                cliURL: lastCliURL,
                retries: retryCount
            )
        }

        guard let options = state.options,
              let supportRoot = state.supportRoot,
              let cliURL = state.cliURL,
              state.retries < maxPortRetries else {
            return false
        }

        let nextPort = currentPort == 65_535 ? 1 : currentPort + 1
        withLock {
            retryCount += 1
        }
        emit("Port \(currentPort) was rejected by macOS; retrying on \(nextPort).")
        try launchProcess(options: options, supportRoot: supportRoot, cliURL: cliURL, socksPort: nextPort)
        return true
    }

    private func handleOutput(_ data: Data) {
        let chunks = withLock {
            outputBuffer.append(data)
            return splitCompleteLines()
        }

        for chunk in chunks {
            guard let line = String(data: chunk, encoding: .utf8)?.trimmingCharacters(in: .newlines),
                  !line.isEmpty else {
                continue
            }
            emit(line)
            if line.contains("address already in use") || line.contains("failed to listen") {
                withLock {
                    portConflictDetected = true
                }
            }
            if line.contains("SOCKS5 server listening") {
                withLock {
                    ready = true
                }
            }
        }
    }

    private func splitCompleteLines() -> [Data] {
        var lines: [Data] = []
        while let range = outputBuffer.firstRange(of: Data([0x0A])) {
            lines.append(outputBuffer[..<range.lowerBound])
            outputBuffer.removeSubrange(...range.lowerBound)
        }
        return lines
    }

    private func handleTermination(status: Int32) {
        let state = withLock {
            let state = (wasStopping: stopping, remaining: outputBuffer)
            outputBuffer.removeAll(keepingCapacity: true)
            process = nil
            outputPipe?.fileHandleForReading.readabilityHandler = nil
            outputPipe = nil
            ready = false
            return state
        }

        if let line = String(data: state.remaining, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines),
           !line.isEmpty {
            emit(line)
        }
        if !state.wasStopping {
            emit("olcRTC process exited with status \(status).")
        }
    }

    private func clearProcess() {
        withLock {
            process = nil
            outputPipe?.fileHandleForReading.readabilityHandler = nil
            outputPipe = nil
            outputBuffer.removeAll(keepingCapacity: true)
            ready = false
            stopping = false
            portConflictDetected = false
            retryCount = 0
            activePort = nil
            lastOptions = nil
            lastSupportRoot = nil
            lastCliURL = nil
        }
    }

    private func emit(_ message: String) {
        eventPair.continuation.yield(message)
    }

    private func cliURL(supportRoot: URL) -> URL? {
        let environment = ProcessInfo.processInfo.environment
        if let value = environment["OLCRTC_CLI_PATH"], !value.isEmpty {
            return URL(fileURLWithPath: value)
        }

        let candidates = [
            Bundle.main.resourceURL?.appendingPathComponent("olcrtc-macos"),
            supportRoot.appendingPathComponent("../apple/.build/olcrtc-macos"),
            supportRoot.appendingPathComponent("build/olcrtc-darwin-arm64"),
            supportRoot.appendingPathComponent("build/olcrtc-darwin-amd64"),
            supportRoot.appendingPathComponent("build/olcrtc"),
        ]
        .compactMap { $0 }

        return candidates.first { FileManager.default.isExecutableFile(atPath: $0.path) }
    }

    private func supportRoot() -> URL? {
        let environment = ProcessInfo.processInfo.environment
        if let value = environment["OLCRTC_REPO_ROOT"], !value.isEmpty {
            return URL(fileURLWithPath: value)
        }

        let candidates = [
            Bundle.main.resourceURL,
            URL(fileURLWithPath: FileManager.default.currentDirectoryPath),
            Bundle.main.bundleURL,
        ]
        .compactMap { $0 }

        for candidate in candidates {
            if let root = walkUpForSupportRoot(from: candidate) {
                return root
            }
        }

        return nil
    }

    private func walkUpForSupportRoot(from url: URL) -> URL? {
        var current = url
        for _ in 0..<10 {
            let names = current.appendingPathComponent("data/names")
            if FileManager.default.fileExists(atPath: names.path) {
                return current
            }
            let nestedNames = current.appendingPathComponent("olcrtc/data/names")
            if FileManager.default.fileExists(atPath: nestedNames.path) {
                return current.appendingPathComponent("olcrtc")
            }
            current.deleteLastPathComponent()
        }
        return nil
    }

    private func withLock<T>(_ body: () -> T) -> T {
        lock.lock()
        defer { lock.unlock() }
        return body()
    }
}
#endif
