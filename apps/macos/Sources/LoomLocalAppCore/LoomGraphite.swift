import SwiftUI

// LoomGraphite lives in LoomLocalAppCore so every client surface (native app,
// probe, permission explorer) shares one token source of truth.

public enum LoomGraphite {
    public static let minimumActionTarget: CGFloat = 44
    public static let cardRadius: CGFloat = 12
    public static let railWidth: CGFloat = 192
    public static let inspectorWidth: CGFloat = 284
    public static let motionDuration = 0.2

    // Loom accent: indigo-violet (#5E6AD2). Distinct from stock system blue
    // and generic AI purple, but legible in both light and dark macOS modes.
    public static let accent = Color(hex: "#5E6AD2")
    public static let onAccent = Color.white
    public static let accentMuted = accent.opacity(0.10)
    public static let accentSecondary = Color(hex: "#818CF8")

    public static let statusSuccess = Color(nsColor: .systemGreen)
    public static let statusWarning = Color(nsColor: .systemOrange)
    public static let statusDanger = Color(nsColor: .systemRed)

    // Semantic text aliases for cross-component consistency.
    public static let textPrimary = Color.primary
    public static let textSecondary = Color.secondary
    public static let textMuted = Color(nsColor: .secondaryLabelColor)

    public static let canvas = Color(
        nsColor: .windowBackgroundColor
    )
    public static let rail = Color(
        nsColor: .underPageBackgroundColor
    )
    public static let surface = Color(
        nsColor: .controlBackgroundColor
    )
    public static let raised = Color(
        nsColor: .textBackgroundColor
    )
    public static let separator = Color(
        nsColor: .separatorColor
    )
}

extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 3: // RGB (12-bit)
            (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6: // RGB (24-bit)
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8: // ARGB (32-bit)
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (255, 0, 0, 0)
        }
        self.init(
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}

public struct LoomActionTarget: ViewModifier {
    public func body(content: Content) -> some View {
        content.frame(
            minWidth: LoomGraphite.minimumActionTarget,
            minHeight: LoomGraphite.minimumActionTarget
        )
    }
}

public extension View {
    func loomActionTarget() -> some View {
        modifier(LoomActionTarget())
    }
}
