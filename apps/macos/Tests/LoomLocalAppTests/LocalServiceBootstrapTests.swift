import XCTest
@testable import LoomLocalAppCore

final class LocalServiceBootstrapTests: XCTestCase {
    func testRegistersMissingBundledService() {
        let registration = StubLocalServiceRegistration(status: .notRegistered)

        let outcome = LocalServiceBootstrapper(registration: registration).prepare()

        XCTAssertEqual(outcome, .registered)
        XCTAssertEqual(registration.registerCalls, 1)
    }

    func testDoesNotReregisterEnabledService() {
        let registration = StubLocalServiceRegistration(status: .enabled)

        let outcome = LocalServiceBootstrapper(registration: registration).prepare()

        XCTAssertEqual(outcome, .enabled)
        XCTAssertEqual(registration.registerCalls, 0)
    }

    func testPreservesApprovalRequirement() {
        let registration = StubLocalServiceRegistration(status: .requiresApproval)

        let outcome = LocalServiceBootstrapper(registration: registration).prepare()

        XCTAssertEqual(outcome, .requiresApproval)
        XCTAssertEqual(registration.registerCalls, 0)
    }
}

private final class StubLocalServiceRegistration: LocalServiceRegistration {
    let status: LocalServiceRegistrationStatus
    private(set) var registerCalls = 0

    init(status: LocalServiceRegistrationStatus) {
        self.status = status
    }

    func register() throws {
        registerCalls += 1
    }
}
