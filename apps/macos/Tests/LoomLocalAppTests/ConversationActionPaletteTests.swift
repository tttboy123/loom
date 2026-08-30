import AppKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
final class ConversationActionPaletteTests: XCTestCase {
    func testChoiceMenuSelectionSkipsUnavailableRowsAndWraps() {
        let choices = [
            ConversationActionChoice(
                id: "unavailable",
                title: "Unavailable",
                detail: "Not ready",
                systemImage: "xmark",
                unavailableReason: "Provider is unavailable"
            ),
            ConversationActionChoice(
                id: "current",
                title: "Current",
                detail: "Selected route",
                systemImage: "checkmark",
                isSelected: true
            ),
            ConversationActionChoice(
                id: "next",
                title: "Next",
                detail: "Another route",
                systemImage: "arrow.right"
            ),
        ]
        var state = ConversationActionChoiceMenuState()
        state.synchronize(with: choices)
        XCTAssertEqual(state.selectedID, "current")
        state.move(.next, within: choices)
        XCTAssertEqual(state.selectedID, "next")
        state.move(.next, within: choices)
        XCTAssertEqual(state.selectedID, "current")
        state.move(.previous, within: choices)
        XCTAssertEqual(state.selectedChoice(in: choices)?.id, "next")
    }

    func testChoicePaletteRendersKeyboardSelectionAtCompactAccessibilitySize()
        throws
    {
        let choices = [
            ConversationActionChoice(
                id: "current",
                title: "Loom Native · DeepSeek · Primary",
                detail: "deepseek-chat",
                systemImage: "point.3.connected.trianglepath.dotted",
                isSelected: true
            ),
            ConversationActionChoice(
                id: "next",
                title: "OpenCode · MiniMax · Primary",
                detail: "MiniMax-M3",
                systemImage: "point.3.connected.trianglepath.dotted"
            ),
        ]
        let width = 360.0
        let height = 300.0
        let size = NSSize(width: width, height: height)
        let hosting = NSHostingView(
            rootView: ConversationActionChoicePalette(
                title: "Choose Route",
                choices: choices,
                selectedID: "next",
                onSelect: { _ in },
                onDismiss: {}
            )
            .dynamicTypeSize(.accessibility3)
            .frame(width: width, height: height)
        )
        hosting.frame = NSRect(origin: .zero, size: size)
        hosting.layoutSubtreeIfNeeded()
        let bitmap = try XCTUnwrap(
            hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
        )
        hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
        XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
        XCTAssertGreaterThan(
            try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).count,
            2_000
        )
    }

    func testPaletteRendersAtDesktopCompactAndAccessibilitySizes() throws {
        for (width, typeSize) in [
            (680.0, DynamicTypeSize.large),
            (360.0, DynamicTypeSize.large),
            (360.0, DynamicTypeSize.accessibility3),
        ] {
            let size = NSSize(width: width, height: 300)
            let hosting = NSHostingView(
                rootView: ConversationActionPalette(
                    suggestions: ConversationActionCatalog.commands,
                    selectedID: .mission,
                    context: ConversationActionContext(),
                    onSelect: { _ in },
                    onDismiss: {}
                )
                .dynamicTypeSize(typeSize)
                .frame(width: width, height: size.height)
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()

            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            XCTAssertEqual(bitmap.size.width, width)
            XCTAssertEqual(bitmap.size.height, size.height)
            XCTAssertGreaterThanOrEqual(bitmap.pixelsWide, Int(width))
            XCTAssertGreaterThanOrEqual(bitmap.pixelsHigh, Int(size.height))
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE6_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                let imageData = try XCTUnwrap(
                    bitmap.representation(using: .png, properties: [:])
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "commands-\(Int(width))-\(typeSize).png"
                    )
                )
            }
        }
    }

    func testComposerSourceContainsKeyboardAndProtectedDispatchPaths() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(".onKeyPress(.downArrow)"))
        XCTAssertTrue(source.contains(".onKeyPress(.upArrow)"))
        XCTAssertTrue(source.contains(".onKeyPress(.escape)"))
        XCTAssertTrue(source.contains(".onKeyPress(.return)"))
        XCTAssertTrue(source.contains("ConversationActionRouter.resolve(draft)"))
        XCTAssertTrue(source.contains("fullGovernancePresentation = .newMission"))
        XCTAssertTrue(source.contains("fullGovernancePresentation = .runtimeProviders"))
        XCTAssertTrue(source.contains("pendingStopAction = request"))
        XCTAssertTrue(source.contains("Credential text rejected"))
        XCTAssertTrue(source.contains("@State private var diagnosticPresentation"))
        XCTAssertTrue(source.contains(".sheet(item: $diagnosticPresentation)"))
        XCTAssertFalse(source.contains("@State private var diagnosticExporter"))
    }
}
