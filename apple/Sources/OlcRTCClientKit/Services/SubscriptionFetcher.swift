import Foundation
import Network

public enum SubscriptionFetchError: LocalizedError {
    case unsupportedScheme(String)
    case invalidPort
    case hostMissing
    case dnsResolutionFailed(String)
    case malformedHTTPResponse
    case httpStatus(Int)
    case nonUTF8Body

    public var errorDescription: String? {
        switch self {
        case .unsupportedScheme(let scheme):
            "Unsupported subscription URL scheme: \(scheme)."
        case .invalidPort:
            "Subscription URL contains an invalid port."
        case .hostMissing:
            "Subscription URL is missing a host."
        case .dnsResolutionFailed(let host):
            "Could not resolve subscription host \(host)."
        case .malformedHTTPResponse:
            "Subscription server returned a malformed HTTP response."
        case .httpStatus(let statusCode):
            "Subscription server returned HTTP \(statusCode)."
        case .nonUTF8Body:
            "Subscription response is not valid UTF-8 text."
        }
    }
}

public struct SubscriptionFetcher {
    private let urlSession: URLSession

    public init(urlSession: URLSession = .shared) {
        self.urlSession = urlSession
    }

    public func fetch(from url: URL) async throws -> String {
        do {
            return try await fetchWithURLSession(from: url)
        } catch {
            guard shouldRetryThroughResolvedEndpoint(error), url.scheme?.lowercased() == "https" else {
                throw error
            }
            return try await fetchThroughResolvedEndpoint(from: url)
        }
    }

    public func shouldRetryThroughResolvedEndpoint(_ error: Error) -> Bool {
        containsURLError(error, matching: [.cannotFindHost])
    }

    func fetchWithURLSession(from url: URL) async throws -> String {
        let (data, response) = try await urlSession.data(from: url)
        if let response = response as? HTTPURLResponse, !(200...299).contains(response.statusCode) {
            throw SubscriptionFetchError.httpStatus(response.statusCode)
        }
        guard let content = String(data: data, encoding: .utf8) else {
            throw SubscriptionFetchError.nonUTF8Body
        }
        return content
    }

    func fetchThroughResolvedEndpoint(from url: URL) async throws -> String {
        guard let scheme = url.scheme?.lowercased() else {
            throw SubscriptionFetchError.unsupportedScheme("")
        }
        guard scheme == "https" else {
            throw SubscriptionFetchError.unsupportedScheme(scheme)
        }
        guard let host = url.host(percentEncoded: false), !host.isEmpty else {
            throw SubscriptionFetchError.hostMissing
        }
        let port = try endpointPort(for: url, defaultPort: 443)
        let addresses = try await resolveHostWithDNSOverHTTPS(host)
        var lastError: Error?

        for address in addresses {
            do {
                let data = try await HTTPSResolvedEndpointRequest(
                    url: url,
                    host: host,
                    port: port,
                    address: address
                ).load()
                guard let content = String(data: data, encoding: .utf8) else {
                    throw SubscriptionFetchError.nonUTF8Body
                }
                return content
            } catch {
                lastError = error
            }
        }

        throw lastError ?? SubscriptionFetchError.dnsResolutionFailed(host)
    }

    private func resolveHostWithDNSOverHTTPS(_ host: String) async throws -> [String] {
        async let ipv4 = resolveHostWithDNSOverHTTPS(host, type: "A", recordType: 1)
        async let ipv6 = resolveHostWithDNSOverHTTPS(host, type: "AAAA", recordType: 28)
        let addresses = try await ipv4 + ipv6
        if addresses.isEmpty {
            throw SubscriptionFetchError.dnsResolutionFailed(host)
        }
        return addresses
    }

    private func resolveHostWithDNSOverHTTPS(_ host: String, type: String, recordType: Int) async throws -> [String] {
        var components = URLComponents()
        components.scheme = "https"
        components.host = "1.1.1.1"
        components.path = "/dns-query"
        components.queryItems = [
            URLQueryItem(name: "name", value: host),
            URLQueryItem(name: "type", value: type),
        ]

        guard let url = components.url else {
            throw SubscriptionFetchError.dnsResolutionFailed(host)
        }

        var request = URLRequest(url: url, timeoutInterval: 10)
        request.setValue("application/dns-json", forHTTPHeaderField: "Accept")

        let (data, response) = try await urlSession.data(for: request)
        if let response = response as? HTTPURLResponse, !(200...299).contains(response.statusCode) {
            throw SubscriptionFetchError.httpStatus(response.statusCode)
        }

        let dnsResponse = try JSONDecoder().decode(DNSOverHTTPSResponse.self, from: data)
        guard dnsResponse.status == 0 else {
            return []
        }

        return (dnsResponse.answer ?? [])
            .filter { $0.type == recordType }
            .map(\.data)
            .filter { !$0.isEmpty }
    }

