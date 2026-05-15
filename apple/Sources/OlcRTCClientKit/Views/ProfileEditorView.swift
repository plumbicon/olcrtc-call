import SwiftUI

private let profileEditorValueColumnWidth: CGFloat = 200
private let profileEditorNumberFieldWidth: CGFloat = 64
private let profileEditorConnectionRowHeight: CGFloat = 46
private let videoCodecOptions = ["qrcode", "tile"]
private let videoHardwareOptions = ["none", "nvenc"]
private let videoQRRecoveryOptions = ["low", "medium", "high", "highest"]

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
        startsAdvancedExpanded: Bool = false,
        onCommit: @escaping () -> Void
    ) {
        _profile = profile
        _useSystemProxy = useSystemProxy
        _selectedNetworkService = selectedNetworkService
        _isAdvancedExpanded = State(initialValue: startsAdvancedExpanded)
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

            Section {
                ConnectionSettingsCard(
                    profile: $profile,
                    isExpanded: $isAdvancedExpanded,
                    onCommit: onCommit
                )
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

private struct ConnectionSettingsCard: View {
    @Binding var profile: ConnectionProfile
    @Binding var isExpanded: Bool
    let onCommit: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Button {
                withAnimation(.easeInOut(duration: 0.16)) {
                    isExpanded.toggle()
                }
            } label: {
                HStack(spacing: 8) {
                    Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
                        .font(.system(size: 12, weight: .semibold))
                        .frame(width: 14)

                    Text("Подключение")
                        .font(.headline)

                    Spacer()
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .padding(.vertical, 10)

            if isExpanded {
                VStack(spacing: 0) {
                    ConnectionTextRow(title: "Room ID", text: $profile.roomID, onCommit: onCommit)
                    Divider()

                    ConnectionTextRow(title: "Client ID", text: $profile.clientID, onCommit: onCommit)
                    Divider()

                    ConnectionSecureRow(title: "64-символьный ключ", text: $profile.keyHex, onCommit: onCommit)

                    transportRows
                    Divider()
                }
            }
        }
    }

    @ViewBuilder
    private var transportRows: some View {
        switch profile.transport {
        case .vp8channel:
            Divider()
            ConnectionNumberRow(title: "FPS", value: $profile.vp8FPS, range: 1...120)
            Divider()
            ConnectionNumberRow(title: "Размер пакета", value: $profile.vp8BatchSize, range: 1...128)

        case .seichannel:
            Divider()
            ConnectionNumberRow(title: "FPS", value: $profile.seiFPS, range: 1...120)
            Divider()
            ConnectionNumberRow(title: "Размер пакета", value: $profile.seiBatchSize, range: 1...128)
            Divider()
            ConnectionNumberRow(
                title: "Размер фрагмента",
                value: $profile.seiFragmentSize,
                range: 64...4_096,
                step: 64
            )
            Divider()
            ConnectionAckTimeoutRow(
                title: "ACK таймаут",
                value: $profile.seiAckTimeoutMillis,
                range: 100...10_000,
                step: 100
            )

        case .videochannel:
            Divider()
            ConnectionPickerRow(title: "Кодек", selection: $profile.videoCodec, options: videoCodecOptions)
            Divider()
            ConnectionNumberRow(title: "Ширина", value: $profile.videoWidth, range: 1...7_680)
            Divider()
            ConnectionNumberRow(title: "Высота", value: $profile.videoHeight, range: 1...4_320)
            Divider()
            ConnectionNumberRow(title: "FPS", value: $profile.videoFPS, range: 1...120)
            Divider()
            ConnectionTextRow(title: "Битрейт", text: $profile.videoBitrate, onCommit: onCommit)
            Divider()
            ConnectionPickerRow(
                title: "Аппаратное ускорение",
                selection: $profile.videoHardwareAcceleration,
                options: videoHardwareOptions
            )
            Divider()
            ConnectionPickerRow(
                title: "QR коррекция",
                selection: $profile.videoQRRecovery,
                options: videoQRRecoveryOptions
            )
            Divider()
            ConnectionNumberRow(title: "QR фрагмент", value: $profile.videoQRSize, range: 0...65_535)

            if profile.videoCodec == "tile" {
                Divider()
                ConnectionNumberRow(title: "Размер тайла", value: $profile.videoTileModule, range: 1...270)
                Divider()
                ConnectionNumberRow(title: "RS паритет, %", value: $profile.videoTileRS, range: 0...200)
            }

        case .datachannel:
            EmptyView()
        }
    }
}

private struct ConnectionAckTimeoutRow: View {
    let title: String
    @Binding var value: Int
    let range: ClosedRange<Int>
    var step = 1

    private var clampedValue: Binding<Int> {
        Binding(
            get: { value },
            set: { newValue in
                value = min(max(newValue, range.lowerBound), range.upperBound)
            }
        )
    }

    private var textValue: Binding<String> {
        Binding(
            get: { "\(value)" },
            set: { newValue in
                let digits = newValue.filter(\.isNumber)
                guard let parsedValue = Int(digits) else {
                    return
                }
                value = min(max(parsedValue, range.lowerBound), range.upperBound)
            }
        )
    }

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            Text(title)
                .lineLimit(1)
                .frame(minWidth: 170, alignment: .leading)

            Spacer(minLength: 16)

            HStack(spacing: 8) {
                TextField("", text: textValue)
                    .textFieldStyle(.roundedBorder)
                    .multilineTextAlignment(.trailing)
                    .font(.body.monospacedDigit())
                    .frame(width: 88)
                    .accessibilityLabel(title)

                Stepper("", value: clampedValue, in: range, step: step)
                    .labelsHidden()
            }
            .frame(width: profileEditorValueColumnWidth, height: profileEditorConnectionRowHeight, alignment: .trailing)
        }
        .frame(height: profileEditorConnectionRowHeight, alignment: .center)
    }
}

