import Foundation

#if os(iOS)
import AVFoundation

public enum BackgroundRuntimeKeeperError: LocalizedError {
    case audioFormatUnavailable
    case audioBufferUnavailable

    public var errorDescription: String? {
        switch self {
        case .audioFormatUnavailable:
            "Unable to create background audio format."
        case .audioBufferUnavailable:
            "Unable to create background audio buffer."
        }
    }
}

@MainActor
public final class BackgroundRuntimeKeeper {
    private let engine = AVAudioEngine()
    private let player = AVAudioPlayerNode()
    private var didAttachPlayer = false
    private var isRunning = false
    private var loopBuffer: AVAudioPCMBuffer?

    public init() {}

    public func start() throws {
        guard !isRunning else { return }

        let session = AVAudioSession.sharedInstance()
        try session.setCategory(.playback, mode: .default, options: [.mixWithOthers])
        try session.setActive(true)

        let format = try makeAudioFormat()
        try configureGraph(format: format)

        guard let loopBuffer else {
            throw BackgroundRuntimeKeeperError.audioBufferUnavailable
        }

        player.scheduleBuffer(loopBuffer, at: nil, options: .loops)
        try engine.start()
        player.play()
        isRunning = true
    }

    public func stop() {
        guard isRunning else { return }

        player.stop()
        engine.stop()
        try? AVAudioSession.sharedInstance().setActive(false, options: [.notifyOthersOnDeactivation])
        isRunning = false
    }

    private func makeAudioFormat() throws -> AVAudioFormat {
        guard let format = AVAudioFormat(standardFormatWithSampleRate: 44_100, channels: 1) else {
            throw BackgroundRuntimeKeeperError.audioFormatUnavailable
        }
        return format
    }

    private func configureGraph(format: AVAudioFormat) throws {
        if !didAttachPlayer {
            engine.attach(player)
            engine.connect(player, to: engine.mainMixerNode, format: format)
            didAttachPlayer = true
        }

        if loopBuffer == nil {
            loopBuffer = try makeSilentLoopBuffer(format: format)
        }
    }

    private func makeSilentLoopBuffer(format: AVAudioFormat) throws -> AVAudioPCMBuffer {
        let frameCount = AVAudioFrameCount(format.sampleRate)
        guard let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: frameCount) else {
            throw BackgroundRuntimeKeeperError.audioBufferUnavailable
        }
        buffer.frameLength = frameCount
        return buffer
    }
}
#endif
