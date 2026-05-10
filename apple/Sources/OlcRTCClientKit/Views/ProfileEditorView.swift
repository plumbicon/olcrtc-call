import SwiftUI

public struct ProfileEditorView: View {
    @Binding var profile: ConnectionProfile
    @Binding var useSystemProxy: Bool
    @Binding var selectedNetworkService: String
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
                Section("Subscription") {
                    LabeledContent("Name", value: subscription.name)

                    if let sourceURL = subscription.sourceURL {
                        LabeledContent("Source") {
                            Text(sourceURL)
                                .lineLimit(1)
                                .truncationMode(.middle)
                        }
                    }

                    if let available = subscription.available {
                        LabeledContent("Available", value: available)
                    }

                    if let used = subscription.used {
                        LabeledContent("Used", value: used)
                    }

                    if let nodeIP = subscription.nodeIP {
                        LabeledContent("Node IP", value: nodeIP)
                    }

                    if let nodeComment = subscription.nodeComment {
                        LabeledContent("Comment") {
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

            Section("Connection") {
                TextField("Profile name", text: $profile.name)
                    .onSubmit(onCommit)

                Picker("Carrier", selection: $profile.carrier) {
                    ForEach(Carrier.allCases) { carrier in
                        Text(carrier.title).tag(carrier)
                    }
                }

                Picker("Transport", selection: $profile.transport) {
                    ForEach(Transport.allCases) { transport in
                        Text(transport.title).tag(transport)
                    }
                }

                TextField("Room ID", text: $profile.roomID)
                    .olcPlainInput()
                    .onSubmit(onCommit)

                TextField("Client ID", text: $profile.clientID)
                    .olcPlainInput()
                    .onSubmit(onCommit)

                SecureField("64-char key", text: $profile.keyHex)
                    .olcPlainInput()
                    .onSubmit(onCommit)
            }

            Section("SOCKS") {
                HStack {
                    TextField("Port", value: $profile.socksPort, format: .number)
                        .frame(maxWidth: 110)
                        #if os(iOS)
                        .keyboardType(.numberPad)
                        #endif
                    Stepper("Port", value: $profile.socksPort, in: 1...65_535)
                        .labelsHidden()

                    if !PortAvailability.isLocalTCPPortAvailable(profile.socksPort) {
                        Button {
                            profile.socksPort = PortAvailability.nextAvailableTCPPort(startingAt: profile.socksPort)
                            onCommit()
                        } label: {
                            Label("Use Free Port", systemImage: "wand.and.stars")
                        }
                    }
                }

                TextField("Username", text: $profile.socksUser)
                    .olcPlainInput()
                    .onSubmit(onCommit)

                SecureField("Password", text: $profile.socksPass)
                    .olcPlainInput()
                    .onSubmit(onCommit)
            }

            #if os(macOS)
            Section("System Proxy") {
                Toggle("Route system traffic through SOCKS", isOn: $useSystemProxy)

                Picker("Network service", selection: $selectedNetworkService) {
                    ForEach(networkServices, id: \.self) { service in
                        Text(service).tag(service)
                    }
                }
                .disabled(!useSystemProxy)
            }
            #elseif os(iOS)
            Section("VPN") {
                Toggle("Route system traffic through VPN", isOn: $useSystemProxy)
            }
            #endif

            Section("Runtime") {
                TextField("DNS server", text: $profile.dnsServer)
                    .olcPlainInput()
                    .onSubmit(onCommit)

                Toggle("Debug logging", isOn: $profile.debugLogging)

                Stepper(value: $profile.startTimeoutMillis, in: 10_000...300_000, step: 5_000) {
                    LabeledContent("Start timeout", value: "\(profile.startTimeoutMillis / 1_000)s")
                }
            }

            if profile.transport == .vp8channel {
                Section("VP8") {
                    Stepper(value: $profile.vp8FPS, in: 1...120) {
                        LabeledContent("FPS", value: "\(profile.vp8FPS)")
                    }

                    Stepper(value: $profile.vp8BatchSize, in: 1...128) {
                        LabeledContent("Batch size", value: "\(profile.vp8BatchSize)")
                    }
                }
            }

            if profile.transport == .seichannel {
                Section("SEI") {
                    Stepper(value: $profile.seiFPS, in: 1...120) {
                        LabeledContent("FPS", value: "\(profile.seiFPS)")
                    }

                    Stepper(value: $profile.seiBatchSize, in: 1...128) {
                        LabeledContent("Batch size", value: "\(profile.seiBatchSize)")
                    }

                    Stepper(value: $profile.seiFragmentSize, in: 64...4_096, step: 64) {
                        LabeledContent("Fragment size", value: "\(profile.seiFragmentSize)")
                    }

                    Stepper(value: $profile.seiAckTimeoutMillis, in: 100...10_000, step: 100) {
                        LabeledContent("ACK timeout", value: "\(profile.seiAckTimeoutMillis)ms")
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
