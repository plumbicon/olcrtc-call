import SwiftUI

public struct ContentView: View {
    @StateObject private var viewModel: ClientViewModel

    @MainActor
    public init() {
        _viewModel = StateObject(wrappedValue: ClientViewModel())
    }

    @MainActor
    public init(viewModel: ClientViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        NavigationSplitView {
            ProfileSidebar(viewModel: viewModel)
        } detail: {
            VStack(spacing: 0) {
                ClientHeaderView(viewModel: viewModel)
                Divider()
                ProfileEditorView(
                    profile: $viewModel.draft,
                    useSystemProxy: $viewModel.useSystemProxy,
                    selectedNetworkService: $viewModel.selectedNetworkService,
                    networkServices: viewModel.networkServices,
                    validationMessage: viewModel.validationMessage,
                    onCommit: viewModel.saveDraft
                )
                Divider()
                LogView(logs: viewModel.logs) {
                    viewModel.clearLogs()
                }
            }
            .navigationTitle(viewModel.selectedProfileName)
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
        }
        #if os(iOS)
        .dynamicTypeSize(.small ... .large)
        .controlSize(.small)
        #endif
    }
}

private struct ProfileSidebar: View {
    @ObservedObject var viewModel: ClientViewModel
    @State private var isShowingImporter = false

    var body: some View {
        List(selection: selection) {
            let groups = subscriptionGroups
            let ungrouped = ungroupedProfiles

            if !ungrouped.isEmpty {
                Section("Profiles") {
                    ForEach(ungrouped) { profile in
                        SelectableProfileRow(profile: profile)
                            .tag(Optional(profile.id))
                            .contextMenu {
                                Button("Delete", role: .destructive) {
                                    viewModel.deleteProfiles(ids: [profile.id])
                                }
                            }
                    }
                    .onDelete { offsets in
                        let ids = offsets.compactMap { ungrouped.indices.contains($0) ? ungrouped[$0].id : nil }
                        viewModel.deleteProfiles(ids: ids)
                    }
                }
            }

            ForEach(groups) { group in
                Section {
                    SubscriptionHeader(
                        group: group,
                        isRefreshing: viewModel.refreshingSubscriptionIDs.contains(group.id),
                        onRefresh: { viewModel.refreshSubscription(group.id) },
                        onDelete: { viewModel.deleteSubscription(group.id) }
                    )
                    #if os(iOS)
                    .listRowSeparator(.hidden, edges: .top)
                    #endif

                    ForEach(group.profiles) { profile in
                        SelectableProfileRow(profile: profile)
                            .tag(Optional(profile.id))
                            .contextMenu {
                                Button("Delete", role: .destructive) {
                                    viewModel.deleteProfiles(ids: [profile.id])
                                }
                            }
                    }
                }
            }
        }
        .navigationTitle("Profiles")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button(action: viewModel.addProfile) {
                    Label("Add Profile", systemImage: "plus")
                }
            }

            ToolbarItem(placement: .primaryAction) {
                Button {
                    isShowingImporter = true
                } label: {
                    Label("Import Profile", systemImage: "square.and.arrow.down")
                }
            }
        }
        .sheet(isPresented: $isShowingImporter) {
            AddProfileImportSheet(isImporting: viewModel.isImporting) { value in
                viewModel.importValue(value)
                isShowingImporter = false
            }
        }
        #if os(macOS)
        .navigationSplitViewColumnWidth(min: 220, ideal: 260, max: 320)
        #endif
    }

    private var selection: Binding<UUID?> {
        Binding(
            get: { viewModel.selectedProfileID },
            set: { viewModel.selectProfile($0) }
        )
    }

    private var ungroupedProfiles: [ConnectionProfile] {
        viewModel.profiles.filter { $0.subscription == nil }
    }

    private var subscriptionGroups: [SubscriptionGroup] {
        var groups: [SubscriptionGroup] = []
        for profile in viewModel.profiles {
            guard let subscription = profile.subscription else {
                continue
            }

            if let index = groups.firstIndex(where: { $0.id == subscription.id }) {
                groups[index].profiles.append(profile)
            } else {
                groups.append(SubscriptionGroup(metadata: subscription, profiles: [profile]))
            }
        }
        return groups
    }
}