    private func endpointPort(for url: URL, defaultPort: UInt16) throws -> UInt16 {
        guard let port = url.port else {
            return defaultPort
        }
        guard (1...Int(UInt16.max)).contains(port), let endpointPort = UInt16(exactly: port) else {
            throw SubscriptionFetchError.invalidPort
        }
        return endpointPort
    }

    private func containsURLError(_ error: Error, matching codes: Set<URLError.Code>) -> Bool {
        if let urlError = error as? URLError, codes.contains(urlError.code) {
            return true
        }

        let nsError = error as NSError
        if nsError.domain == NSURLErrorDomain,
           codes.contains(URLError.Code(rawValue: nsError.code)) {
            return true
        }

        if let underlying = nsError.userInfo[NSUnderlyingErrorKey] as? Error {
            return containsURLError(underlying, matching: codes)
        }

        return false
    }
}

private struct DNSOverHTTPSResponse: Decodable {
    let status: Int
    let answer: [DNSOverHTTPSAnswer]?

    enum CodingKeys: String, CodingKey {
        case status = "Status"
        case answer = "Answer"
    }
}

private struct DNSOverHTTPSAnswer: Decodable {
    let type: Int
    let data: String
}

private final class HTTPSResolvedEndpointRequest {
    private let url: URL
    private let host: String
    private let port: UInt16
    private let address: String
    private let queue = DispatchQueue(label: "community.openlibre.olcrtc.subscription-fetch")
    private var connection: NWConnection?
    private var state: RequestState?

    init(url: URL, host: String, port: UInt16, address: String) {
        self.url = url
        self.host = host
        self.port = port
        self.address = address
    }

