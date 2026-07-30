import AppKit
import CryptoKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
final class LocalProductExperienceViewTests: XCTestCase {
    func testProviderDirectoryUsesConnectThenCapabilitySpecificManagement() {
        XCTAssertEqual(
            providerConnectionPrimaryAction(
                provider: .codex,
                connected: false
            ),
            .connect
        )
        XCTAssertEqual(
            providerConnectionPrimaryAction(
                provider: .codex,
                connected: true
            ),
            .manage
        )
        XCTAssertEqual(
            providerConnectionPrimaryAction(
                provider: .miniMax,
                connected: false
            ),
            .connect
        )
        XCTAssertEqual(
            providerConnectionPrimaryAction(
                provider: .miniMax,
                connected: true
            ),
            .manage
        )
    }

    func testProviderConnectionDirectoryRendersNativeConnectionCards()
        async throws
    {
        let store = LocalProductStore(client: try ProviderSetupStubClient())
        await store.refreshSetup()
        let rendered = try XCTUnwrap(
            render(
                ProviderConnectionDirectory(store: store)
                    .padding(28)
                    .frame(maxWidth: .infinity, maxHeight: .infinity),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 820,
                height: 320
            )
        )
        XCTAssertEqual(rendered.pixelWidth, 820)
        XCTAssertEqual(rendered.pixelHeight, 320)
        XCTAssertGreaterThan(rendered.png.count, 12_000)
        XCTAssertGreaterThan(rendered.colorBucketCount, 10)

        let dark = try XCTUnwrap(
            render(
                ProviderConnectionDirectory(store: store)
                    .padding(28)
                    .frame(maxWidth: .infinity, maxHeight: .infinity),
                colorScheme: .dark,
                dynamicTypeSize: .large,
                width: 820,
                height: 320
            )
        )
        XCTAssertNotEqual(rendered.digest, dark.digest)

        let accessible = try XCTUnwrap(
            render(
                ProviderConnectionDirectory(store: store)
                    .padding(20)
                    .frame(maxWidth: .infinity, maxHeight: .infinity),
                colorScheme: .light,
                dynamicTypeSize: .accessibility3,
                width: 560,
                height: 460
            )
        )
        XCTAssertEqual(accessible.pixelWidth, 560)
        XCTAssertEqual(accessible.pixelHeight, 460)
        XCTAssertGreaterThan(accessible.png.count, 12_000)

        if let requestedPath = ProcessInfo.processInfo.environment[
            "LOOM_PROVIDER_UI_PREVIEW_PATH"
        ] {
            let requestedURL = try validatedPreviewURL(
                requestedPath: requestedPath,
                expectedURL: expectedProviderPreviewURL(
                    currentDirectory: FileManager.default.currentDirectoryPath
                )
            )
            try rendered.png.write(to: requestedURL, options: .atomic)
        }
    }

    func testExplicitRecoveryTargetMeetsNativeAccessibilityMinimum() {
        XCTAssertGreaterThanOrEqual(
            LoomDesign.minimumActionTarget,
            44
        )
    }

