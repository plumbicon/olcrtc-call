import Foundation

public enum OlcRTCSubscriptionParserError: LocalizedError, Equatable {
    case noServers
    case invalidServer(line: Int, reason: String)

    public var errorDescription: String? {
        switch self {
        case .noServers:
            "Subscription does not contain any olcRTC server links."
        case .invalidServer(let line, let reason):
            "Subscription server on line \(line) is invalid: \(reason)"
        }
    }
}

public struct OlcRTCSubscriptionImport: Equatable {
    public var subscriptionID: UUID
    public var name: String
    public var profiles: [ConnectionProfile]

    public init(subscriptionID: UUID, name: String, profiles: [ConnectionProfile]) {
        self.subscriptionID = subscriptionID
        self.name = name
        self.profiles = profiles
    }
}

public struct OlcRTCSubscriptionParser {
    private let uriParser: OlcRTCURIParser

    public init(uriParser: OlcRTCURIParser = OlcRTCURIParser()) {
        self.uriParser = uriParser
    }

    public func parse(_ rawValue: String, sourceURL: URL? = nil) throws -> OlcRTCSubscriptionImport {
        var globalFields: [String: String] = [:]
        var servers: [ParsedServer] = []
        var currentServerIndex: Int?

        for (offset, rawLine) in rawValue.components(separatedBy: .newlines).enumerated() {
            let line = rawLine.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !line.isEmpty else {
                continue
            }

            if line.lowercased().hasPrefix("olcrtc://") {
                servers.append(ParsedServer(uri: line, fields: [:], lineNumber: offset + 1))
                currentServerIndex = servers.count - 1
                continue
            }

            if line.hasPrefix("##") {
                guard let currentServerIndex, let field = parseField(String(line.dropFirst(2))) else {
                    continue
                }
                servers[currentServerIndex].fields[field.key] = field.value
                continue
            }

            if line.hasPrefix("#"), let field = parseField(String(line.dropFirst())) {
                globalFields[field.key] = field.value
            }
        }

        guard !servers.isEmpty else {
            throw OlcRTCSubscriptionParserError.noServers
        }

        let subscriptionID = UUID()
        let subscriptionName = normalized(globalFields["name"]) ?? sourceURL?.host ?? "Imported subscription"
        let profiles = try servers.enumerated().map { index, server in
            var profile = ConnectionProfile.empty
            do {
                profile = try uriParser.parse(server.uri, into: profile)
            } catch {
                throw OlcRTCSubscriptionParserError.invalidServer(
                    line: server.lineNumber,
                    reason: error.localizedDescription
                )
            }

            let nodeName = normalized(server.fields["name"]) ?? normalized(profile.name) ?? "Node \(index + 1)"
            profile.id = UUID()
            profile.name = nodeName
            profile.subscription = SubscriptionMetadata(
                id: subscriptionID,
                name: subscriptionName,
                sourceURL: sourceURL?.absoluteString,
                updatedAtUnix: globalFields["update"].flatMap(TimeInterval.init),
                refreshInterval: normalized(globalFields["refresh"]),
                color: normalized(globalFields["color"]),
                icon: normalized(globalFields["icon"]),
                used: normalized(globalFields["used"]),
                available: normalized(globalFields["available"]),
                nodeColor: normalized(server.fields["color"]),
                nodeIcon: normalized(server.fields["icon"]),
                nodeUsed: normalized(server.fields["used"]),
                nodeAvailable: normalized(server.fields["available"]),
                nodeIP: normalized(server.fields["ip"]),
                nodeComment: normalized(server.fields["comment"]),
                nodeURI: server.uri
            )
            return profile
        }

        return OlcRTCSubscriptionImport(subscriptionID: subscriptionID, name: subscriptionName, profiles: profiles)
    }

    private func parseField(_ value: String) -> (key: String, value: String)? {
        let parts = value.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false)
        guard parts.count == 2 else {
            return nil
        }

        let key = parts[0].trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        let value = parts[1].trimmingCharacters(in: .whitespacesAndNewlines)
        guard !key.isEmpty else {
            return nil
        }
        return (key, value)
    }

    private func normalized(_ value: String?) -> String? {
        let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? nil : trimmed
    }
}

private struct ParsedServer {
    var uri: String
    var fields: [String: String]
    var lineNumber: Int
}
