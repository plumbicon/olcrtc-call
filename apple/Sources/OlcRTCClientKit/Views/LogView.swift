import SwiftUI

public struct LogView: View {
    let logs: [String]
    let onClear: () -> Void

    public init(logs: [String], onClear: @escaping () -> Void) {
        self.logs = logs
        self.onClear = onClear
    }

    public var body: some View {
        VStack(spacing: 0) {
            HStack {
                Label("Events", systemImage: "list.bullet.rectangle")
                    #if os(iOS)
                    .font(.subheadline.weight(.semibold))
                    #else
                    .font(.headline)
                    #endif
                Spacer()
                Button(action: onClear) {
                    Label("Clear", systemImage: "trash")
                }
                .disabled(logs.isEmpty)
            }
            .padding([.horizontal, .top])

            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 6) {
                        if logs.isEmpty {
                            Text("No events yet.")
                                .foregroundStyle(.secondary)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .padding(.vertical, 8)
                        } else {
                            ForEach(Array(logs.enumerated()), id: \.offset) { index, line in
                                Text(line)
                                    .font(.system(.caption, design: .monospaced))
                                    .foregroundStyle(.secondary)
                                    .textSelection(.enabled)
                                    .id(index)
                            }
                        }
                    }
                    .padding()
                }
                .frame(minHeight: 120, idealHeight: 180, maxHeight: 240)
                .onChange(of: logs.count) { count in
                    guard count > 0 else { return }
                    proxy.scrollTo(count - 1, anchor: .bottom)
                }
            }
        }
        #if os(iOS)
        .font(.subheadline)
        #endif
    }
}
