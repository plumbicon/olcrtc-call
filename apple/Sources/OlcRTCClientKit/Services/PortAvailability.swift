import Foundation

#if canImport(Darwin)
import Darwin
#endif

public enum PortAvailability {
    public static func nextAvailableTCPPort(startingAt preferredPort: Int = 10808, host: String = "127.0.0.1") -> Int {
        let start = min(max(preferredPort, 1), 65_535)

        for port in start...65_535 where isLocalTCPPortAvailable(port, host: host) {
            return port
        }

        if start > 1 {
            for port in 1..<start where isLocalTCPPortAvailable(port, host: host) {
                return port
            }
        }

        return preferredPort
    }

    public static func isLocalTCPPortAvailable(_ port: Int, host: String = "127.0.0.1") -> Bool {
        guard (1...65_535).contains(port) else {
            return false
        }

        #if canImport(Darwin)
        let fileDescriptor = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP)
        guard fileDescriptor >= 0 else {
            return false
        }
        defer { close(fileDescriptor) }

        var address = sockaddr_in()
        address.sin_len = UInt8(MemoryLayout<sockaddr_in>.stride)
        address.sin_family = sa_family_t(AF_INET)
        address.sin_port = in_port_t(port).bigEndian

        guard inet_pton(AF_INET, host, &address.sin_addr) == 1 else {
            return false
        }

        let result = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { socketAddress in
                bind(fileDescriptor, socketAddress, socklen_t(MemoryLayout<sockaddr_in>.stride))
            }
        }

        return result == 0
        #else
        return true
        #endif
    }
}
