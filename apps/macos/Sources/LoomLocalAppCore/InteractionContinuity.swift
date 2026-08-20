import Foundation

public struct LocalProductRoleOptionChoice:
    Equatable, Sendable, Identifiable
{
    public let id: String
    public let field: String
	public let agentDefinitionID: String
	public let runtimeProfileID: String
    public let responsibility: String
    public let runtime: String
	public let harness: String
    public let provider: String
	public let providerAccount: String
    public let model: String
    public let auth: String
	public let credential: String
	public let reasoning: String
	public let timeout: String
	public let budget: String
	public let capabilities: [String]
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
			let model = option.modelID.nonempty ?? currentRole?.modelID.nonempty ??
                runtime?.modelID.nonempty ??
                runtime?.modelIDs.first ??
                "Configured model"
			let authMode = option.authMode.nonempty ?? currentRole?.authMode.nonempty
            return Self(
                id: option.id,
                field: option.kind == "main"
                    ? "main_role"
                    : "subagent_role",
				agentDefinitionID: option.agentDefinitionID,
				runtimeProfileID: option.runtimeProfileID,
                responsibility: SafeText.sanitize(
                    option.responsibility,
                    limit: 96
                ),
                runtime: SafeText.sanitize(
                    runtime?.displayName ?? "Compatible Runtime",
                    limit: 96
                ),
				harness: LocalProductRoleReview.humanized(
					option.harnessAdapter,
					fallback: "Configured harness"
				),
				provider: LocalProductRoleReview.providerName(
					providerID: option.providerID,
					authMode: authMode ?? ""
				),
				providerAccount: SafeText.sanitize(
					option.providerAccountID,
					limit: 96
				),
                model: SafeText.sanitize(model, limit: 96),
                auth: authMode.map {
                    LocalProductRoleReview.humanized(
                        $0,
                        fallback: "Configured"
                    )
                } ?? "",
				credential: LocalProductRoleReview.credentialLabel(
					revision: option.credentialRevision,
					authMode: authMode ?? ""
				),
				reasoning: LocalProductRoleReview.reasoningLabel(
					effort: option.reasoningEffort
				),
				timeout: LocalProductRoleReview.timeoutLabel(
					milliseconds: option.timeoutMilliseconds
				),
				budget: LocalProductRoleReview.budgetLabel(
					available: option.budgetAvailable,
					units: option.budgetUnits
				),
				capabilities: option.requiredCapabilities.map {
					LocalProductRoleReview.humanized($0, fallback: "Capability")
				},
                isCurrent: currentRole != nil
            )
        }
    }

	public static func fallbacks(
		setup: LocalProductSetupSnapshot,
		role: LocalProductBuilderRole
	) -> [Self] {
		setup.roleOptions.compactMap { option in
			guard option.kind == role.kind,
				option.agentDefinitionID == role.agentDefinitionID,
				option.runtimeProfileID != role.runtimeProfileID else {
				return nil
			}
			let runtime = setup.runtimes.first {
				$0.runtimeInstanceID == option.runtimeInstanceID
			}
			let authMode = option.authMode.nonempty
			let model = option.modelID.nonempty ?? runtime?.modelID.nonempty ??
				runtime?.modelIDs.first ?? "Configured model"
			return Self(
				id: option.id,
				field: role.kind == "main"
					? "main_fallback_role"
					: "subagent_fallback_role",
				agentDefinitionID: option.agentDefinitionID,
				runtimeProfileID: option.runtimeProfileID,
				responsibility: SafeText.sanitize(option.responsibility, limit: 96),
				runtime: SafeText.sanitize(
					runtime?.displayName ?? "Compatible Runtime",
					limit: 96
				),
				harness: LocalProductRoleReview.humanized(
					option.harnessAdapter,
					fallback: "Configured harness"
				),
				provider: LocalProductRoleReview.providerName(
					providerID: option.providerID,
					authMode: authMode ?? ""
				),
				providerAccount: SafeText.sanitize(
					option.providerAccountID,
					limit: 96
				),
				model: SafeText.sanitize(model, limit: 96),
				auth: authMode.map {
					LocalProductRoleReview.humanized($0, fallback: "Configured")
				} ?? "",
				credential: LocalProductRoleReview.credentialLabel(
					revision: option.credentialRevision,
					authMode: authMode ?? ""
				),
				reasoning: LocalProductRoleReview.reasoningLabel(
					effort: option.reasoningEffort
				),
				timeout: LocalProductRoleReview.timeoutLabel(
					milliseconds: option.timeoutMilliseconds
				),
				budget: LocalProductRoleReview.budgetLabel(
					available: option.budgetAvailable,
					units: option.budgetUnits
				),
				capabilities: option.requiredCapabilities.map {
					LocalProductRoleReview.humanized($0, fallback: "Capability")
				},
				isCurrent: role.fallbackConfigured &&
					option.runtimeProfileID == role.fallbackRuntimeProfileID
			)
		}
	}

	public static func routes(
		setup: LocalProductSetupSnapshot,
		role: LocalProductBuilderRole,
		field: String
	) -> [Self] {
		let allowedField: String
		switch (role.kind, field) {
		case ("main", "main_harness_route"),
			("main", "main_provider_account_route"),
			("subagent", "subagent_harness_route"),
			("subagent", "subagent_provider_account_route"):
			allowedField = field
		default:
			return []
		}
		return setup.roleOptions.compactMap { option in
			guard option.kind == role.kind,
				option.agentDefinitionID == role.agentDefinitionID else {
				return nil
			}
			if allowedField.hasSuffix("_harness_route") {
				guard option.providerID == role.providerID,
					option.providerAccountID == role.providerAccountID,
					option.authMode == role.authMode,
					option.credentialRevision == role.credentialRevision,
					option.modelID == role.modelID else {
					return nil
				}
			} else {
				guard option.harnessAdapter == role.harnessAdapter,
					option.runtimeInstanceID == role.runtime.runtimeInstanceID else {
					return nil
				}
			}
			let runtime = setup.runtimes.first {
				$0.runtimeInstanceID == option.runtimeInstanceID
			}
			let authMode = option.authMode.nonempty
			let model = option.modelID.nonempty ?? runtime?.modelID.nonempty ??
				runtime?.modelIDs.first ?? "Configured model"
			let isCurrent: Bool
			if allowedField.hasSuffix("_harness_route") {
				isCurrent = option.harnessAdapter == role.harnessAdapter &&
					option.runtimeInstanceID == role.runtime.runtimeInstanceID
			} else {
				isCurrent = option.providerID == role.providerID &&
					option.providerAccountID == role.providerAccountID &&
					option.authMode == role.authMode &&
					option.credentialRevision == role.credentialRevision &&
					option.modelID == role.modelID
			}
			return Self(
				id: option.id,
				field: allowedField,
				agentDefinitionID: option.agentDefinitionID,
				runtimeProfileID: option.runtimeProfileID,
				responsibility: SafeText.sanitize(option.responsibility, limit: 96),
				runtime: SafeText.sanitize(
					runtime?.displayName ?? "Compatible Runtime",
					limit: 96
				),
				harness: LocalProductRoleReview.humanized(
					option.harnessAdapter,
					fallback: "Configured harness"
				),
				provider: LocalProductRoleReview.providerName(
					providerID: option.providerID,
					authMode: authMode ?? ""
				),
				providerAccount: SafeText.sanitize(
					option.providerAccountID,
					limit: 96
				),
				model: SafeText.sanitize(model, limit: 96),
				auth: authMode.map {
					LocalProductRoleReview.humanized($0, fallback: "Configured")
				} ?? "",
				credential: LocalProductRoleReview.credentialLabel(
					revision: option.credentialRevision,
					authMode: authMode ?? ""
				),
				reasoning: LocalProductRoleReview.reasoningLabel(
					effort: option.reasoningEffort
				),
				timeout: LocalProductRoleReview.timeoutLabel(
					milliseconds: option.timeoutMilliseconds
				),
				budget: LocalProductRoleReview.budgetLabel(
					available: option.budgetAvailable,
					units: option.budgetUnits
				),
				capabilities: option.requiredCapabilities.map {
					LocalProductRoleReview.humanized($0, fallback: "Capability")
				},
				isCurrent: isCurrent
			)
		}
	}
}

