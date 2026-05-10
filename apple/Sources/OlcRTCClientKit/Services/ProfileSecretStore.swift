import Foundation
import Security

public final class ProfileSecretStore {
    private let service = "community.openlibre.olcrtc.profile"

    public init() {}

    public func loadSecrets(into profile: inout ConnectionProfile) {
        profile.keyHex = read(profileID: profile.id, field: "keyHex") ?? profile.keyHex
        profile.socksPass = read(profileID: profile.id, field: "socksPass") ?? profile.socksPass
    }

    public func saveSecrets(from profile: ConnectionProfile) {
        save(profile.keyHex, profileID: profile.id, field: "keyHex")
        save(profile.socksPass, profileID: profile.id, field: "socksPass")
    }

    public func deleteSecrets(profileID: UUID) {
        delete(profileID: profileID, field: "keyHex")
        delete(profileID: profileID, field: "socksPass")
    }

    private func read(profileID: UUID, field: String) -> String? {
        var query = baseQuery(profileID: profileID, field: field)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne

        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess, let data = result as? Data else {
            return nil
        }

        return String(data: data, encoding: .utf8)
    }

    private func save(_ value: String, profileID: UUID, field: String) {
        if value.isEmpty {
            delete(profileID: profileID, field: field)
            return
        }

        let data = Data(value.utf8)
        let query = baseQuery(profileID: profileID, field: field)
        let update = [kSecValueData as String: data]

        let status = SecItemUpdate(query as CFDictionary, update as CFDictionary)
        if status == errSecSuccess {
            return
        }

        var addQuery = query
        addQuery[kSecValueData as String] = data
        addQuery[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        SecItemAdd(addQuery as CFDictionary, nil)
    }

    private func delete(profileID: UUID, field: String) {
        SecItemDelete(baseQuery(profileID: profileID, field: field) as CFDictionary)
    }

    private func baseQuery(profileID: UUID, field: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "\(profileID.uuidString).\(field)",
        ]
    }
}
