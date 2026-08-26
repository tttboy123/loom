import Foundation
import XCTest
@testable import LoomLocalAppCore

@MainActor
final class LocalConversationModelGatingTests: XCTestCase {
    private func setupSnapshot(
        openCodeProfile: Bool,
        deepSeekProfile: Bool,
        miniMaxProfile: Bool = false,
        zhipuProfile: Bool = false,
        openCodeModels: [String]? = nil
    ) throws -> LocalProductSetupSnapshot {
        var profiles: [String] = []
        if openCodeProfile {
            profiles.append(
                #"{"profile_id":"conversation-opencode-default","harness_adapter":"opencode","provider_id":"opencode","provider_account_id":"","display_name":"OpenCode","protocol":"opencode_agent","model_id":"opencode/deepseek-v4-flash-free","auth_mode":"native_auth","credential_revision":0}"#
            )
        }
        if deepSeekProfile {
            profiles.append(
                #"{"profile_id":"conversation-deepseek-deepseek-chat-r2","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":2}"#
            )
            if openCodeProfile {
                profiles.append(
                    #"{"profile_id":"conversation-opencode-deepseek-deepseek-primary-r2","harness_adapter":"opencode","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"opencode_agent","model_id":"deepseek/deepseek-chat","auth_mode":"brokered","credential_revision":2}"#
                )
            }
        }
        if miniMaxProfile {
            profiles.append(
                #"{"profile_id":"conversation-minimax-minimax-m3-r4","harness_adapter":"loom-native","provider_id":"minimax","provider_account_id":"minimax.primary","display_name":"MiniMax","protocol":"openai_compatible","model_id":"MiniMax-M3","auth_mode":"brokered","credential_revision":4}"#
            )
            if openCodeProfile {
                profiles.append(
                    #"{"profile_id":"conversation-opencode-minimax-minimax-primary-r4","harness_adapter":"opencode","provider_id":"minimax","provider_account_id":"minimax.primary","display_name":"MiniMax","protocol":"opencode_agent","model_id":"minimax-cn/MiniMax-M3","auth_mode":"brokered","credential_revision":4}"#
                )
            }
        }
        if zhipuProfile {
            profiles.append(
                #"{"profile_id":"conversation-zhipu-glm-5-r3","harness_adapter":"loom-native","provider_id":"zhipu","provider_account_id":"zhipu.primary","display_name":"Zhipu GLM","protocol":"openai_compatible","model_id":"glm-5","auth_mode":"brokered","credential_revision":3}"#
            )
            if openCodeProfile {
                profiles.append(
                    #"{"profile_id":"conversation-opencode-zhipu-zhipu-primary-r3","harness_adapter":"opencode","provider_id":"zhipu","provider_account_id":"zhipu.primary","display_name":"Zhipu GLM","protocol":"opencode_agent","model_id":"zai/glm-5.2","auth_mode":"brokered","credential_revision":3}"#
                )
            }
        }
        let runtimes: String
        if let openCodeModels {
            let modelJSON = openCodeModels.map { "\"\($0)\"" }.joined(separator: ",")
            runtimes = """
            {"runtime_instance_id":"runtime.opencode.local","display_name":"OpenCode","adapter_type":"opencode","executable_version":"sha256:current","status":"online","capacity":3,"model_ids":[\(modelJSON)],"observed_capabilities":["workspace_edit"],"source_probe_id":"probe.opencode.local"}
            """
        } else {
            runtimes = ""
        }
        let json = """
        {
          "schema_version":1,
          "view_version":"view-1",
          "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
          "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
          "providers":[],
          "conversation_profiles":[\(profiles.joined(separator: ","))],
          "runtimes":[\(runtimes)],"saved_teams":[],"templates":[],"role_options":[],
          "skills":[],"permissions":[],"resources":[]
        }
        """
        return try LocalProductSetupWire.decodeSnapshot(Data(json.utf8))
    }