public struct LocalProductRoleReview: Equatable, Sendable, Identifiable {
    public var id: String { "\(kind):\(name):\(model)" }
    public let kind: String
    public let name: String
    public let runtime: String
	public let harness: String
    public let provider: String
	public let providerAccount: String
    public let model: String
    public let auth: String
	public let credential: String
	public let reasoning: String
	public let timeout: String
	public let budget: String
	public let capabilities: [String]
	public let fallback: String
	public let fallbackAuth: String
	public let fallbackCredential: String
	public let fallbackReasoning: String
	public let fallbackTimeout: String
	public let fallbackBudget: String
	public let fallbackCapabilities: [String]
	public let fallbackApproval: String
    public let permissions: [String]
    public let compatibility: String

    public init(role: LocalProductBuilderRole) {
        kind = Self.humanized(role.kind, fallback: "Role")
        name = SafeText.sanitize(role.displayName, limit: 96)
        runtime = SafeText.sanitize(role.runtime.displayName, limit: 96)
		harness = Self.humanized(role.harnessAdapter, fallback: "Configured harness")
		provider = Self.providerName(
			providerID: role.providerID,
			authMode: role.authMode
		)
		providerAccount = SafeText.sanitize(role.providerAccountID, limit: 96)
        model = SafeText.sanitize(role.modelID, limit: 96)
        auth = Self.humanized(role.authMode, fallback: "Configured")
		credential = Self.credentialLabel(
			revision: role.credentialRevision,
			authMode: role.authMode
		)
		reasoning = Self.reasoningLabel(effort: role.reasoningEffort)
		timeout = Self.timeoutLabel(milliseconds: role.timeoutMilliseconds)
		budget = Self.budgetLabel(
			available: role.budgetAvailable,
			units: role.budgetUnits
		)
		capabilities = role.requiredCapabilities.map {
			Self.humanized($0, fallback: "Capability")
		}
		if role.fallbackConfigured {
			let fallbackProvider = Self.providerName(
				providerID: role.fallbackProviderID,
				authMode: role.fallbackAuthMode
			)
			fallback = [
				Self.humanized(
					role.fallbackHarnessAdapter,
					fallback: "Configured harness"
				),
				role.fallbackProviderAccountID.isEmpty
					? fallbackProvider
					: "\(fallbackProvider) / \(SafeText.sanitize(role.fallbackProviderAccountID, limit: 96))",
				SafeText.sanitize(role.fallbackModelID, limit: 96),
			].filter { !$0.isEmpty }.joined(separator: " · ")
			fallbackAuth = Self.humanized(
				role.fallbackAuthMode,
				fallback: "Configured"
			)
			fallbackCredential = Self.credentialLabel(
				revision: role.fallbackCredentialRevision,
				authMode: role.fallbackAuthMode
			)
			fallbackReasoning = Self.reasoningLabel(
				effort: role.fallbackReasoningEffort
			)
			fallbackTimeout = Self.timeoutLabel(
				milliseconds: role.fallbackTimeoutMilliseconds
			)
			fallbackBudget = Self.budgetLabel(
				available: role.fallbackBudgetAvailable,
				units: role.fallbackBudgetUnits
			)
			fallbackCapabilities = role.fallbackRequiredCapabilities.map {
				Self.humanized($0, fallback: "Capability")
			}
			fallbackApproval = role.fallbackApprovalRequired
				? "Approval required"
				: "Approval missing"
		} else {
			fallback = ""
			fallbackAuth = ""
			fallbackCredential = ""
			fallbackReasoning = ""
			fallbackTimeout = ""
			fallbackBudget = ""
			fallbackCapabilities = []
			fallbackApproval = ""
		}
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

	static func providerName(
		providerID: String,
		authMode: String
	) -> String {
		switch providerID {
		case "openai": return "OpenAI"
		case "anthropic": return "Anthropic"
		case "deepseek": return "DeepSeek"
		case "moonshot": return "Moonshot / Kimi"
		case "minimax": return "MiniMax"
		case "opencode": return "OpenCode"
		default:
			if !providerID.isEmpty {
				return humanized(providerID, fallback: "Configured provider")
			}
		}
        switch authMode {
        case "native_auth": return "Codex"
        case "brokered": return "MiniMax"
        default: return "Local provider"
        }
    }

	fileprivate static func credentialLabel(
		revision: Int64,
		authMode: String
	) -> String {
		if revision > 0 { return "Credential v\(revision)" }
		return authMode == "native_auth" ? "Native credential" : "Credential pending"
	}

	fileprivate static func timeoutLabel(milliseconds: Int64) -> String {
		guard milliseconds > 0 else { return "Timeout pending" }
		if milliseconds % 1_000 == 0 {
			return "\(milliseconds / 1_000)s timeout"
		}
		return "\(milliseconds)ms timeout"
	}

	fileprivate static func reasoningLabel(effort: String) -> String {
		let value = humanized(effort, fallback: "")
		return value.isEmpty ? "Provider default" : "\(value) reasoning"
	}

	fileprivate static func budgetLabel(
		available: Bool,
		units: Int64
	) -> String {
		available ? "\(units) units" : "No profile budget"
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
