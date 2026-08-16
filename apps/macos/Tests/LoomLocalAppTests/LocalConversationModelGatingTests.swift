import Foundation
import XCTest
@testable import LoomLocalAppCore

@MainActor
final class LocalConversationModelGatingTests: XCTestCase {
    private func setupSnapshot(
        openCodeProfile: Bool,
        deepSeekProfile: Bool,
        miniMaxProfile: Bool = false
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
        }
        if miniMaxProfile {
            profiles.append(
                #"{"profile_id":"conversation-minimax-minimax-m3-r4","harness_adapter":"loom-native","provider_id":"minimax","provider_account_id":"minimax.primary","display_name":"MiniMax","protocol":"openai_compatible","model_id":"MiniMax-M3","auth_mode":"brokered","credential_revision":4}"#
            )
        }
        let json = """
        {
          "schema_version":1,
          "view_version":"view-1",
          "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
          "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
          "providers":[],
          "conversation_profiles":[\(profiles.joined(separator: ","))],
          "runtimes":[],"saved_teams":[],"templates":[],"role_options":[],
          "skills":[],"permissions":[],"resources":[]
        }
        """
        return try LocalProductSetupWire.decodeSnapshot(Data(json.utf8))
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
        // DeepSeek is verified -> its model under the OpenCode bucket is usable.
        XCTAssertTrue(
            store.isConversationModelAvailable(
                providerID: "opencode",
                modelID: "deepseek/deepseek-chat"
            )
        )
        // MiniMax is NOT verified -> its model must not be selectable yet, and
        // the user gets an actionable reason instead of a send-time failure.
        XCTAssertFalse(
            store.isConversationModelAvailable(
                providerID: "opencode",
                modelID: "minimax/MiniMax-M3"
            )
        )
        XCTAssertNotNil(
            store.conversationModelUnavailableReason(
                providerID: "opencode",
                modelID: "minimax/MiniMax-M3"
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
        // selectConversationModel refuses an unavailable model.
        store.selectConversationModel("minimax/MiniMax-M3")
        XCTAssertNotEqual(
            store.selectedConversationModelID,
            "minimax/MiniMax-M3"
        )
        store.selectConversationModel("deepseek/deepseek-chat")
        XCTAssertEqual(
            store.selectedConversationModelID,
            "deepseek/deepseek-chat"
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
            "opencode/deepseek-v4-flash-free"
        )
        // With no selection the profile default (native free tier) is used.
        store.selectedConversationModelID = ""
        XCTAssertEqual(
            store.effectiveConversationModelID,
            "opencode/deepseek-v4-flash-free"
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
            store.conversationModelSelectionNotice?.contains("zai") ?? false
        )
        // Selecting a usable model clears the notice.
        store.selectConversationModel("opencode/deepseek-v4-flash-free")
        XCTAssertNil(store.conversationModelSelectionNotice)
        XCTAssertEqual(
            store.selectedConversationModelID,
            "opencode/deepseek-v4-flash-free"
        )
    }
}