    func testOpenCodeModelsComeFromInstalledRuntimeCatalog() throws {
        let snapshot = try setupSnapshot(
            openCodeProfile: true,
            deepSeekProfile: false,
            openCodeModels: [
                "opencode/big-pickle",
                "opencode/mimo-v2.5-free",
                "zai/glm-5.2",
            ]
        )
        let store = LocalProductStore(
            client: StubLocalProductClient(snapshots: []),
            initialSetupSnapshot: snapshot
        )
        XCTAssertEqual(
            store.conversationModels(providerID: "opencode").map(\.modelID),
            ["opencode/big-pickle", "opencode/mimo-v2.5-free", "zai/glm-5.2"]
        )
        XCTAssertFalse(
            store.conversationModels(providerID: "opencode").contains {
                $0.modelID == "opencode/deepseek-v4-flash-free"
            }
        )
        XCTAssertEqual(store.effectiveConversationModelID, "opencode/big-pickle")
    }

    func testConversationModelGatingRequiresVerifiedOwningProvider() throws {
        let snapshot = try setupSnapshot(
            openCodeProfile: true,
            deepSeekProfile: true
        )
        let store = LocalProductStore(
            client: StubLocalProductClient(snapshots: []),
            initialSetupSnapshot: snapshot
        )
        // OpenCode's own hosted free-tier model is native and always selectable.
        XCTAssertTrue(
            store.isConversationModelAvailable(
                providerID: "opencode",
                modelID: "opencode/deepseek-v4-flash-free"
            )
        )
        // Native OpenCode cannot silently borrow a DeepSeek account.
        XCTAssertFalse(
            store.isConversationModelAvailable(
                profile: store.selectedConversationProfile,
                modelID: "deepseek/deepseek-chat"
            )
        )
        // MiniMax is NOT verified -> its model must not be selectable yet, and
        // the user gets an actionable reason instead of a send-time failure.
        XCTAssertFalse(
            store.isConversationModelAvailable(
                providerID: "opencode",
                modelID: "minimax-cn/MiniMax-M3"
            )
        )
        XCTAssertNotNil(
            store.conversationModelUnavailableReason(
                providerID: "opencode",
                modelID: "minimax-cn/MiniMax-M3"
            )
        )
        // zai (GLM) has no verified account either.
        XCTAssertFalse(
            store.isConversationModelAvailable(
                providerID: "opencode",
                modelID: "zai/glm-5.2"
            )
        )
        // Plain models under a verified Provider bucket are selectable.
        XCTAssertTrue(
            store.isConversationModelAvailable(
                providerID: "deepseek",
                modelID: "deepseek-v4-pro"
            )
        )
        // The account-scoped OpenCode profile owns only DeepSeek models.
        store.selectConversationProfile(
            "conversation-opencode-deepseek-deepseek-primary-r2"
        )
        XCTAssertTrue(
            store.isConversationModelAvailable(
                profile: store.selectedConversationProfile,
                modelID: "deepseek/deepseek-chat"
            )
        )
        let deepSeekModels = store.conversationModels(
            profile: store.selectedConversationProfile
        ).map(\.modelID)
        XCTAssertTrue(deepSeekModels.contains("deepseek/deepseek-chat"))
        XCTAssertTrue(deepSeekModels.allSatisfy { $0.hasPrefix("deepseek/") })
        store.selectConversationModel("minimax-cn/MiniMax-M3")
        XCTAssertNotEqual(
            store.selectedConversationModelID,
            "minimax-cn/MiniMax-M3"
        )
        store.selectConversationModel("deepseek/deepseek-chat")
        XCTAssertEqual(
            store.selectedConversationModelID,
            "deepseek/deepseek-chat"
        )
    }

