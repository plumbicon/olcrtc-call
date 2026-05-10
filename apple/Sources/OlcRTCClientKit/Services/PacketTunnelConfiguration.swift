import Foundation

public enum PacketTunnelConfigurationError: LocalizedError, Equatable {
    case missingValue(String)
    case invalidValue(String)

    public var errorDescription: String? {
        switch self {
        case .missingValue(let key):
            "Packet tunnel configuration is missing \(key)."
        case .invalidValue(let key):
            "Packet tunnel configuration has invalid \(key)."
        }
    }
}

public struct PacketTunnelConfiguration: Equatable {
    private enum Key {
        static let carrierName = "carrierName"
        static let transportName = "transportName"
        static let roomID = "roomID"
        static let clientID = "clientID"
        static let keyHex = "keyHex"
        static let socksPort = "socksPort"
        static let socksUser = "socksUser"
        static let socksPass = "socksPass"
        static let dnsServer = "dnsServer"
        static let debugLogging = "debugLogging"
        static let vp8FPS = "vp8FPS"
        static let vp8BatchSize = "vp8BatchSize"
        static let seiFPS = "seiFPS"
        static let seiBatchSize = "seiBatchSize"
        static let seiFragmentSize = "seiFragmentSize"
        static let seiAckTimeoutMillis = "seiAckTimeoutMillis"
        static let startTimeoutMillis = "startTimeoutMillis"
    }

    public var carrierName: String
    public var transportName: String
    public var roomID: String
    public var clientID: String
    public var keyHex: String
    public var socksPort: Int
    public var socksUser: String
    public var socksPass: String
    public var dnsServer: String
    public var debugLogging: Bool
    public var vp8FPS: Int
    public var vp8BatchSize: Int
    public var seiFPS: Int
    public var seiBatchSize: Int
    public var seiFragmentSize: Int
    public var seiAckTimeoutMillis: Int
    public var startTimeoutMillis: Int

    public init(profile: ConnectionProfile) {
        carrierName = profile.carrier.rawValue
        transportName = profile.transport.rawValue
        roomID = profile.roomID
        clientID = profile.clientID
        keyHex = profile.keyHex
        socksPort = profile.socksPort
        socksUser = profile.socksUser
        socksPass = profile.socksPass
        dnsServer = profile.dnsServer
        debugLogging = profile.debugLogging
        vp8FPS = profile.vp8FPS
        vp8BatchSize = profile.vp8BatchSize
        seiFPS = profile.seiFPS
        seiBatchSize = profile.seiBatchSize
        seiFragmentSize = profile.seiFragmentSize
        seiAckTimeoutMillis = profile.seiAckTimeoutMillis
        startTimeoutMillis = profile.startTimeoutMillis
    }

    public init(providerConfiguration: [String: Any]?, startOptions: [String: NSObject]?) throws {
        var values = providerConfiguration ?? [:]
        for (key, value) in startOptions ?? [:] {
            values[key] = value
        }

        carrierName = try Self.stringValue(Key.carrierName, from: values)
        transportName = try Self.stringValue(Key.transportName, from: values)
        roomID = try Self.stringValue(Key.roomID, from: values)
        clientID = try Self.stringValue(Key.clientID, from: values)
        keyHex = try Self.stringValue(Key.keyHex, from: values)
        socksPort = try Self.intValue(Key.socksPort, from: values)
        socksUser = Self.optionalStringValue(Key.socksUser, from: values)
        socksPass = Self.optionalStringValue(Key.socksPass, from: values)
        dnsServer = try Self.stringValue(Key.dnsServer, from: values)
        debugLogging = Self.boolValue(Key.debugLogging, from: values)
        vp8FPS = Self.optionalIntValue(Key.vp8FPS, from: values) ?? 60
        vp8BatchSize = Self.optionalIntValue(Key.vp8BatchSize, from: values) ?? 8
        seiFPS = Self.optionalIntValue(Key.seiFPS, from: values) ?? 60
        seiBatchSize = Self.optionalIntValue(Key.seiBatchSize, from: values) ?? 64
        seiFragmentSize = Self.optionalIntValue(Key.seiFragmentSize, from: values) ?? 900
        seiAckTimeoutMillis = Self.optionalIntValue(Key.seiAckTimeoutMillis, from: values) ?? 2_000
        startTimeoutMillis = Self.optionalIntValue(Key.startTimeoutMillis, from: values)
            ?? ConnectionProfile.defaultStartTimeoutMillis
    }

