import Foundation

public enum OlcRTCURIParserError: LocalizedError, Equatable {
    case unsupportedScheme
    case missingCarrier
    case missingFragment
    case missingKey
    case missingClientID

    public var errorDescription: String? {
        switch self {
        case .unsupportedScheme:
            "Only olcrtc:// links are supported."
        case .missingCarrier:
            "Carrier is missing in the olcRTC link."
        case .missingFragment:
            "Key/client fragment is missing in the olcRTC link."
        case .missingKey:
            "Encryption key is missing in the olcRTC link."
        case .missingClientID:
            "Client ID is missing in the olcRTC link."
        }
    }
}

public struct OlcRTCURIParser {
    public init() {}

    public func parse(_ rawValue: String, into profile: ConnectionProfile) throws -> ConnectionProfile {
        let value = rawValue.trimmingCharacters(in: .whitespacesAndNewlines)
        guard value.lowercased().hasPrefix("olcrtc://") else {
            throw OlcRTCURIParserError.unsupportedScheme
        }

        let mainPart = value.components(separatedBy: " / ").first ?? value
        let body = String(mainPart.dropFirst("olcrtc://".count))
        let carrierAndRest = body.split(separator: "?", maxSplits: 1, omittingEmptySubsequences: false)
        guard let carrierValue = carrierAndRest.first, !carrierValue.isEmpty else {
            throw OlcRTCURIParserError.missingCarrier
        }

        var parsed = profile
        if let carrier = Carrier(rawValue: String(carrierValue)) {
            parsed.carrier = carrier
        }

        guard carrierAndRest.count > 1 else {
            throw OlcRTCURIParserError.missingFragment
        }

        let transportRoomAndFragment = carrierAndRest[1].split(separator: "#", maxSplits: 1, omittingEmptySubsequences: false)
        guard transportRoomAndFragment.count == 2 else {
            throw OlcRTCURIParserError.missingFragment
        }

        parseTransportAndRoom(String(transportRoomAndFragment[0]), into: &parsed)
        try parseFragment(String(transportRoomAndFragment[1]), into: &parsed)

        if parsed.name == ConnectionProfile.empty.name || parsed.name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            parsed.name = "\(parsed.carrier.title) \(parsed.clientID)"
        }

        return parsed
    }

    private func parseTransportAndRoom(_ value: String, into profile: inout ConnectionProfile) {
        let parts = value.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false)
        let transportValue = parts.first.map(String.init) ?? ""
        let (transportName, parameters) = parseTransportValue(transportValue)

        if let transport = Transport(rawValue: transportName) {
            profile.transport = transport
        } else if transportName == "vp8" {
            profile.transport = .vp8channel
        } else if transportName == "dc" || transportName == "data" {
            profile.transport = .datachannel
        }

        if parts.count > 1 {
            profile.roomID = String(parts[1])
        }

        applyTransportParameters(parameters, to: &profile)
    }

    private func parseTransportValue(_ value: String) -> (name: String, parameters: [String: String]) {
        let parts = value.split(separator: "<", maxSplits: 1, omittingEmptySubsequences: false)
        let name = parts.first.map(String.init) ?? value
        guard parts.count == 2 else {
            return (name, [:])
        }

        let parameterText = parts[1].split(separator: ">", maxSplits: 1).first.map(String.init) ?? ""
        var parameters: [String: String] = [:]
        for pair in parameterText.split(separator: "&") {
            let keyValue = pair.split(separator: "=", maxSplits: 1, omittingEmptySubsequences: false)
            guard keyValue.count == 2 else {
                continue
            }
            let key = keyValue[0].trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
            let value = keyValue[1].trimmingCharacters(in: .whitespacesAndNewlines)
            parameters[key] = value
        }
        return (name, parameters)
    }

    private func applyTransportParameters(_ parameters: [String: String], to profile: inout ConnectionProfile) {
        if let value = parameters["vp8-fps"].flatMap(Int.init) {
            profile.vp8FPS = value
        }
        if let value = parameters["vp8-batch"].flatMap(Int.init) {
            profile.vp8BatchSize = value
        }
        if let value = parameters["fps"].flatMap(Int.init) {
            profile.seiFPS = value
        }
        if let value = parameters["batch"].flatMap(Int.init) {
            profile.seiBatchSize = value
        }
        if let value = parameters["frag"].flatMap(Int.init) {
            profile.seiFragmentSize = value
        }
        if let value = parameters["ack-ms"].flatMap(Int.init) {
            profile.seiAckTimeoutMillis = value
        }
    }

    private func parseFragment(_ value: String, into profile: inout ConnectionProfile) throws {
        let keyAndClient = value.split(separator: "%", maxSplits: 1, omittingEmptySubsequences: false)
        guard let key = keyAndClient.first, !key.isEmpty else {
            throw OlcRTCURIParserError.missingKey
        }
        guard keyAndClient.count > 1 else {
            throw OlcRTCURIParserError.missingClientID
        }

        profile.keyHex = String(key)

        let clientAndMeta = keyAndClient[1].split(separator: "$", maxSplits: 1, omittingEmptySubsequences: false)
        guard let clientID = clientAndMeta.first, !clientID.isEmpty else {
            throw OlcRTCURIParserError.missingClientID
        }

        profile.clientID = String(clientID)
    }
}
