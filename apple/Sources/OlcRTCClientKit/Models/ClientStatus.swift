import Foundation

public enum ClientStatus: Equatable {
    case stopped
    case starting
    case ready
    case stopping
    case failed(String)

    public var title: String {
        switch self {
        case .stopped: "Отключено"
        case .starting: "Подключение..."
        case .ready: "Подключено"
        case .stopping: "Отключение..."
        case .failed: "Ошибка"
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