    func load() async throws -> Data {
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                start(continuation: continuation)
            }
        } onCancel: {
            connection?.cancel()
        }
    }

    private func start(continuation: CheckedContinuation<Data, Error>) {
        let tlsOptions = NWProtocolTLS.Options()
        sec_protocol_options_set_tls_server_name(tlsOptions.securityProtocolOptions, host)

        let parameters = NWParameters(tls: tlsOptions)
        let endpointPort = NWEndpoint.Port(rawValue: port) ?? .https
        let connection = NWConnection(host: NWEndpoint.Host(address), port: endpointPort, using: parameters)
        self.connection = connection

        let state = RequestState(continuation: continuation, connection: connection) { [weak self] in
            self?.state = nil
        }
        self.state = state
        connection.stateUpdateHandler = { [weak self, weak state] connectionState in
            guard let self, let state else {
                return
            }

            switch connectionState {
            case .ready:
                self.sendRequest(on: connection, state: state)
            case .failed(let error):
                state.finish(throwing: error)
            case .cancelled:
                state.finish(throwing: URLError(.cancelled))
            default:
                break
            }
        }
        queue.asyncAfter(deadline: .now() + 15) { [weak state] in
            state?.finish(throwing: URLError(.timedOut))
        }
        connection.start(queue: queue)
    }

    private func sendRequest(on connection: NWConnection, state: RequestState) {
        let request = [
            "GET \(requestPath) HTTP/1.1",
            "Host: \(hostHeader)",
            "User-Agent: olcRTC Apple",
            "Accept: text/plain, */*",
            "Connection: close",
            "",
            "",
        ].joined(separator: "\r\n")

        connection.send(content: Data(request.utf8), completion: .contentProcessed { [weak state] error in
            if let error {
                state?.finish(throwing: error)
            }
        })
        receiveNextChunk(on: connection, state: state)
    }

    private func receiveNextChunk(on connection: NWConnection, state: RequestState) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 64 * 1024) { [weak self, weak state] data, _, isComplete, error in
            guard let self, let state else {
                return
            }
            if let data, !data.isEmpty {
                state.append(data)
                do {
                    if let response = try self.completeResponse(state.data) {
                        state.finish(returning: response)
                        return
                    }
                } catch {
                    state.finish(throwing: error)
                    return
                }
            }
            if let error {
                state.finish(throwing: error)
                return
            }
            if isComplete {
                state.finish { try self.parseResponse(state.data) }
                return
            }
            self.receiveNextChunk(on: connection, state: state)
        }
    }

    private func completeResponse(_ data: Data) throws -> Data? {
        guard let separatorRange = data.range(of: Data("\r\n\r\n".utf8)) else {
            return nil
        }

        let headerData = data[..<separatorRange.lowerBound]
        let body = data[separatorRange.upperBound...]

        guard let headers = String(data: headerData, encoding: .isoLatin1) else {
            throw SubscriptionFetchError.malformedHTTPResponse
        }
        let headerLines = headers.components(separatedBy: "\r\n")
        guard let statusLine = headerLines.first else {
            throw SubscriptionFetchError.malformedHTTPResponse
        }

        try validateStatusLine(statusLine)

        guard let contentLength = headerLines.contentLength else {
            return nil
        }
        guard body.count >= contentLength else {
            return nil
        }

        return Data(body.prefix(contentLength))
    }

    private func parseResponse(_ data: Data) throws -> Data {
        guard let separatorRange = data.range(of: Data("\r\n\r\n".utf8)) else {
            throw SubscriptionFetchError.malformedHTTPResponse
        }

        let headerData = data[..<separatorRange.lowerBound]
        let body = data[separatorRange.upperBound...]

        guard let headers = String(data: headerData, encoding: .isoLatin1),
              let statusLine = headers.components(separatedBy: "\r\n").first else {
            throw SubscriptionFetchError.malformedHTTPResponse
        }

        try validateStatusLine(statusLine)
        return Data(body)
    }

    private func validateStatusLine(_ statusLine: String) throws {
        let statusParts = statusLine.split(separator: " ", maxSplits: 2)
        guard statusParts.count >= 2, let statusCode = Int(statusParts[1]) else {
            throw SubscriptionFetchError.malformedHTTPResponse
        }
        guard (200...299).contains(statusCode) else {
            throw SubscriptionFetchError.httpStatus(statusCode)
        }
    }

    private var requestPath: String {
        let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
        var path = components?.percentEncodedPath ?? ""
        if path.isEmpty {
            path = "/"
        }
        if let query = components?.percentEncodedQuery, !query.isEmpty {
            path += "?\(query)"
        }
        return path
    }

    private var hostHeader: String {
        if port == 443 {
            return host
        }
        return "\(host):\(port)"
    }
}

private final class RequestState {
    private let lock = NSLock()
    private var continuation: CheckedContinuation<Data, Error>?
    private let connection: NWConnection
    private let onFinish: () -> Void
    private(set) var data = Data()

    init(continuation: CheckedContinuation<Data, Error>, connection: NWConnection, onFinish: @escaping () -> Void) {
        self.continuation = continuation
        self.connection = connection
        self.onFinish = onFinish
    }

    func append(_ chunk: Data) {
        lock.lock()
        data.append(chunk)
        lock.unlock()
    }

    func finish(_ result: () throws -> Data) {
        do {
            finish(returning: try result())
        } catch {
            finish(throwing: error)
        }
    }

    func finish(returning data: Data) {
        lock.lock()
        guard let continuation else {
            lock.unlock()
            return
        }
        self.continuation = nil
        lock.unlock()

        connection.cancel()
        onFinish()
        continuation.resume(returning: data)
    }

    func finish(throwing error: Error) {
        lock.lock()
        guard let continuation else {
            lock.unlock()
            return
        }
        self.continuation = nil
        lock.unlock()

        connection.cancel()
        onFinish()
        continuation.resume(throwing: error)
    }
}

private extension Array where Element == String {
    var contentLength: Int? {
        for line in self {
            let parts = line.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false)
            guard parts.count == 2,
                  parts[0].trimmingCharacters(in: .whitespacesAndNewlines).lowercased() == "content-length" else {
                continue
            }
            return Int(parts[1].trimmingCharacters(in: .whitespacesAndNewlines))
        }
        return nil
    }
}