    func testMinimumLayoutHasNoRequiredMotion() async throws {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()
        let standard = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 720,
                height: 560
            )
        )
        XCTAssertEqual(standard.pixelWidth, 720)
        XCTAssertEqual(standard.pixelHeight, 560)
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .appendingPathComponent(
                "../../Sources/LoomLocalAppUI/ContentView.swift"
            )
            .standardizedFileURL
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        for requiredMotion in ["withAnimation", ".animation(", "TimelineView"] {
            XCTAssertFalse(
                source.contains(requiredMotion),
                "Primary workspace requires motion through \(requiredMotion)"
            )
        }
    }

    func testRealContentViewRendersNonconnectingStateAndAppearanceMatrix()
        async throws
    {
        let populated = try ExperienceFixtures.populatedSnapshot()
        let partial = try ExperienceFixtures.snapshot(partial: true)
        let stale = try ExperienceFixtures.snapshot(stale: true)

        let fixtures: [ExperienceViewFixture] = [
            .loading(),
            .connected(snapshot: .empty(viewVersion: "empty")),
            .connected(snapshot: populated),
            .connected(snapshot: partial),
            .connected(snapshot: stale),
            .offline(),
            .offlinePreserving(populated),
            .fatal(),
        ]

        var stateDigests = Set<String>()
        for fixture in fixtures {
            let store = LocalProductStore(client: fixture.client)
            for _ in 0..<fixture.refreshCount {
                await store.refresh()
            }
            XCTAssertEqual(
                LocalProductExperience(
                    snapshot: store.snapshot,
                    connectionState: store.connectionState
                ).state,
                fixture.expectedState
            )
            for scheme in [ColorScheme.light, .dark] {
                let rendered = render(
                    ContentView(
                        store: store,
                        refreshOnAppear: false
                    ),
                    colorScheme: scheme,
                    dynamicTypeSize: .large
                )
                let image = try XCTUnwrap(
                    rendered,
                    "\(fixture.name) failed in \(scheme)"
                )
                XCTAssertEqual(image.pixelWidth, 1_100)
                XCTAssertEqual(image.pixelHeight, 720)
                XCTAssertGreaterThan(image.png.count, 20_000)
                XCTAssertGreaterThan(image.colorBucketCount, 12)
                XCTAssertGreaterThan(
                    image.sidebarContrastPixelCount,
                    180,
                    "\(fixture.name) rendered a visually empty sidebar in \(scheme)"
                )
                stateDigests.insert(image.digest)
            }
        }
        XCTAssertEqual(stateDigests.count, fixtures.count * 2)

        let accessibilityStore = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(populated)])
        )
        await accessibilityStore.refresh()
        let accessibilityImage = try XCTUnwrap(
            render(
                ContentView(
                    store: accessibilityStore,
                    refreshOnAppear: false
                ),
                colorScheme: .light,
                dynamicTypeSize: .accessibility3,
                width: 780
            )
        )
        XCTAssertEqual(accessibilityImage.pixelWidth, 780)
        XCTAssertEqual(accessibilityImage.pixelHeight, 720)
        XCTAssertGreaterThan(accessibilityImage.png.count, 20_000)
        XCTAssertGreaterThan(accessibilityImage.colorBucketCount, 12)
        XCTAssertGreaterThan(
            accessibilityImage.sidebarContrastPixelCount,
            180
        )
        XCTAssertFalse(stateDigests.contains(accessibilityImage.digest))
    }

    func testExportPopulatedPreviewOnlyAtAuthorizedEvidencePath()
        async throws
    {
        guard let requestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_PREVIEW_PATH"
        ], let compactRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_COMPACT_PREVIEW_PATH"
        ] else {
            throw XCTSkip(
                "Wide and compact preview export is enabled only for visual audit"
            )
        }
        let requestedURL = try validatedPreviewURL(
            requestedPath: requestedPath,
            expectedURL: expectedPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let compactRequestedURL = try validatedPreviewURL(
            requestedPath: compactRequestedPath,
            expectedURL: expectedCompactPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )

        let snapshot = try ExperienceFixtures.populatedSnapshot()
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()
        guard let rendered = render(
            ContentView(
                store: store,
                refreshOnAppear: false
            ),
            colorScheme: .light,
            dynamicTypeSize: .large
        ) else {
            return XCTFail("Could not encode the non-connecting preview")
        }
        try rendered.png.write(to: requestedURL, options: .atomic)
        XCTAssertGreaterThan(rendered.png.count, 20_000)
        let compact = try XCTUnwrap(
            render(
                ContentView(
                    store: store,
                    refreshOnAppear: false
                ),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 780,
                height: 720
            )
        )
        try compact.png.write(to: compactRequestedURL, options: .atomic)
        XCTAssertGreaterThan(compact.png.count, 20_000)
        XCTAssertNotEqual(rendered.digest, compact.digest)
    }

    private func render<Root: View>(
        _ view: Root,
        colorScheme: ColorScheme,
        dynamicTypeSize: DynamicTypeSize,
        width: Int = 1_100,
        height: Int = 720
    ) -> RenderedPreview? {
        let rootView = view
                .frame(width: CGFloat(width), height: CGFloat(height))
                .environment(\.colorScheme, colorScheme)
                .environment(\.dynamicTypeSize, dynamicTypeSize)
        let hostingView = NSHostingView(rootView: rootView)
        hostingView.frame = NSRect(
            origin: .zero,
            size: CGSize(width: width, height: height)
        )
        hostingView.layoutSubtreeIfNeeded()
        guard let bitmap = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: width,
            pixelsHigh: height,
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0
        ) else {
            return nil
        }
        bitmap.size = hostingView.bounds.size
        hostingView.cacheDisplay(
            in: hostingView.bounds,
            to: bitmap
        )
        guard let png = bitmap.representation(
            using: .png,
            properties: [:]
        ) else {
            return nil
        }
        guard bitmap.pixelsWide == width,
              bitmap.pixelsHigh == height else {
            return nil
        }
        var buckets = Set<UInt32>()
        var sidebarContrastPixelCount = 0
        let step = 12
        for y in stride(from: 0, to: bitmap.pixelsHigh, by: step) {
            for x in stride(from: 0, to: bitmap.pixelsWide, by: step) {
                guard let color = bitmap.colorAt(x: x, y: y)?
                    .usingColorSpace(.deviceRGB) else {
                    continue
                }
                let red = UInt32((color.redComponent * 31).rounded())
                let green = UInt32((color.greenComponent * 31).rounded())
                let blue = UInt32((color.blueComponent * 31).rounded())
                let alpha = UInt32((color.alphaComponent * 31).rounded())
                buckets.insert(
                    red << 15 | green << 10 | blue << 5 | alpha
                )
            }
        }
        if let reference = bitmap.colorAt(x: 24, y: 100)?
            .usingColorSpace(.deviceRGB) {
            for y in stride(from: 100, to: min(620, height), by: 2) {
                for x in stride(from: 24, to: min(190, width), by: 2) {
                    guard let color = bitmap.colorAt(x: x, y: y)?
                        .usingColorSpace(.deviceRGB) else {
                        continue
                    }
                    let contrast =
                        abs(color.redComponent - reference.redComponent) +
                        abs(color.greenComponent - reference.greenComponent) +
                        abs(color.blueComponent - reference.blueComponent)
                    if contrast > 0.24 {
                        sidebarContrastPixelCount += 1
                    }
                }
            }
        }
        return RenderedPreview(
            png: png,
            digest: SHA256.hash(data: png)
                .map { String(format: "%02x", $0) }
                .joined(),
            colorBucketCount: buckets.count,
            sidebarContrastPixelCount: sidebarContrastPixelCount,
            pixelWidth: bitmap.pixelsWide,
            pixelHeight: bitmap.pixelsHigh
        )
    }

    private func validatedPreviewURL(
        requestedPath: String,
        expectedURL: URL
    ) throws -> URL {
        let requestedURL = URL(fileURLWithPath: requestedPath)
            .standardizedFileURL
        let standardizedExpectedURL = expectedURL.standardizedFileURL
        guard requestedURL == standardizedExpectedURL else {
            throw PreviewPathError.outsideOwnedEvidence
        }
        let expectedParent = standardizedExpectedURL
            .deletingLastPathComponent()
        guard expectedParent == expectedParent
            .resolvingSymlinksInPath()
            .standardizedFileURL else {
            throw PreviewPathError.symlinkEscape
        }
        if let values = try? requestedURL.resourceValues(
            forKeys: [.isSymbolicLinkKey]
        ), values.isSymbolicLink == true {
            throw PreviewPathError.symlinkEscape
        }
        return requestedURL
    }

    private func expectedPreviewURL(currentDirectory: String) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "interaction-continuity-wide.png"
            )
            .standardizedFileURL
    }

    private func expectedCompactPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "interaction-continuity-compact.png"
            )
            .standardizedFileURL
    }

    private func expectedProviderPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "provider-connection-directory.png"
            )
            .standardizedFileURL
    }

    func testPreviewPathValidationRejectsMismatchAndSymlink() throws {
        let currentDirectory = FileManager.default.currentDirectoryPath
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: "/private/tmp/not-owned.png",
                expectedURL: expectedPreviewURL(
                    currentDirectory: currentDirectory
                )
            )
        )

        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let outside = root.appendingPathComponent("outside.png")
        let expected = root.appendingPathComponent("expected.png")
        try FileManager.default.createSymbolicLink(
            at: expected,
            withDestinationURL: outside
        )
        defer { try? FileManager.default.removeItem(at: expected) }
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: expected.path,
                expectedURL: expected
            )
        )

        let outsideDirectory = root.appendingPathComponent("outside")
        try FileManager.default.createDirectory(
            at: outsideDirectory,
            withIntermediateDirectories: false
        )
        let linkedParent = root.appendingPathComponent("linked-parent")
        try FileManager.default.createSymbolicLink(
            at: linkedParent,
            withDestinationURL: outsideDirectory
        )
        let parentEscaped = linkedParent.appendingPathComponent("preview.png")
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: parentEscaped.path,
                expectedURL: parentEscaped
            )
        )
    }
}

