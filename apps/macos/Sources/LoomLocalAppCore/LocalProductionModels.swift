import Foundation

// C-W1 production surface for the native app. Read-only: renders the
// Journal-authoritative activation state and recovery status; activation and
// deactivation happen through the TUI / Go service layer with explicit diff
// confirmation.

public struct ProductionRecoveryStatus: Codable, Equatable, Sendable {
    public var degraded: Bool
    public var reason: String?
    public var lastActivated: Bool
    public var configDigestMismatch: Bool

    public init(
        degraded: Bool = false,
        reason: String? = nil,
        lastActivated: Bool = false,
        configDigestMismatch: Bool = false
    ) {
        self.degraded = degraded
        self.reason = reason
        self.lastActivated = lastActivated
        self.configDigestMismatch = configDigestMismatch
    }

    enum CodingKeys: String, CodingKey {
        case degraded, reason
        case lastActivated = "last_activated"
        case configDigestMismatch = "config_digest_mismatch"
    }
}

public struct ProductionSnapshot: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var activated: Bool
    public var activatedAt: String?
    public var targetMode: String?
    public var recovery: ProductionRecoveryStatus
    public var lastPreview: String?

    public init(
        viewVersion: String = "",
        activated: Bool = false,
        activatedAt: String? = nil,
        targetMode: String? = nil,
        recovery: ProductionRecoveryStatus = ProductionRecoveryStatus(),
        lastPreview: String? = nil
    ) {
        self.viewVersion = viewVersion
        self.activated = activated
        self.activatedAt = activatedAt
        self.targetMode = targetMode
        self.recovery = recovery
        self.lastPreview = lastPreview
    }

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case activated
        case activatedAt = "activated_at"
        case targetMode = "target_mode"
        case recovery
        case lastPreview = "last_preview"
    }
}

public enum ProductionWire {
    public static func decodeSnapshot(_ data: Data) throws -> ProductionSnapshot {
        do {
            return try JSONDecoder().decode(ProductionSnapshot.self, from: data)
        } catch {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public protocol LocalProductProductionSnapshotClientProtocol {
    func productionSnapshot(journeyID: String) async throws -> ProductionSnapshot
}

extension LocalIPCClient: LocalProductProductionSnapshotClientProtocol {}