private struct ConnectionPickerRow: View {
    let title: String
    @Binding var selection: String
    let options: [String]

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            Text(title)
                .lineLimit(1)
                .frame(minWidth: 170, alignment: .leading)

            Spacer(minLength: 16)

            Picker("", selection: $selection) {
                ForEach(options, id: \.self) { option in
                    Text(option).tag(option)
                }
            }
            .labelsHidden()
            .frame(width: profileEditorValueColumnWidth, alignment: .trailing)
        }
        .frame(height: profileEditorConnectionRowHeight)
    }
}

private struct ConnectionTextRow: View {
    let title: String
    @Binding var text: String
    let onCommit: () -> Void

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            Text(title)
                .lineLimit(1)
                .frame(minWidth: 170, alignment: .leading)

            Spacer(minLength: 16)

            TextField("", text: $text)
                .olcPlainInput()
                .textFieldStyle(.roundedBorder)
                .multilineTextAlignment(.leading)
                .frame(width: profileEditorValueColumnWidth)
                .accessibilityLabel(title)
                .onSubmit(onCommit)
        }
        .frame(height: profileEditorConnectionRowHeight)
    }
}

private struct ConnectionSecureRow: View {
    let title: String
    @Binding var text: String
    let onCommit: () -> Void

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            Text(title)
                .lineLimit(1)
                .frame(minWidth: 170, alignment: .leading)

            Spacer(minLength: 16)

            SecureField("", text: $text)
                .olcPlainInput()
                .textFieldStyle(.roundedBorder)
                .multilineTextAlignment(.leading)
                .frame(width: profileEditorValueColumnWidth)
                .accessibilityLabel(title)
                .onSubmit(onCommit)
        }
        .frame(height: profileEditorConnectionRowHeight)
    }
}

private struct ConnectionNumberRow: View {
    let title: String
    @Binding var value: Int
    let range: ClosedRange<Int>
    var step = 1
    var suffix = ""

    private var clampedValue: Binding<Int> {
        Binding(
            get: { value },
            set: { newValue in
                value = min(max(newValue, range.lowerBound), range.upperBound)
            }
        )
    }

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            Text(title)
                .lineLimit(1)
                .frame(minWidth: 170, alignment: .leading)

            Spacer(minLength: 16)

            HStack(spacing: 8) {
                TextField("", value: clampedValue, format: .number)
                    .textFieldStyle(.roundedBorder)
                    .multilineTextAlignment(.trailing)
                    .frame(width: profileEditorNumberFieldWidth)
                    .accessibilityLabel(title)

                if !suffix.isEmpty {
                    Text(suffix)
                        .foregroundStyle(.secondary)
                        .frame(width: 28, alignment: .leading)
                }

                Stepper("", value: clampedValue, in: range, step: step)
                    .labelsHidden()
            }
            .frame(width: profileEditorValueColumnWidth, alignment: .trailing)
        }
        .frame(height: profileEditorConnectionRowHeight)
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
