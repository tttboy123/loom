import Foundation

public struct LocalProductRoleOptionChoice:
    Equatable, Sendable, Identifiable
{
    public let id: String
    public let field: String
    public let responsibility: String
    public let runtime: String
    public let provider: String
    public let model: String
    public let auth: String
    public let isCurrent: Bool

    public static func all(
        setup: LocalProductSetupSnapshot,
        preview: LocalProductBuilderPreview
    ) -> [Self] {
        setup.roleOptions.compactMap { option in
            guard option.kind == "main" || option.kind == "subagent" else {
                return nil
            }
            let runtime = setup.runtimes.first {
                $0.runtimeInstanceID == option.runtimeInstanceID
            }
            let currentRole = preview.roles.first {
                $0.kind == option.kind &&
                    $0.agentDefinitionID == option.agentDefinitionID &&
                    $0.runtimeProfileID == option.runtimeProfileID &&
                    $0.runtime.runtimeInstanceID == option.runtimeInstanceID
            }
            let model = currentRole?.modelID.nonempty ??
                runtime?.modelID.nonempty ??
                runtime?.modelIDs.first ??
                "Configured model"
            let authMode = currentRole?.authMode.nonempty
            return Self(
                id: option.id,
                field: option.kind == "main"
                    ? "main_role"
                    : "subagent_role",
                responsibility: SafeText.sanitize(
                    option.responsibility,
                    limit: 96
                ),
                runtime: SafeText.sanitize(
                    runtime?.displayName ?? "Compatible Runtime",
                    limit: 96
                ),
                provider: authMode.map(LocalProductRoleReview.providerName) ??
                    "",
                model: SafeText.sanitize(model, limit: 96),
                auth: authMode.map {
                    LocalProductRoleReview.humanized(
                        $0,
                        fallback: "Configured"
                    )
                } ?? "",
                isCurrent: currentRole != nil
            )
        }
    }
}

public struct LocalProductRoleReview: Equatable, Sendable, Identifiable {
    public var id: String { "\(kind):\(name):\(model)" }
    public let kind: String
    public let name: String
    public let runtime: String
    public let provider: String
    public let model: String
    public let auth: String
    public let permissions: [String]
    public let compatibility: String

    public init(role: LocalProductBuilderRole) {
        kind = Self.humanized(role.kind, fallback: "Role")
        name = SafeText.sanitize(role.displayName, limit: 96)
        runtime = SafeText.sanitize(role.runtime.displayName, limit: 96)
        provider = Self.providerName(authMode: role.authMode)
        model = SafeText.sanitize(role.modelID, limit: 96)
        auth = Self.humanized(role.authMode, fallback: "Configured")
        permissions = role.permissionIDs.map {
            Self.humanized($0, fallback: "Permission")
        }
        compatibility = role.compatible
            ? "Compatible"
            : Self.humanized(
                role.compatibilityReason,
                fallback: "Needs review"
            )
    }

    fileprivate static func providerName(authMode: String) -> String {
        switch authMode {
        case "native_auth": return "Codex"
        case "brokered": return "MiniMax"
        default: return "Local provider"
        }
    }

    fileprivate static func humanized(
        _ value: String,
        fallback: String
    ) -> String {
        let safe = SafeText.sanitize(value, limit: 96)
            .replacingOccurrences(of: "_", with: " ")
            .replacingOccurrences(of: ".", with: " ")
        guard !safe.isEmpty else { return fallback }
        return safe.prefix(1).uppercased() + safe.dropFirst()
    }
}

private extension String {
    var nonempty: String? { isEmpty ? nil : self }
}

public struct LocalProductPreflightReview: Equatable, Sendable {
    public let roles: [LocalProductRoleReview]
    public let permissions: [String]
    public let compatibility: String
    public let maximumCost: String
    public let maximumBudget: String

    public init(preview: LocalProductBuilderPreview) {
        roles = preview.roles.map(LocalProductRoleReview.init)
        permissions = preview.permissions.map {
            LocalProductRoleReview.humanized(
                $0,
                fallback: "Permission"
            )
        }
        if preview.compatibilityGaps.isEmpty &&
            preview.roles.allSatisfy(\.compatible) {
            compatibility = "Compatible"
        } else {
            compatibility = "Needs review"
        }
        maximumCost = SafeText.sanitize(
            preview.estimatedMaximumCost,
            limit: 96
        )
        maximumBudget = "\(preview.maximumBudgetCredits) credits"
    }
}

public enum LocalProductInteractionCopy {
    public static let primaryFlow = [
        "New task",
        "Recent work",
        "What would you like Loom to help with?",
        "Build your team",
        "Choose a provider and model",
        "Review and save",
        "Team",
        "Context",
        "Changes",
        "Evidence",
        "Developer details",
        "No work has started yet.",
        "Your saved team is ready.",
    ]
}