    public var providerConfiguration: [String: NSObject] {
        [
            Key.carrierName: carrierName as NSString,
            Key.transportName: transportName as NSString,
            Key.roomID: roomID as NSString,
            Key.clientID: clientID as NSString,
            Key.keyHex: keyHex as NSString,
            Key.socksPort: socksPort as NSNumber,
            Key.socksUser: socksUser as NSString,
            Key.socksPass: socksPass as NSString,
            Key.dnsServer: dnsServer as NSString,
            Key.debugLogging: debugLogging as NSNumber,
            Key.vp8FPS: vp8FPS as NSNumber,
            Key.vp8BatchSize: vp8BatchSize as NSNumber,
            Key.seiFPS: seiFPS as NSNumber,
            Key.seiBatchSize: seiBatchSize as NSNumber,
            Key.seiFragmentSize: seiFragmentSize as NSNumber,
            Key.seiAckTimeoutMillis: seiAckTimeoutMillis as NSNumber,
            Key.startTimeoutMillis: startTimeoutMillis as NSNumber,
        ]
    }

    public var providerMetadata: [String: NSObject] {
        [
            Key.carrierName: carrierName as NSString,
            Key.transportName: transportName as NSString,
            Key.roomID: roomID as NSString,
            Key.clientID: clientID as NSString,
            Key.socksPort: socksPort as NSNumber,
            Key.dnsServer: dnsServer as NSString,
            Key.debugLogging: debugLogging as NSNumber,
            Key.vp8FPS: vp8FPS as NSNumber,
            Key.vp8BatchSize: vp8BatchSize as NSNumber,
            Key.seiFPS: seiFPS as NSNumber,
            Key.seiBatchSize: seiBatchSize as NSNumber,
            Key.seiFragmentSize: seiFragmentSize as NSNumber,
            Key.seiAckTimeoutMillis: seiAckTimeoutMillis as NSNumber,
            Key.startTimeoutMillis: startTimeoutMillis as NSNumber,
        ]
    }

    public var connectionProfile: ConnectionProfile {
        ConnectionProfile(
            name: "Packet Tunnel",
            carrier: Carrier(rawValue: carrierName) ?? .wbstream,
            transport: Transport(rawValue: transportName) ?? .vp8channel,
            roomID: roomID,
            clientID: clientID,
            keyHex: keyHex,
            socksPort: socksPort,
            socksUser: socksUser,
            socksPass: socksPass,
            dnsServer: dnsServer,
            debugLogging: debugLogging,
            vp8FPS: vp8FPS,
            vp8BatchSize: vp8BatchSize,
            seiFPS: seiFPS,
            seiBatchSize: seiBatchSize,
            seiFragmentSize: seiFragmentSize,
            seiAckTimeoutMillis: seiAckTimeoutMillis,
            startTimeoutMillis: startTimeoutMillis
        )
    }

    private static func stringValue(_ key: String, from values: [String: Any]) throws -> String {
        guard let value = values[key] else {
            throw PacketTunnelConfigurationError.missingValue(key)
        }
        if let string = value as? String {
            return string
        }
        if let string = value as? NSString {
            return string as String
        }
        throw PacketTunnelConfigurationError.invalidValue(key)
    }

    private static func optionalStringValue(_ key: String, from values: [String: Any]) -> String {
        if let string = values[key] as? String {
            return string
        }
        if let string = values[key] as? NSString {
            return string as String
        }
        return ""
    }

    private static func intValue(_ key: String, from values: [String: Any]) throws -> Int {
        guard let value = optionalIntValue(key, from: values) else {
            if values[key] == nil {
                throw PacketTunnelConfigurationError.missingValue(key)
            }
            throw PacketTunnelConfigurationError.invalidValue(key)
        }
        return value
    }

    private static func optionalIntValue(_ key: String, from values: [String: Any]) -> Int? {
        if let int = values[key] as? Int {
            return int
        }
        if let number = values[key] as? NSNumber {
            return number.intValue
        }
        return nil
    }

    private static func boolValue(_ key: String, from values: [String: Any]) -> Bool {
        if let bool = values[key] as? Bool {
            return bool
        }
        if let number = values[key] as? NSNumber {
            return number.boolValue
        }
        return false
    }
}