    func testOpenCodeRuntimePrefixesResolveToLoomProviderAccounts() throws {
        let snapshot = try setupSnapshot(
            openCodeProfile: true,
            deepSeekProfile: false,
            miniMaxProfile: true,
            zhipuProfile: true
        )
        let store = LocalProductStore(
            client: StubLocalProductClient(snapshots: []),
            initialSetupSnapshot: snapshot
        )

        XCTAssertEqual(
            store.conversationModelOwnerProvider(
                providerID: "opencode", modelID: "minimax-cn/MiniMax-M3"
            ),
            "minimax"
        )
        XCTAssertEqual(
            store.conversationModelOwnerProvider(
                providerID: "opencode", modelID: "zai/glm-5.2"
            ),
            "zhipu"
        )
        store.selectConversationProfile(
            "conversation-opencode-minimax-minimax-primary-r4"
        )
        XCTAssertTrue(
            store.isConversationModelAvailable(
                profile: store.selectedConversationProfile,
                modelID: "minimax-cn/MiniMax-M3"
            )
        )
        store.selectConversationProfile(
            "conversation-opencode-zhipu-zhipu-primary-r3"
        )
        XCTAssertTrue(
            store.isConversationModelAvailable(
                profile: store.selectedConversationProfile,
                modelID: "zai/glm-5.2"
            )
        )
    }

    func testEffectiveConversationModelFallsBackWhenOwningAccountUnavailable() throws {
        let snapshot = try setupSnapshot(
            openCodeProfile: true,
            deepSeekProfile: false
        )
        let store = LocalProductStore(
            client: StubLocalProductClient(snapshots: []),
            initialSetupSnapshot: snapshot
        )
        // Simulate a stale user selection whose owning Provider was revoked:
        // the effective model must fall back to a usable native model instead
        // of sending into an unverified Provider.
        store.selectedConversationModelID = "deepseek/deepseek-chat"
        XCTAssertEqual(
            store.effectiveConversationModelID,
            "opencode/big-pickle"
        )
        // With no selection the profile default (native free tier) is used.
        store.selectedConversationModelID = ""
        XCTAssertEqual(
            store.effectiveConversationModelID,
            "opencode/big-pickle"
        )
    }

    func testEffectiveConversationModelIgnoresStaleCrossProviderSelection() throws {
        let snapshot = try setupSnapshot(
            openCodeProfile: false,
            deepSeekProfile: false,
            miniMaxProfile: true
        )
        let store = LocalProductStore(
            client: StubLocalProductClient(snapshots: []),
            initialSetupSnapshot: snapshot
        )
        // A stale "deepseek-chat" selection must not display on the MiniMax
        // conversation even though MiniMax is verified: the model is not in
        // the MiniMax catalog, so the effective model stays MiniMax-M3.
        store.selectedConversationModelID = "deepseek-chat"
        XCTAssertEqual(store.effectiveConversationModelID, "MiniMax-M3")
        XCTAssertEqual(store.selectedConversationProfile?.providerID, "minimax")
    }

    func testSelectConversationModelSurfacesActionableNoticeWhenRefused() throws {
        let snapshot = try setupSnapshot(
            openCodeProfile: true,
            deepSeekProfile: false
        )
        let store = LocalProductStore(
            client: StubLocalProductClient(snapshots: []),
            initialSetupSnapshot: snapshot
        )
        // zai/GLM has no verified account: selecting it is refused and the
        // user sees an actionable notice instead of a silent no-op.
        store.selectConversationModel("zai/glm-4.5")
        XCTAssertEqual(store.selectedConversationModelID, "")
        XCTAssertNotNil(store.conversationModelSelectionNotice)
        XCTAssertTrue(
            store.conversationModelSelectionNotice?.contains("zhipu") ?? false
        )
        // Selecting a usable model clears the notice.
        store.selectConversationModel("opencode/big-pickle")
        XCTAssertNil(store.conversationModelSelectionNotice)
        XCTAssertEqual(
            store.selectedConversationModelID,
            "opencode/big-pickle"
        )
    }
}
