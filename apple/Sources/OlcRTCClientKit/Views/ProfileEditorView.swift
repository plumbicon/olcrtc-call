import SwiftUI

public struct ProfileEditorView: View {
    @Binding var profile: ConnectionProfile
    @Binding var useSystemProxy: Bool
    @Binding var selectedNetworkService: String
    @State private var isAdvancedExpanded = false

    let networkServices: [String]
    let validationMessage: String?
    let onCommit: () -> Void

    public init(
        profile: Binding<ConnectionProfile>,
        useSystemProxy: Binding<Bool>,
        selectedNetworkService: Binding<String>,
        networkServices: [String],
        validationMessage: String?,
        onCommit: @escaping () -> Void
    ) {
        _profile = profile
        _useSystemProxy = useSystemProxy
        _selectedNetworkService = selectedNetworkService
        self.networkServices = networkServices
        self.validationMessage = validationMessage
        self.onCommit = onCommit
    }

    public var body: some View {
        Form {
            if let subscription = profile.subscription {
                Section("Подписка") {
                    LabeledContent("Название", value: subscription.name)

                    if let sourceURL = subscription.sourceURL {
                        LabeledContent("Источник") {
                            Text(sourceURL)
                                .lineLimit(1)
                                .truncationMode(.middle)
                                .textSelection(.enabled)
                        }
                    }

                    if let available = subscription.available {
                        LabeledContent("Доступно", value: available)
                    }

                    if let used = subscription.used {
                        LabeledContent("Использовано", value: used)
                    }

                    if let nodeIP = subscription.nodeIP {
                        LabeledContent("IP узла", value: nodeIP)
                    }

                    if let nodeComment = subscription.nodeComment {
                        LabeledContent("Комментарий") {
                            Text(nodeComment)
                                .lineLimit(2)
                                .multilineTextAlignment(.trailing)
                        }
                    }
                }
            }

            if let validationMessage {
                Section {
                    Label(validationMessage, systemImage: "exclamationmark.triangle.fill")
                        .foregroundStyle(.orange)
                }
            }

            Section("Профиль") {
                TextField("Название профиля", text: $profile.name)
                    .onSubmit(onCommit)

                Picker("Провайдер", selection: $profile.carrier) {
                    ForEach(Carrier.allCases) { carrier in
                        Text(carrier.title).tag(carrier)
                    }
                }

                Picker("Транспорт", selection: $profile.transport) {
                    ForEach(Transport.allCases) { transport in
                        Text(transport.title).tag(transport)
                    }
                }
            }

            DisclosureGroup("Подключение", isExpanded: $isAdvancedExpanded) {
                Section("Подключение") {
                    TextField("Room ID", text: $profile.roomID)
                        .olcPlainInput()
                        .onSubmit(onCommit)

                    TextField("Client ID", text: $profile.clientID)
                        .olcPlainInput()
                        .onSubmit(onCommit)

                    SecureField("64-символьный ключ", text: $profile.keyHex)
                        .olcPlainInput()
                        .onSubmit(onCommit)
                }

                if profile.transport == .vp8channel {
                    Section("VP8") {
                        Stepper(value: $profile.vp8FPS, in: 1...120) {
                            LabeledContent("FPS", value: "\(profile.vp8FPS)")
                        }

                        Stepper(value: $profile.vp8BatchSize, in: 1...128) {
                            LabeledContent("Размер пакета", value: "\(profile.vp8BatchSize)")
                        }
                    }
                }

                if profile.transport == .seichannel {
                    Section("SEI") {
                        Stepper(value: $profile.seiFPS, in: 1...120) {
                            LabeledContent("FPS", value: "\(profile.seiFPS)")
                        }

                        Stepper(value: $profile.seiBatchSize, in: 1...128) {
                            LabeledContent("Размер пакета", value: "\(profile.seiBatchSize)")
                        }

                        Stepper(value: $profile.seiFragmentSize, in: 64...4_096, step: 64) {
                            LabeledContent("Размер фрагмента", value: "\(profile.seiFragmentSize)")
                        }

                        Stepper(value: $profile.seiAckTimeoutMillis, in: 100...10_000, step: 100) {
                            LabeledContent("ACK таймаут", value: "\(profile.seiAckTimeoutMillis)ms")
                        }
                    }
                }
            }
        }
        .formStyle(.grouped)
        #if os(iOS)
        .font(.subheadline)
        .environment(\.defaultMinListRowHeight, 38)
        .environment(\.defaultMinListHeaderHeight, 22)
        #endif
        .onDisappear(perform: onCommit)
    }
}

private extension View {
    @ViewBuilder
    func olcPlainInput() -> some View {
        #if os(iOS)
        self
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
        #else
        self
        #endif
    }
}