private struct AddProfileImportSheet: View {
    @Environment(\.dismiss) private var dismiss
    @State private var importText = ""

    let isImporting: Bool
    let onImport: (String) -> Void

    private var canImport: Bool {
        !importText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && !isImporting
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Add profile")
                .font(.headline)

            ZStack(alignment: .topLeading) {
                TextField("", text: $importText, axis: .vertical)
                    .lineLimit(4...8)
                    .textFieldStyle(.plain)
                    #if os(iOS)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    #endif
                    .onSubmit(importValue)

                if importText.isEmpty {
                    Text("olcrtc://, subscription URL, or sub.md text")
                        .foregroundStyle(.tertiary)
                        .lineLimit(3)
                        .allowsHitTesting(false)
                }
            }
            .font(.body)
            .padding(10)
            .background(Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 8))

            HStack {
                Spacer()
                Button("Cancel", role: .cancel) {
                    dismiss()
                }

                Button(action: importValue) {
                    ImportLabel(isImporting: isImporting)
                }
                .disabled(!canImport)
                .buttonStyle(.borderedProminent)
            }
        }
        .padding()
        #if os(macOS)
        .frame(width: 460)
        #endif
    }

    private func importValue() {
        let value = importText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !value.isEmpty else {
            return
        }

        onImport(value)
        dismiss()
    }
}

private struct ImportLabel: View {
    let isImporting: Bool

    var body: some View {
        if isImporting {
            Label("Importing", systemImage: "arrow.triangle.2.circlepath")
        } else {
            Label("Import", systemImage: "square.and.arrow.down")
        }
    }
}

private struct SubscriptionGroup: Identifiable {
    var metadata: SubscriptionMetadata
    var profiles: [ConnectionProfile]

    var id: UUID { metadata.id }
}

private struct SubscriptionHeader: View {
    let group: SubscriptionGroup
    let isRefreshing: Bool
    let onRefresh: () -> Void
    let onDelete: () -> Void

    var body: some View {
        HStack(spacing: 8) {
            SubscriptionMarker(metadata: group.metadata)

            VStack(alignment: .leading, spacing: 2) {
                Text(group.metadata.name)
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.primary)
                    .lineLimit(1)

                HStack(spacing: 6) {
                    Text("\(group.profiles.count) servers")
                    if let available = group.metadata.available {
                        Text(available)
                    }
                }
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            }
            .layoutPriority(1)

            Spacer()

            SubscriptionActionButton(
                systemImage: isRefreshing ? "arrow.triangle.2.circlepath" : "arrow.clockwise",
                accessibilityLabel: "Refresh subscription",
                action: onRefresh
            )
            .disabled(isRefreshing || group.metadata.sourceURL == nil)
            .foregroundStyle(.secondary)
            .help(group.metadata.sourceURL == nil ? "Subscription has no source URL" : "Refresh subscription")

            SubscriptionActionButton(
                systemImage: "trash",
                accessibilityLabel: "Delete subscription",
                action: onDelete
            )
            .foregroundStyle(.red)
            .help("Delete subscription")
        }
        .padding(.vertical, 4)
    }
}

private struct SubscriptionActionButton: View {
    let systemImage: String
    let accessibilityLabel: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemImage)
                #if os(iOS)
                .font(.system(size: 18, weight: .semibold))
                .frame(width: 40, height: 40)
                #else
                .font(.system(size: 13, weight: .semibold))
                .frame(width: 26, height: 26)
                #endif
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(accessibilityLabel)
    }
}

private struct SelectableProfileRow: View {
    let profile: ConnectionProfile

    var body: some View {
        #if os(iOS)
        NavigationLink(value: profile.id) {
            ProfileRow(profile: profile)
        }
        #else
        ProfileRow(profile: profile)
        #endif
    }
}

private struct ProfileRow: View {
    let profile: ConnectionProfile

