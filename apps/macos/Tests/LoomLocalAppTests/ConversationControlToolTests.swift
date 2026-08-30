import Foundation
import XCTest
@testable import LoomLocalAppCore

final class ConversationControlToolTests: XCTestCase {
    func testConversationActionProposalDecodesStrictGovernedMissionDraft() throws {
        let digest = String(repeating: "a", count: 64)
        let json = """
        {
          "schema_version":1,"proposal_id":"proposal-mission-1",
          "tool_id":"loom.missions.create.preview","tool_version":1,
          "confirmation":"user","action":"mission",
          "argument":"Ship the governed release",
          "target_conversation_id":"conversation-1","target_content_digest":"\(digest)",
          "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(proposal.action, .mission)
        XCTAssertEqual(proposal.argument, "Ship the governed release")
        XCTAssertEqual(proposal.status, .pending)

        let credentialShaped = json.replacingOccurrences(
            of: "Ship the governed release",
            with: "sk-secret-123456789012345678901234567890123456"
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(credentialShaped.utf8)
        ))
        let unknownField = json.replacingOccurrences(
            of: "\"argument\":",
            with: "\"secret\":\"forbidden\",\"argument\":"
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(unknownField.utf8)
        ))
    }

    func testConversationActionProposalFreezesExactRoutePayload() throws {
        let digest = String(repeating: "a", count: 64)
        let json = """
        {
          "schema_version":1,"proposal_id":"proposal-route-1",
          "tool_id":"loom.conversation.route.change.preview","tool_version":1,
          "confirmation":"user","action":"route","argument":"",
          "payload":{"profile_id":"conversation-deepseek-primary-r3"},
          "target_conversation_id":"conversation-1","target_content_digest":"\(digest)",
          "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(proposal.action, .route)
        XCTAssertEqual(
            proposal.payload?.profileID,
            "conversation-deepseek-primary-r3"
        )

        let substitutedTool = json.replacingOccurrences(
            of: "loom.conversation.route.change.preview",
            with: "loom.conversation.model.change.preview"
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(substitutedTool.utf8)
        ))
        let substitutedPayload = json.replacingOccurrences(
            of: "\"profile_id\":\"conversation-deepseek-primary-r3\"",
            with: "\"model_id\":\"deepseek-chat\""
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(substitutedPayload.utf8)
        ))
        let unknownPayloadField = json.replacingOccurrences(
            of: "\"profile_id\":",
            with: "\"credential_reference\":\"forbidden\",\"profile_id\":"
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(unknownPayloadField.utf8)
        ))
    }

    func testConversationActionProposalV2FreezesTurnAndExactMissionTarget() throws {
        let digest = String(repeating: "a", count: 64)
        let bindingDigest = String(repeating: "b", count: 64)
        let capsuleDigest = String(repeating: "c", count: 64)
        let segmentCapsuleDigest = String(repeating: "0", count: 64)
        let workspaceDigest = String(repeating: "d", count: 64)
        let registryDigest = String(repeating: "e", count: 64)
        let json = """
        {
          "schema_version":2,"proposal_id":"proposal-continue-1",
          "tool_id":"loom.missions.continue.preview","tool_version":2,
          "confirmation":"user","action":"continue_mission",
          "argument":"Retry only the failed verification step",
          "payload":{"mission_id":"mission-release-7"},
          "route":{"harness_adapter":"codex","provider_id":"openai",
            "provider_account_id":"openai.primary","credential_revision":4,
            "model_id":"gpt-5.6-sol","reasoning_effort":"max",
            "execution_binding_digest":"\(bindingDigest)",
            "context_capsule_digest":"\(capsuleDigest)"},
          "workspace":{"workspace_id":"workspace-primary",
            "workspace_digest":"\(workspaceDigest)"},
          "registry_digest":"\(registryDigest)","incident_id":"incident-continue-1",
          "target_conversation_id":"conversation-1","target_content_digest":"\(digest)",
          "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(proposal.schemaVersion, 2)
        XCTAssertEqual(proposal.payload?.missionID, "mission-release-7")
        XCTAssertEqual(proposal.route?.modelID, "gpt-5.6-sol")
        XCTAssertEqual(proposal.route?.contextCapsuleDigest, capsuleDigest)
        XCTAssertEqual(proposal.workspace?.workspaceID, "workspace-primary")
        XCTAssertEqual(proposal.registryDigest, registryDigest)
        XCTAssertEqual(proposal.incidentID, "incident-continue-1")

        let binding = LocalProductConversationExecutionBinding(
            schemaVersion: 4,
            harnessAdapter: "codex",
            providerID: "openai",
            providerAccountID: "openai.primary",
            credentialRevision: 4,
            modelID: "gpt-5.6-sol"
        )
        let segment = LocalProductConversationSegment(
            segmentID: "segment-1",
            profileID: "conversation-openai-primary-r4",
            contextMode: .summaryOnly,
            contextCapsuleDigest: segmentCapsuleDigest,
            executionBinding: binding,
            bindingDigest: bindingDigest,
            modelID: "gpt-5.6-sol",
            reasoningEffort: "max"
        )
        let attempt = LocalProductConversationAttempt(
            attemptID: "attempt-1",
            segmentID: "segment-1",
            profileID: "conversation-openai-primary-r4",
            modelID: "gpt-5.6-sol",
            reasoningEffort: "max",
            contextMode: .summaryOnly,
            contextCapsuleDigest: capsuleDigest,
            executionBinding: binding,
            bindingDigest: String(repeating: "f", count: 64),
            incidentID: "incident-continue-1",
            status: "succeeded",
            failureCode: ""
        )
        let thread = LocalProductChatThread(
            threadID: "conversation-1",
            profileID: "conversation-openai-primary-r4",
            controlWorkspace: proposal.workspace,
            segments: [segment], attempts: [attempt], messages: [],
            actionProposals: [proposal], canReply: true,
            requiresConfirmation: true
        )
        XCTAssertTrue(proposal.matchesFrozenTurn(in: thread))
        let driftedAttempt = LocalProductConversationAttempt(
            attemptID: "attempt-1", segmentID: "segment-1",
            profileID: "conversation-openai-primary-r4",
            modelID: "gpt-5.6-sol", reasoningEffort: "max",
            contextMode: .summaryOnly, contextCapsuleDigest: capsuleDigest,
            executionBinding: binding,
            bindingDigest: String(repeating: "f", count: 64),
            incidentID: "incident-other", status: "succeeded", failureCode: ""
        )
        let drifted = LocalProductChatThread(
            threadID: "conversation-1",
            profileID: "conversation-openai-primary-r4",
            controlWorkspace: proposal.workspace,
            segments: [segment], attempts: [driftedAttempt], messages: [],
            actionProposals: [proposal], canReply: true,
            requiresConfirmation: true
        )
        XCTAssertFalse(proposal.matchesFrozenTurn(in: drifted))

        let missingWorkspace = LocalProductChatThread(
            threadID: "conversation-1",
            profileID: "conversation-openai-primary-r4",
            segments: [segment], attempts: [attempt], messages: [],
            actionProposals: [proposal], canReply: true,
            requiresConfirmation: true
        )
        XCTAssertFalse(proposal.matchesFrozenTurn(in: missingWorkspace))

        let missingRoute = json.replacingOccurrences(
            of: "\"route\":{",
            with: "\"omitted_route\":{"
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(missingRoute.utf8)
        ))
        let unknownRouteField = json.replacingOccurrences(
            of: #""harness_adapter":"codex""#,
            with: #""secret":"forbidden","harness_adapter":"codex""#
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(unknownRouteField.utf8)
        ))
    }

    func testConversationActionDecisionReceiptRestoresExactConfirmedAuthority() throws {
        let proposalDigest = String(repeating: "a", count: 64)
        let bindingDigest = String(repeating: "b", count: 64)
        let capsuleDigest = String(repeating: "c", count: 64)
        let workspaceDigest = String(repeating: "d", count: 64)
        let registryDigest = String(repeating: "e", count: 64)
        let receiptDigest = String(repeating: "f", count: 64)
        let json = """
        {
          "thread_id":"conversation-1","profile_id":"codex",
          "segments":[],"attempts":[],"messages":[],
          "action_proposals":[{
            "schema_version":2,"proposal_id":"proposal-mission-1",
            "tool_id":"loom.missions.create.preview","tool_version":2,
            "confirmation":"user","action":"mission","argument":"Ship release 7",
            "route":{"harness_adapter":"codex","provider_id":"openai",
              "provider_account_id":"openai.primary","credential_revision":4,
              "model_id":"gpt-5.6-sol","reasoning_effort":"max",
              "execution_binding_digest":"\(bindingDigest)",
              "context_capsule_digest":"\(capsuleDigest)"},
            "workspace":{"workspace_id":"workspace-primary",
              "workspace_digest":"\(workspaceDigest)"},
            "registry_digest":"\(registryDigest)","incident_id":"incident-proposal-1",
            "target_conversation_id":"conversation-1",
            "target_content_digest":"\(proposalDigest)",
            "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
            "status":"confirmed","created_at":"2026-08-28T12:00:00Z",
            "expires_at":"2026-08-28T12:05:00Z",
            "proposal_digest":"\(proposalDigest)"
          }],
          "proposal_decision_receipts":[{
            "schema_version":1,"proposal_id":"proposal-mission-1",
            "proposal_digest":"\(proposalDigest)",
            "tool_id":"loom.missions.create.preview","decision":"confirm",
            "decision_incident_id":"incident-confirm-1",
            "target_conversation_id":"conversation-1",
            "segment_id":"segment-1","attempt_id":"attempt-1",
            "registry_digest":"\(registryDigest)",
            "workspace_digest":"\(workspaceDigest)",
            "execution_binding_digest":"\(bindingDigest)",
            "context_capsule_digest":"\(capsuleDigest)",
            "decided_at":"2026-08-28T12:01:00Z",
            "receipt_digest":"\(receiptDigest)"
          }],
          "can_reply":true,"requires_confirmation":false
        }
        """
        let thread = try JSONDecoder().decode(
            LocalProductChatThread.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(thread.proposalDecisionReceipts.count, 1)
        XCTAssertTrue(thread.hasConfirmedDecisionReceipt(for: thread.actionProposals[0]))
        XCTAssertEqual(
            thread.proposalDecisionReceipts[0].decisionIncidentID,
            "incident-confirm-1"
        )

        let unknownReceiptField = json.replacingOccurrences(
            of: #""schema_version":1,"proposal_id":"proposal-mission-1""#,
            with: #""schema_version":1,"secret":"forbidden","proposal_id":"proposal-mission-1""#
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductChatThread.self,
            from: Data(unknownReceiptField.utf8)
        ))
        let mismatchedDecision = json.replacingOccurrences(
            of: #""decision":"confirm""#,
            with: #""decision":"cancel""#
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductChatThread.self,
            from: Data(mismatchedDecision.utf8)
        ))
        var legacyObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any]
        )
        legacyObject.removeValue(forKey: "proposal_decision_receipts")
        let legacy = try JSONDecoder().decode(
            LocalProductChatThread.self,
            from: JSONSerialization.data(withJSONObject: legacyObject)
        )
        XCTAssertTrue(legacy.proposalDecisionReceipts.isEmpty)
        XCTAssertFalse(legacy.hasConfirmedDecisionReceipt(for: legacy.actionProposals[0]))
    }

    func testConversationAlignmentProposalV2FreezesTurnAuthority() throws {
        let digest = String(repeating: "a", count: 64)
        let sourceDigest = String(repeating: "b", count: 64)
        let json = """
        {
          "schema_version":2,"proposal_id":"proposal-align-2",
          "tool_id":"loom.sessions.align.preview","tool_version":2,
          "confirmation":"user","target_conversation_id":"s3",
          "target_content_digest":"\(digest)","sources":[{
            "conversation_id":"s1","title":"Session one",
            "content_digest":"\(sourceDigest)","message_count":2
          }],"context_mode":"summary_only","catalog_digest":"\(digest)",
          "route":{"harness_adapter":"codex","provider_id":"openai",
            "model_id":"gpt-5.6-sol","credential_revision":0,
            "execution_binding_digest":"\(digest)","context_capsule_digest":"\(sourceDigest)"},
          "workspace":{"workspace_id":"workspace-primary","workspace_digest":"\(digest)"},
          "registry_digest":"\(sourceDigest)","incident_id":"incident-align-2",
          "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let proposal = try JSONDecoder().decode(
            LocalProductConversationControlProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(proposal.schemaVersion, 2)
        XCTAssertEqual(proposal.toolVersion, 2)
        XCTAssertEqual(proposal.route?.executionBindingDigest, digest)
        XCTAssertEqual(proposal.workspace?.workspaceDigest, digest)
        XCTAssertEqual(proposal.incidentID, "incident-align-2")
        XCTAssertTrue(proposal.isConfirmable)
    }

    func testLegacyAlignmentProposalRemainsReadableButNotConfirmable() throws {
        let digest = String(repeating: "a", count: 64)
        let sourceDigest = String(repeating: "b", count: 64)
        let json = """
        {
          "schema_version":1,"proposal_id":"proposal-align-legacy",
          "tool_id":"loom.sessions.align.preview","tool_version":1,
          "confirmation":"user","target_conversation_id":"s3",
          "target_content_digest":"\(digest)","sources":[{
            "conversation_id":"s1","title":"Session one",
            "content_digest":"\(sourceDigest)","message_count":2
          }],"context_mode":"summary_only","catalog_digest":"\(digest)",
          "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let proposal = try JSONDecoder().decode(
            LocalProductConversationControlProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertFalse(proposal.isConfirmable)
    }

    func testConversationActionProposalRejectsCredentialShapedRoundTableGuidance()
        throws
    {
        let digest = String(repeating: "b", count: 64)
        let json = """
        {
          "schema_version":1,"proposal_id":"proposal-steer-1",
          "tool_id":"loom.roundtables.steer.preview","tool_version":1,
          "confirmation":"user","action":"roundtable_steer","argument":"",
          "payload":{"session_id":"session-1","round_id":"round-1",
            "seat_id":"seat-coder","attempt_id":"attempt-1",
            "guidance":"Focus the review on accessibility."},
          "target_conversation_id":"conversation-1","target_content_digest":"\(digest)",
          "segment_id":"segment-1","attempt_id":"attempt-control-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(proposal.payload?.seatID, "seat-coder")

        let credentialShaped = json.replacingOccurrences(
            of: "Focus the review on accessibility.",
            with: "Use Authorization: Bearer sk-secret-123456789012345678901234567890"
        )
        XCTAssertThrowsError(try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(credentialShaped.utf8)
        ))
    }

    func testLegacyUnboundRoundTableProposalRemainsReadableButNotConfirmable()
        throws
    {
        let digest = String(repeating: "b", count: 64)
        let bindingDigest = String(repeating: "c", count: 64)
        let capsuleDigest = String(repeating: "d", count: 64)
        let workspaceDigest = String(repeating: "e", count: 64)
        let registryDigest = String(repeating: "f", count: 64)
        let json = """
        {
          "schema_version":2,"proposal_id":"proposal-skip-legacy-v2",
          "tool_id":"loom.roundtables.skip.preview","tool_version":2,
          "confirmation":"user","action":"roundtable_skip","argument":"",
          "payload":{"session_id":"session-1","round_id":"round-1","seat_id":"seat-coder"},
          "route":{"harness_adapter":"codex","provider_id":"openai",
            "provider_account_id":"openai.primary","credential_revision":4,
            "model_id":"gpt-5.6-sol","reasoning_effort":"max",
            "execution_binding_digest":"\(bindingDigest)",
            "context_capsule_digest":"\(capsuleDigest)"},
          "workspace":{"workspace_id":"workspace-primary",
            "workspace_digest":"\(workspaceDigest)"},
          "registry_digest":"\(registryDigest)","incident_id":"incident-skip-legacy-v2",
          "target_conversation_id":"conversation-1","target_content_digest":"\(digest)",
          "segment_id":"segment-1","attempt_id":"attempt-control-1","message_id":"msg-1",
          "status":"pending","created_at":"2026-08-28T12:00:00Z",
          "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
        }
        """
        let legacy = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(json.utf8)
        )
        XCTAssertFalse(legacy.isConfirmable)

        let boundJSON = json.replacingOccurrences(
            of: "\"seat_id\":\"seat-coder\"",
            with: "\"seat_id\":\"seat-coder\",\"membership_revision\":7,"
                + "\"seat_binding_digest\":\"\(bindingDigest)\""
        )
        let bound = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(boundJSON.utf8)
        )
        XCTAssertTrue(bound.isConfirmable)
    }

    func testChatThreadDecodesGovernedAlignmentProposalAndReceipt() throws {
        let digest = String(repeating: "a", count: 64)
        let sourceDigest = String(repeating: "b", count: 64)
        let bindingDigest = String(repeating: "c", count: 64)
        let capsuleDigest = String(repeating: "d", count: 64)
        let receiptDigest = String(repeating: "e", count: 64)
        let json = """
        {
          "thread_id":"s3","profile_id":"codex",
          "segments":[{
            "segment_id":"segment-1","profile_id":"codex","context_mode":"start_clean",
            "context_capsule_digest":"\(capsuleDigest)","binding_digest":"\(bindingDigest)",
            "context_alignment_digest":"\(receiptDigest)","created_at":"2026-08-28T12:00:00Z"
          }],
          "attempts":[],
          "messages":[{
            "message_id":"msg-1","segment_id":"segment-1","role":"loom",
            "content":"Prepared for review","tentative":true,"created_at":"2026-08-28T12:00:00Z"
          }],
          "control_proposals":[{
            "schema_version":1,"proposal_id":"proposal-1","tool_id":"loom.sessions.align.preview",
            "tool_version":1,"confirmation":"user","target_conversation_id":"s3",
            "target_content_digest":"\(digest)","sources":[{
              "conversation_id":"s1","title":"Session one","content_digest":"\(sourceDigest)","message_count":2
            }],"context_mode":"summary_only","catalog_digest":"\(digest)",
            "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"msg-1",
            "status":"confirmed","created_at":"2026-08-28T12:00:00Z",
            "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
          }],
          "context_alignments":[{
            "schema_version":1,"alignment_id":"alignment-proposal-1","proposal_id":"proposal-1",
            "proposal_digest":"\(digest)","target_conversation_id":"s3","sources":[{
              "conversation_id":"s1","title":"Session one","content_digest":"\(sourceDigest)","message_count":2
            }],"context_mode":"summary_only","receipt_digest":"\(receiptDigest)",
            "confirmed_at":"2026-08-28T12:01:00Z","confirmation_incident_id":"incident-1",
            "applied_segment_id":"segment-1"
          }],
          "can_reply":true,"requires_confirmation":false
        }
        """
        let thread = try JSONDecoder().decode(
            LocalProductChatThread.self,
            from: Data(json.utf8)
        )
        XCTAssertEqual(thread.controlProposals.count, 1)
        XCTAssertEqual(thread.controlProposals[0].sources[0].title, "Session one")
        XCTAssertEqual(thread.controlProposals[0].status, .confirmed)
        XCTAssertEqual(thread.contextAlignments[0].appliedSegmentID, "segment-1")
        XCTAssertEqual(thread.segments[0].contextAlignmentDigest, receiptDigest)
    }

    func testChatMessageRequestEncodesBoundedSessionCatalog() throws {
        let request = LocalProductChatMessageRequest(
            threadID: "s3",
            content: "Align the other sessions",
            profileID: "codex",
            sessionCatalog: [
                LocalProductConversationSessionReference(
                    conversationID: "s1",
                    title: "Session one",
                    updatedAt: "2026-08-28T12:00:00.000Z"
                ),
            ]
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(request))
                as? [String: Any]
        )
        let catalog = try XCTUnwrap(object["session_catalog"] as? [[String: Any]])
        XCTAssertEqual(catalog.count, 1)
        XCTAssertEqual(catalog[0]["conversation_id"] as? String, "s1")
        XCTAssertEqual(catalog[0]["title"] as? String, "Session one")
    }

    func testControlDecisionRequestContainsNoTranscriptOrCredentialFields() throws {
        let request = LocalProductChatControlDecisionRequest(
            threadID: "s3",
            proposalID: "proposal-1",
            proposalDigest: String(repeating: "a", count: 64),
            decision: .confirm
        )
        let encoded = String(decoding: try JSONEncoder().encode(request), as: UTF8.self)
        XCTAssertTrue(encoded.contains("proposal_digest"))
        XCTAssertFalse(encoded.contains("content"))
        XCTAssertFalse(encoded.contains("credential"))
        XCTAssertFalse(encoded.contains("api_key"))
    }
}
