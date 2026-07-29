import XCTest
@testable import LoomLocalAppCore

final class SafeTextTests: XCTestCase {
    func testSanitizeRemovesTerminalControlsAndBidiOverrides() {
        let input = "ok\u{001B}[31m red\u{001B}[0m\u{0000}\u{0007}\u{202E}done"

        XCTAssertEqual(SafeText.sanitize(input), "ok reddone")
    }

    func testSanitizeBoundsRenderedCharacters() {
        XCTAssertEqual(
            SafeText.sanitize("abcdef", limit: 4),
            "abcd…"
        )
        XCTAssertEqual(SafeText.sanitize("abc", limit: 4), "abc")
    }

    func testSanitizeRemovesOperatingSystemCommandPayload() {
        let input = "safe\u{001B}]0;hidden title\u{0007}done"
        XCTAssertEqual(SafeText.sanitize(input), "safedone")
    }

    func testSanitizeNormalizesSingleLineWhitespace() {
        XCTAssertEqual(
            SafeText.sanitize("alpha\nbeta\tgamma\rdelta"),
            "alpha beta gammadelta"
        )
    }
}