    var body: some View {
        HStack(spacing: 8) {
            SubscriptionMarker(metadata: profile.subscription, fallbackSystemImage: "network")

            VStack(alignment: .leading, spacing: 2) {
                Text(profile.name.isEmpty ? "Untitled profile" : profile.name)
                    .font(.body.weight(.medium))
                    .lineLimit(1)

                if let detail {
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
        }
    }

    private var detail: String? {
        guard let subscription = profile.subscription else {
            return nil
        }

        var values: [String] = []
        if let ip = subscription.nodeIP {
            values.append(ip)
        }
        values.append("\(profile.carrier.title) / \(profile.transport.title)")
        if let available = subscription.nodeAvailable {
            values.append(available)
        }
        return values.joined(separator: " · ")
    }
}

private struct SubscriptionMarker: View {
    let metadata: SubscriptionMetadata?
    var fallbackSystemImage = "folder"

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: 4)
                .fill(markerColor.opacity(0.18))
                .frame(width: 22, height: 22)

            if let icon = metadata?.nodeIcon ?? metadata?.icon, !icon.isEmpty {
                Text(icon)
                    .font(.caption)
                    .lineLimit(1)
            } else {
                Image(systemName: fallbackSystemImage)
                    .font(.caption)
                    .foregroundStyle(markerColor)
            }
        }
        .frame(width: 22, height: 22)
    }

    private var markerColor: Color {
        Color(hex: metadata?.nodeColor ?? metadata?.color) ?? .accentColor
    }
}

private struct ClientHeaderView: View {
    @ObservedObject var viewModel: ClientViewModel

    var body: some View {
        #if os(iOS)
        HStack(spacing: 10) {
            VStack(alignment: .leading, spacing: 4) {
                Text(viewModel.selectedProfileName)
                    .font(.headline.weight(.semibold))
                    .lineLimit(1)
                    .minimumScaleFactor(0.75)
                StatusBadge(status: viewModel.status)
            }

            Spacer(minLength: 8)

            Button(action: viewModel.stop) {
                Label("Stop", systemImage: "stop.fill")
            }
            .disabled(!viewModel.status.isRunning)

            Button(action: viewModel.start) {
                Label("Start", systemImage: "play.fill")
            }
            .buttonStyle(.borderedProminent)
            .disabled(!viewModel.canStart)
        }
        .font(.subheadline)
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        #else
        HStack(spacing: 16) {
            VStack(alignment: .leading, spacing: 6) {
                Text(viewModel.selectedProfileName)
                    .font(.title2.weight(.semibold))
                    .lineLimit(1)
                StatusBadge(status: viewModel.status)
            }

            Spacer()

            Button(action: viewModel.stop) {
                Label("Stop", systemImage: "stop.fill")
            }
            .disabled(!viewModel.status.isRunning)

            Button(action: viewModel.start) {
                Label("Start", systemImage: "play.fill")
            }
            .buttonStyle(.borderedProminent)
            .disabled(!viewModel.canStart)
        }
        .padding()
        #endif
    }
}

private struct StatusBadge: View {
    let status: ClientStatus

    var body: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(color)
                .frame(width: 8, height: 8)
            Text(title)
                .font(.caption.weight(.medium))
                .foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .combine)
    }

    private var title: String {
        switch status {
        case .failed(let message):
            message.isEmpty ? status.title : "\(status.title): \(message)"
        default:
            status.title
        }
    }

    private var color: Color {
        switch status {
        case .stopped: .secondary
        case .starting, .stopping: .orange
        case .ready: .green
        case .failed: .red
        }
    }
}

private extension Color {
    init?(hex: String?) {
        guard var value = hex?.trimmingCharacters(in: .whitespacesAndNewlines), !value.isEmpty else {
            return nil
        }

        if value.hasPrefix("#") {
            value.removeFirst()
        }

        guard value.count == 6, let raw = Int(value, radix: 16) else {
            return nil
        }

        self.init(
            red: Double((raw >> 16) & 0xFF) / 255.0,
            green: Double((raw >> 8) & 0xFF) / 255.0,
            blue: Double(raw & 0xFF) / 255.0
        )
    }
}
