import XCTest
@testable import LoomLocalAppUI

final class LoomWorkspaceShellStateTests: XCTestCase {
    func testGovernancePanelCanOpenPinSwitchAndCloseIndependently() {
        var panel = LoomGovernancePanelState()
        XCTAssertEqual(panel.mode, .hidden)

        panel.open(.team)
        XCTAssertEqual(panel.mode, .visible)
        XCTAssertEqual(panel.destination, .team)

        panel.togglePin()
        panel.open(.evidence)
        XCTAssertEqual(panel.mode, .pinned)
        XCTAssertEqual(panel.destination, .evidence)

        panel.close()
        XCTAssertEqual(panel.mode, .hidden)
        XCTAssertEqual(panel.destination, .evidence)
    }

    func testEscapeDismissesVisibleAndPinnedGovernancePanel() {
        var panel = LoomGovernancePanelState()

        XCTAssertFalse(panel.dismissIfPresented())
        XCTAssertEqual(panel.mode, .hidden)

        panel.open(.overview)
        XCTAssertTrue(panel.dismissIfPresented())
        XCTAssertEqual(panel.mode, .hidden)

        panel.open(.attention)
        panel.togglePin()
        XCTAssertEqual(panel.mode, .pinned)
        XCTAssertTrue(panel.dismissIfPresented())
        XCTAssertEqual(panel.mode, .hidden)
        XCTAssertEqual(panel.destination, .attention)
    }

    func testGovernanceSwitcherIncludesTopologyAndTimelineAsRealViews() {
        let destinations = LoomGovernanceDestination.allCases

        XCTAssertTrue(destinations.contains(.topology))
        XCTAssertTrue(destinations.contains(.timeline))
        XCTAssertEqual(
            Set(destinations.map(\.rawValue)).count,
            destinations.count,
            "Every governance view needs a unique accessible label"
        )
    }

    func testLayoutProtectsConversationAndUsesOverlayAtCompactWidth() {
        let wide = LoomWorkspaceLayoutPolicy.metrics(
            for: 1_440,
            panelMode: .visible
        )
        XCTAssertEqual(wide.railWidth, 192)
        XCTAssertEqual(wide.panelPlacement, .split)
        XCTAssertEqual(wide.panelWidth, 360)
        XCTAssertGreaterThanOrEqual(wide.conversationMinimumWidth, 560)

        let medium = LoomWorkspaceLayoutPolicy.metrics(
            for: 1_080,
            panelMode: .visible
        )
        XCTAssertEqual(medium.railWidth, 64)
        XCTAssertEqual(medium.panelPlacement, .split)
        XCTAssertGreaterThanOrEqual(medium.conversationMinimumWidth, 500)

        let compact = LoomWorkspaceLayoutPolicy.metrics(
            for: 900,
            panelMode: .pinned
        )
        XCTAssertEqual(compact.railWidth, 64)
        XCTAssertEqual(compact.panelPlacement, .overlay)
        XCTAssertEqual(compact.panelWidth, 360)
        XCTAssertGreaterThanOrEqual(compact.conversationMinimumWidth, 500)

        let hidden = LoomWorkspaceLayoutPolicy.metrics(
            for: 900,
            panelMode: .hidden
        )
        XCTAssertEqual(hidden.panelPlacement, .hidden)
    }
}
