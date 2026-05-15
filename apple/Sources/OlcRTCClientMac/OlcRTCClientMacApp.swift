import OlcRTCClientKit
import AppKit
import SwiftUI

@main
struct OlcRTCClientMacApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
                .frame(minWidth: 500, minHeight: 620)
                .background(WindowInitialSizeReader())
        }
        .windowStyle(.titleBar)
        .defaultSize(width: 540, height: 680)
    }
}

private struct WindowInitialSizeReader: NSViewRepresentable {
    func makeNSView(context: Context) -> NSView {
        let view = NSView()
        DispatchQueue.main.async {
            context.coordinator.applyInitialSize(to: view.window)
        }
        return view
    }

    func updateNSView(_ nsView: NSView, context: Context) {
        DispatchQueue.main.async {
            context.coordinator.applyInitialSize(to: nsView.window)
        }
    }

    func makeCoordinator() -> Coordinator {
        Coordinator()
    }

    final class Coordinator {
        private var didApplyInitialSize = false

        func applyInitialSize(to window: NSWindow?) {
            guard !didApplyInitialSize, let window else {
                return
            }

            didApplyInitialSize = true
            window.minSize = NSSize(width: 500, height: 620)
            window.setContentSize(NSSize(width: 540, height: 680))
            window.center()
        }
    }
}
