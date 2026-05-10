import Foundation

public final class ProfileStore {
    private let defaults: UserDefaults
    private let secretStore: ProfileSecretStore
    private let profilesKey = "olcrtc.apple.profiles.v1"
    private let selectedKey = "olcrtc.apple.selectedProfile.v1"

    public init(
        defaults: UserDefaults = .standard,
        secretStore: ProfileSecretStore = ProfileSecretStore()
    ) {
        self.defaults = defaults
        self.secretStore = secretStore
    }

    public func loadProfiles() -> [ConnectionProfile] {
        guard let data = defaults.data(forKey: profilesKey) else {
            return [.empty]
        }

        do {
            let profiles = try JSONDecoder().decode([ConnectionProfile].self, from: data)
            return profiles.isEmpty ? [.empty] : profiles.map { profile in
                var profile = profile
                secretStore.loadSecrets(into: &profile)
                return profile
            }
        } catch {
            return [.empty]
        }
    }

    public func saveProfiles(_ profiles: [ConnectionProfile]) {
        profiles.forEach(secretStore.saveSecrets)
        let publicProfiles = profiles.map { profile in
            var profile = profile
            profile.keyHex = ""
            profile.socksPass = ""
            return profile
        }

        guard let data = try? JSONEncoder().encode(publicProfiles) else {
            return
        }

        defaults.set(data, forKey: profilesKey)
    }

    public func deleteSecrets(profileIDs: [UUID]) {
        profileIDs.forEach(secretStore.deleteSecrets)
    }

    public func loadSelectedProfileID() -> UUID? {
        guard let value = defaults.string(forKey: selectedKey) else {
            return nil
        }

        return UUID(uuidString: value)
    }

    public func saveSelectedProfileID(_ id: UUID?) {
        defaults.set(id?.uuidString, forKey: selectedKey)
    }
}
