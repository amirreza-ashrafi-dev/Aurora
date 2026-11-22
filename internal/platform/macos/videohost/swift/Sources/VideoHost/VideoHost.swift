import AppKit

@MainActor func makeHost() throws -> any Host {
    AuroraVideoHost()
}

// MyGo owns this NSView. MPV's patched macvk output adds its existing input
// view, backed by MPV's existing CAMetalLayer, as a child of this container.
@MainActor final class AuroraVideoHost: NSView, Host {
    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        wantsLayer = true
        autoresizingMask = [.width, .height]
        postsBoundsChangedNotifications = true
        postsFrameChangedNotifications = true
    }

    required init?(coder: NSCoder) {
        fatalError("AuroraVideoHost is created programmatically")
    }
}
