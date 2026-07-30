import SwiftUI

public enum LoomGraphite {
    public static let minimumActionTarget: CGFloat = 44
    public static let cardRadius: CGFloat = 10
    public static let railWidth: CGFloat = 176
    public static let inspectorWidth: CGFloat = 284
    public static let motionDuration = 0.2

    public static let accent = Color(
        red: 0.16,
        green: 0.39,
        blue: 0.92
    )
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

struct LoomActionTarget: ViewModifier {
    func body(content: Content) -> some View {
        content.frame(
            minWidth: LoomGraphite.minimumActionTarget,
            minHeight: LoomGraphite.minimumActionTarget
        )
    }
}

extension View {
    func loomActionTarget() -> some View {
        modifier(LoomActionTarget())
    }
}
