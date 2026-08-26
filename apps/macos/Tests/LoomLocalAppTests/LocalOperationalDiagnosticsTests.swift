import Foundation
import XCTest

@testable import LoomLocalAppCore

final class LocalOperationalDiagnosticsTests: XCTestCase {
    func testMissionExecutionRecordPersistsStageWithoutMissionContent() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-mission-diagnostics-\(UUID().uuidString)")
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let store = try LocalOperationalDiagnostics(
            directory: root,
            maximumBytes: 2_048,
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )

        try store.recordMissionExecution(
            incidentID: "loom-mission-11111111-1111-4111-8111-111111111111",
            executionOperation: "start",
            stage: .dispatchAdmission,
            result: "failed",
            errorCode: .conflict,
            retryable: true,
            elapsedMilliseconds: 17
        )

        let data = try Data(
            contentsOf: root.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains("\"operation\":\"mission_execution\""))
        XCTAssertTrue(text.contains("\"execution_operation\":\"start\""))
        XCTAssertTrue(text.contains("\"stage\":\"dispatch_admission\""))
        XCTAssertFalse(text.contains("objective"))
        XCTAssertFalse(text.contains("workspace"))
        XCTAssertFalse(text.contains("mission_id"))
        XCTAssertFalse(text.contains("team_instance_id"))
    }

    func testProviderAccountPolicyInputAdmissionPersistsSafeCorrelation() async throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-policy-diag-\(UUID().uuidString.prefix(8))")
        let socketDirectory = root.appendingPathComponent("run")
        let diagnosticDirectory = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: socketDirectory,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let diagnostics = try LocalOperationalDiagnostics(
            directory: diagnosticDirectory,
            maximumBytes: 2_048,
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )
        let incidentID = "loom-policy-11111111-1111-4111-8111-111111111111"
        let client = try LocalIPCClient(
            socketPath: socketDirectory.appendingPathComponent("loomd.sock").path,
            requestID: { incidentID },
            operationalDiagnostics: diagnostics
        )

        do {
            _ = try await client.configureProviderAccountPolicy(
                providerID: "deepseek",
                providerAccountID: "openai.work",
                expectedRevision: 0,
                maximumConcurrentAttempts: 4,
                dispatchWindowSeconds: 60,
                maximumDispatchStarts: 20,
                maximumAssignedBudgetUnits: 12_000,
                trustDomain: "external_provider",
                retentionMode: "provider_default",
                dataRegion: "global"
            )
            XCTFail("expected account scope rejection")
        } catch let error as LocalIPCRemoteError {
            XCTAssertEqual(error.incidentID, incidentID)
            XCTAssertEqual(error.stage, .inputAdmission)
            XCTAssertEqual(error.code, .invalidRequest)
        }

        let data = try Data(
            contentsOf: diagnosticDirectory.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains("provider_account_policy_configure"))
        XCTAssertTrue(text.contains("input_admission"))
        XCTAssertFalse(text.contains("maximum_concurrent_attempts"))
        XCTAssertFalse(text.contains("maximum_assigned_budget_units"))
    }

    func testCredentialRecordIsPrivateBoundedAndContainsOnlySafeFields() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-app-diagnostics-\(UUID().uuidString)")
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let store = try LocalOperationalDiagnostics(
            directory: root,
            maximumBytes: 1_024,
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )

        try store.recordCredential(
            incidentID: "loom-swift-11111111-1111-4111-8111-111111111111",
            operation: "credential_configure",
            providerID: "deepseek",
            providerAccountID: "deepseek.work",
            stage: .inputAdmission,
            result: "failed",
            errorCode: .invalidRequest,
            retryable: false,
            elapsedMilliseconds: 3
        )

        let path = root.appendingPathComponent("app-operational.jsonl")
        let attributes = try FileManager.default.attributesOfItem(atPath: path.path)
        XCTAssertEqual((attributes[.posixPermissions] as? NSNumber)?.intValue, 0o600)
        let data = try Data(contentsOf: path)
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertFalse(text.contains("secret"))
        XCTAssertFalse(text.contains("authorization"))
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        XCTAssertEqual(object["incident_id"] as? String, "loom-swift-11111111-1111-4111-8111-111111111111")
        XCTAssertEqual(object["provider_id"] as? String, "deepseek")
        XCTAssertEqual(object["provider_account_id"] as? String, "deepseek.work")
        XCTAssertEqual(object["stage"] as? String, "input_admission")
        XCTAssertEqual(object["error_code"] as? String, "invalid_request")
    }

    func testCredentialInputAdmissionPersistsTheClientIncidentID() async throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-diag-\(UUID().uuidString.prefix(8))")
        let socketDirectory = root.appendingPathComponent("run")
        let diagnosticDirectory = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: socketDirectory,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let diagnostics = try LocalOperationalDiagnostics(
            directory: diagnosticDirectory,
            maximumBytes: 1_024,
            now: { Date() }
        )
        let incidentID = "loom-swift-22222222-2222-4222-8222-222222222222"
        let client = try LocalIPCClient(
            socketPath: socketDirectory.appendingPathComponent("loomd.sock").path,
            requestID: { incidentID },
            operationalDiagnostics: diagnostics
        )

        do {
            _ = try await client.configureCredential(
                providerID: "deepseek",
                secret: "sk-deep\nseek"
            )
            XCTFail("expected input admission failure")
        } catch let error as LocalIPCRemoteError {
            XCTAssertEqual(error.incidentID, incidentID)
            XCTAssertEqual(error.stage, .inputAdmission)
        }

        let data = try Data(
            contentsOf: diagnosticDirectory.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains("input_admission"))
        XCTAssertTrue(text.contains("deepseek.primary"))
        XCTAssertFalse(text.contains("sk-deep"))
    }

    func testConversationFailurePersistsSafeCorrelationWithoutContent() async throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-chat-diag-\(UUID().uuidString.prefix(8))")
        let socketDirectory = root.appendingPathComponent("run")
        let diagnosticDirectory = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: socketDirectory,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let diagnostics = try LocalOperationalDiagnostics(
            directory: diagnosticDirectory,
            maximumBytes: 2_048,
            now: { Date() }
        )
        let incidentID = "loom-chat-33333333-3333-4333-8333-333333333333"
        let client = try LocalIPCClient(
            socketPath: socketDirectory.appendingPathComponent("loomd.sock").path,
            requestID: { "unused" },
            operationalDiagnostics: diagnostics
        )

        do {
            _ = try await client.sendChatMessage(
                threadID: "thread-deepseek-r3",
                content: "private conversation text",
                profileID: "conversation-deepseek-deepseek-chat-r3",
                incidentID: incidentID
            )
            XCTFail("expected local transport failure")
        } catch let error as LocalIPCRemoteError {
            XCTAssertEqual(error.incidentID, incidentID)
            XCTAssertEqual(error.stage, .udsTransport)
            XCTAssertTrue(error.recoverable)
        }

        let data = try Data(
            contentsOf: diagnosticDirectory.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains("thread-deepseek-r3"))
        XCTAssertTrue(text.contains("conversation-deepseek-deepseek-chat-r3"))
        XCTAssertTrue(text.contains("uds_transport"))
        XCTAssertFalse(text.contains("private conversation text"))
        XCTAssertFalse(text.contains("\"content\""))
    }

    func testAgentInputRecordPersistsCorrelationWithoutInputContent() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-agent-input-diag-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let store = try LocalOperationalDiagnostics(
            directory: root,
            maximumBytes: 2_048,
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )
        let incidentID = "loom-agent-input-55555555-5555-4555-8555-555555555555"
        let privateContent = "private agent instruction must not appear"
        let request = LocalProductAgentInputRequest(
            segmentID: "segment-agent-1",
            agentInstanceID: "agent-instance-1",
            workItemID: "work-item-1",
            runID: "run-1",
            claimGeneration: 7,
            mode: .steer,
            content: Data(privateContent.utf8)
        )

        try store.recordAgentInput(
            incidentID: incidentID,
            request: request,
            stage: .agentInputAdmission,
            result: "failed",
            errorCode: .stateUnavailable,
            retryable: true,
            elapsedMilliseconds: 12
        )

        let data = try Data(
            contentsOf: root.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains("segment-agent-1"))
        XCTAssertTrue(text.contains("agent-instance-1"))
        XCTAssertTrue(text.contains("\"mode\":\"steer\""))
        XCTAssertTrue(text.contains("agent_input_admission"))
        XCTAssertFalse(text.contains(privateContent))
        XCTAssertFalse(text.contains("\"content\""))
    }

    func testAgentRecoveryTransportFailurePersistsOnlySafeCorrelation() async throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-agent-recovery-diag-\(UUID().uuidString.prefix(8))")
        let socketDirectory = root.appendingPathComponent("run")
        let diagnosticDirectory = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: socketDirectory,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let diagnostics = try LocalOperationalDiagnostics(
            directory: diagnosticDirectory,
            maximumBytes: 2_048,
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )
        let incidentID = "loom-agent-recovery-66666666-6666-4666-8666-666666666666"
        let client = try LocalIPCClient(
            socketPath: socketDirectory.appendingPathComponent("loomd.sock").path,
            requestID: { "unused" },
            operationalDiagnostics: diagnostics
        )

        do {
            _ = try await client.recoverAgentAttempt(.preview, incidentID: incidentID)
            XCTFail("expected local transport failure")
        } catch let error as LocalIPCRemoteError {
            XCTAssertEqual(error.incidentID, incidentID)
            XCTAssertEqual(error.stage, .udsTransport)
            XCTAssertTrue(error.recoverable)
        }

        let data = try Data(
            contentsOf: diagnosticDirectory.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains("agent_attempt_recovery"))
        XCTAssertTrue(text.contains("\"recovery_operation\":\"preview\""))
        XCTAssertTrue(text.contains("uds_transport"))
        XCTAssertFalse(text.contains("candidate_digest"))
        XCTAssertFalse(text.contains("capability_digest"))
        XCTAssertFalse(text.contains("principal_id"))
        XCTAssertFalse(text.contains("prompt"))
        XCTAssertFalse(text.contains("provider_response"))
    }

    func testToolRecoveryTransportFailurePersistsOnlySafeCorrelation() async throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-tool-recovery-diag-\(UUID().uuidString.prefix(8))")
        let socketDirectory = root.appendingPathComponent("run")
        let diagnosticDirectory = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: socketDirectory,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let diagnostics = try LocalOperationalDiagnostics(
            directory: diagnosticDirectory,
            maximumBytes: 2_048,
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )
        let incidentID = "loom-tool-recovery-77777777-7777-4777-8777-777777777777"
        let client = try LocalIPCClient(
            socketPath: socketDirectory.appendingPathComponent("loomd.sock").path,
            requestID: { "unused" },
            operationalDiagnostics: diagnostics
        )

        do {
            _ = try await client.recoverToolCall(.preview, incidentID: incidentID)
            XCTFail("expected local transport failure")
        } catch let error as LocalIPCRemoteError {
            XCTAssertEqual(error.incidentID, incidentID)
            XCTAssertEqual(error.stage, .udsTransport)
            XCTAssertTrue(error.recoverable)
        }

        let data = try Data(
            contentsOf: diagnosticDirectory.appendingPathComponent("app-operational.jsonl")
        )
        let text = try XCTUnwrap(String(data: data, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains("tool_recovery"))
        XCTAssertTrue(text.contains("\"recovery_operation\":\"preview\""))
        XCTAssertTrue(text.contains("uds_transport"))
        XCTAssertFalse(text.contains("candidate_digest"))
        XCTAssertFalse(text.contains("decision_id"))
        XCTAssertFalse(text.contains("principal_id"))
        XCTAssertFalse(text.contains("prompt"))
        XCTAssertFalse(text.contains("provider_response"))
    }

    func testFailedRecordStillRequiresSafeIdentityAndProviderFields() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-app-diagnostics-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let store = try LocalOperationalDiagnostics(
            directory: root,
            maximumBytes: 1_024,
            now: { Date() }
        )

        XCTAssertThrowsError(
            try store.recordCredential(
                incidentID: "../../unsafe",
                operation: "credential_configure",
                providerID: "deepseek",
                stage: .inputAdmission,
                result: "failed",
                errorCode: .invalidRequest,
                retryable: false,
                elapsedMilliseconds: 0
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: root.appendingPathComponent("app-operational.jsonl").path
            )
        )
    }

	func testConversationProviderFailurePersistsSafeHTTPClassification() throws {
		let root = FileManager.default.temporaryDirectory
			.appendingPathComponent("loom-chat-provider-diag-\(UUID().uuidString)")
		defer { try? FileManager.default.removeItem(at: root) }
		let store = try LocalOperationalDiagnostics(
			directory: root,
			maximumBytes: 2_048,
			now: { Date(timeIntervalSince1970: 1_786_276_800) }
		)

		try store.recordConversation(
			incidentID: "loom-chat-44444444-4444-4444-8444-444444444444",
			threadID: "thread-deepseek-r6",
			profileID: "conversation-deepseek-deepseek-chat-r6",
			stage: .providerRateLimit,
			result: "failed",
			errorCode: .providerRateLimit,
			httpStatus: 429,
			providerErrorCode: "rate_limit_exceeded",
			retryAfterSeconds: 18,
			retryable: true,
			elapsedMilliseconds: 950
		)

		let data = try Data(
			contentsOf: root.appendingPathComponent("app-operational.jsonl")
		)
		let text = try XCTUnwrap(String(data: data, encoding: .utf8))
		XCTAssertTrue(text.contains("\"http_status\":429"))
		XCTAssertTrue(text.contains("\"provider_error_code\":\"rate_limit_exceeded\""))
		XCTAssertTrue(text.contains("\"retry_after_seconds\":18"))
		XCTAssertFalse(text.contains("Provider rate limit reached"))
	}
}
