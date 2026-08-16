import Foundation
import ServiceManagement

public enum LocalServiceRegistrationStatus: Equatable, Sendable {
    case notRegistered
    case enabled
    case requiresApproval
    case notFound
}

public enum LocalServiceBootstrapOutcome: Equatable, Sendable {
    case enabled
    case registered
    case requiresApproval
    case unavailable

    public var shouldWaitForConnection: Bool {
        self == .enabled || self == .registered
    }
}

public protocol LocalServiceRegistration: AnyObject {
    var status: LocalServiceRegistrationStatus { get }
    func register() throws
}

public struct LocalServiceBootstrapper {
    private let registration: any LocalServiceRegistration

    public init(registration: any LocalServiceRegistration) {
        self.registration = registration
    }

    public func prepare() -> LocalServiceBootstrapOutcome {
        switch registration.status {
        case .enabled:
            return .enabled
        case .requiresApproval:
            return .requiresApproval
        case .notFound:
            return .unavailable
        case .notRegistered:
            do {
                try registration.register()
                return .registered
            } catch {
                return registration.status == .requiresApproval
                    ? .requiresApproval
                    : .unavailable
            }
        }
    }
}

public final class SystemLocalServiceRegistration: LocalServiceRegistration {
    private let service: SMAppService

    public init(
        plistName: String = "com.earendilworks.loom.local.daemon.plist"
    ) {
        service = SMAppService.agent(plistName: plistName)
    }

    public var status: LocalServiceRegistrationStatus {
        switch service.status {
        case .notRegistered: return .notRegistered
        case .enabled: return .enabled
        case .requiresApproval: return .requiresApproval
        case .notFound: return .notFound
        @unknown default: return .notFound
        }
    }

    public func register() throws {
        try service.register()
    }

    public static func openApprovalSettings() {
        SMAppService.openSystemSettingsLoginItems()
    }
}