private struct RenderedPreview {
    let png: Data
    let digest: String
    let colorBucketCount: Int
    let sidebarContrastPixelCount: Int
    let pixelWidth: Int
    let pixelHeight: Int
}

private enum PreviewPathError: Error {
    case outsideOwnedEvidence
    case symlinkEscape
}

private struct ExperienceViewFixture {
    let name: String
    let client: ExperienceViewStubClient
    let refreshCount: Int
    let expectedState: LocalProductExperienceState

    static func loading() -> Self {
        Self(
            name: "loading",
            client: ExperienceViewStubClient(
                results: [.success(.empty(viewVersion: "loading"))]
            ),
            refreshCount: 0,
            expectedState: .loading
        )
    }

    static func connected(snapshot: LocalProductSnapshot) -> Self {
        Self(
            name: snapshot.partial
                ? "partial"
                : snapshot.stale ? "stale" : "connected",
            client: ExperienceViewStubClient(results: [.success(snapshot)]),
            refreshCount: 1,
            expectedState: snapshot.partial
                ? .partial
                : snapshot.stale
                    ? .stalePreserved
                    : snapshot == .empty(viewVersion: "empty")
                        ? .connectedEmpty
                        : .connectedPopulated
        )
    }

    static func offline() -> Self {
        Self(
            name: "offline",
            client: ExperienceViewStubClient(
                results: [.failure(LocalProductClientError.unavailable)]
            ),
            refreshCount: 1,
            expectedState: .offline
        )
    }

    static func offlinePreserving(_ snapshot: LocalProductSnapshot) -> Self {
        Self(
            name: "offline-preserved",
            client: ExperienceViewStubClient(
                results: [
                    .success(snapshot),
                    .failure(LocalProductClientError.unavailable),
                ]
            ),
            refreshCount: 2,
            expectedState: .offlinePreserved
        )
    }

    static func fatal() -> Self {
        Self(
            name: "fatal",
            client: ExperienceViewStubClient(
                results: [
                    .failure(
                        LocalIPCRemoteError(
                            code: .unauthorizedPeer,
                            recoverable: false
                        )
                    ),
                ]
            ),
            refreshCount: 1,
            expectedState: .fatal
        )
    }
}

private final class ExperienceViewStubClient: LocalProductClientProtocol {
    private var results: [Result<LocalProductSnapshot, Error>]

    init(results: [Result<LocalProductSnapshot, Error>]) {
        self.results = results
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        guard !results.isEmpty else {
            throw LocalProductClientError.unavailable
        }
        return try results.removeFirst().get()
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }
}
