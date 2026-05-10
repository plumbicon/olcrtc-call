import Foundation

public enum ClientStatus: Equatable {
    case stopped
    case starting
    case ready
    case stopping
    case failed(String)

    public var title: String {
        switch self {
        case .stopped: "Stopped"
        case .starting: "Starting"
        case .ready: "Ready"
        case .stopping: "Stopping"
        case .failed: "Failed"
        }
    }

    public var isRunning: Bool {
        switch self {
        case .starting, .ready, .stopping:
            true
        case .stopped, .failed:
            false
        }
    }
}
