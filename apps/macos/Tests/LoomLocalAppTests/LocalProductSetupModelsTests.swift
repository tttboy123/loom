import Foundation
import Testing
@testable import LoomLocalAppCore

@Test
func nativeAuthConnectResultAcceptsExplicitCancellation() throws {
    let result = try LocalProductSetupWire.decodeProviderConnectResult(
        Data(
            #"{"provider_id":"claude-code","auth_mode":"native_auth","status":"cancelled"}"#.utf8
        )
    )
    #expect(result.providerID == "claude-code")
    #expect(result.status == "cancelled")
}

@Test
func conversationProfileRejectsCrossProviderAccountAndRevisionDrift() throws {
    let valid =
        #"{"profile_id":"conversation-deepseek-deepseek-chat-account-work-r7","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.work","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":7}"#
    let profile = try JSONDecoder().decode(
        LocalProductConversationProfile.self,
        from: Data(valid.utf8)
    )
    #expect(profile.providerAccountID == "deepseek.work")
    #expect(profile.credentialRevision == 7)

    for invalid in [
        valid.replacingOccurrences(
            of: "\"deepseek.work\"",
            with: "\"openai.work\""
        ),
        valid.replacingOccurrences(
            of: "\"credential_revision\":7",
            with: "\"credential_revision\":0"
        ),
    ] {
        #expect(throws: LocalProductWireError.invalidJSON) {
            try JSONDecoder().decode(
                LocalProductConversationProfile.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func setupSnapshotDecodesStrictCanonicalCollections() throws {
    let payload = Data(
        """
        {
          "schema_version": 1,
          "view_version": "\(String(repeating: "a", count: 64))",
          "codex": {
            "provider_id": "codex",
            "auth_mode": "native_auth",
            "credential_reference": "",
            "revision": 0,
            "status": "available",
            "reason": ""
          },
          "minimax": {
            "provider_id": "minimax",
            "auth_mode": "brokered",
            "credential_reference": "",
            "revision": 0,
            "status": "unconfigured",
            "reason": ""
          },
          "provider_accounts": [{
            "provider_id": "deepseek",
            "provider_account_id": "deepseek.work",
            "auth_mode": "brokered",
            "credential_reference": "credential-ref-deepseek-work",
            "revision": 3,
            "status": "verified",
            "reason": "",
            "policy_available": true,
            "policy_version": 2,
            "policy_revision": 2,
            "policy_digest": "\(String(repeating: "b", count: 64))",
            "maximum_concurrent_attempts": 3,
            "dispatch_window_seconds": 60,
            "maximum_dispatch_starts": 12,
            "maximum_assigned_budget_units": 8000,
            "trust_domain": "external_provider",
            "retention_mode": "zero_data_retention",
            "data_region": "apac"
          }],
          "credential_import_candidates": [{
            "candidate_id": "\(String(repeating: "c", count: 64))",
            "candidate_digest": "\(String(repeating: "d", count: 64))",
            "source_application": "CC Switch",
            "display_name": "DeepSeek",
            "target_provider_id": "deepseek",
            "protocol": "openai_responses",
            "endpoint": "https://api.deepseek.com",
            "endpoint_fingerprint": "\(String(repeating: "e", count: 64))",
            "model_ids": ["deepseek-chat"],
            "import_mode": "exact_provider",
            "current": true,
            "credential_available": true,
            "review_policy_version": 1,
            "review_policy_digest": "\(String(repeating: "f", count: 64))"
          }],
          "credential_vault": {
            "schema_version": 1,
            "status": "unlocked",
            "storage_mode": "local_key_file",
            "migration_required_accounts": 0,
            "recovery_required_accounts": 0
          },
          "runtimes": [{
            "runtime_instance_id": "runtime-pi",
            "display_name": "Pi Coding Agent",
            "adapter_type": "pi",
            "executable_version": "0.82.1",
            "status": "online",
            "capacity": 2,
            "model_ids": ["model-a"],
            "observed_capabilities": ["rpc"],
            "source_probe_id": "pi-probe"
          }],
          "saved_teams": [{
            "id": "team-review",
            "version": 1,
            "name": "Review Team",
            "status": "active",
            "definition_digest": "\(String(repeating: "d", count: 64))",
            "stream_head": 3
          }],
          "templates": [],
          "role_options": [],
          "skills": [],
          "permissions": [],
          "resources": []
        }
        """.utf8
    )

    let snapshot = try LocalProductSetupWire.decodeSnapshot(payload)
    #expect(snapshot.schemaVersion == 1)
    #expect(snapshot.codex.authMode == "native_auth")
    #expect(snapshot.miniMax.authMode == "brokered")
    #expect(snapshot.providerAccounts.first?.providerAccountID == "deepseek.work")
    #expect(snapshot.providerAccounts.first?.revision == 3)
    #expect(snapshot.providerAccounts.first?.policyAvailable == true)
    #expect(snapshot.providerAccounts.first?.policyVersion == 2)
    #expect(snapshot.providerAccounts.first?.policyRevision == 2)
    #expect(snapshot.providerAccounts.first?.maximumConcurrentAttempts == 3)
    #expect(snapshot.providerAccounts.first?.dispatchWindowSeconds == 60)
    #expect(snapshot.providerAccounts.first?.maximumDispatchStarts == 12)
    #expect(snapshot.providerAccounts.first?.maximumAssignedBudgetUnits == 8_000)
    #expect(snapshot.providerAccounts.first?.disclosurePolicyConfigured == true)
    #expect(snapshot.providerAccounts.first?.retentionMode == "zero_data_retention")
    #expect(snapshot.credentialImportCandidates.first?.displayName == "DeepSeek")
    #expect(snapshot.credentialImportCandidates.first?.candidateDigest == String(repeating: "d", count: 64))
    #expect(snapshot.credentialImportCandidates.first?.endpointFingerprint == String(repeating: "e", count: 64))
    #expect(snapshot.credentialImportCandidates.first?.modelIDs == ["deepseek-chat"])
    #expect(snapshot.credentialImportCandidates.first?.importMode == "exact_provider")
    #expect(snapshot.credentialImportCandidates.first?.reviewPolicyVersion == 1)
    #expect(snapshot.credentialImportCandidates.first?.reviewPolicyDigest == String(repeating: "f", count: 64))
    #expect(snapshot.credentialVault?.status == "unlocked")
    #expect(snapshot.credentialVault?.storageMode == "local_key_file")
    #expect(snapshot.credentialVault?.migrationRequiredAccounts == 0)
    #expect(snapshot.credentialVault?.recoveryRequiredAccounts == 0)
    #expect(snapshot.runtimes.first?.executableVersion == "0.82.1")
    #expect(snapshot.runtimes.first?.modelIDs == ["model-a"])
    #expect(snapshot.savedTeams.first?.streamHead == 3)
    #expect(snapshot.templates.isEmpty)
}

@Test
func setupRuntimeRejectsMalformedAuthoritativeProjectionValues() throws {
    let valid =
        #"{"runtime_instance_id":"runtime-pi","display_name":"Pi Coding Agent","adapter_type":"pi","executable_version":"0.82.1","status":"online","capacity":2,"model_id":"model-a","model_ids":["model-a"],"observed_capabilities":["rpc"],"source_probe_id":"pi-probe"}"#

    for adapter in ["pi", "pi-cli", "loom-native", "codex", "claude-code", "opencode"] {
        let candidate = valid.replacingOccurrences(
            of: #""adapter_type":"pi""#,
            with: #""adapter_type":"\#(adapter)""#
        )
        #expect(throws: Never.self) {
            try JSONDecoder().decode(
                LocalProductSetupRuntime.self,
                from: Data(candidate.utf8)
            )
        }
    }

    let invalid = [
        valid.replacingOccurrences(of: "runtime-pi", with: ""),
        valid.replacingOccurrences(of: "Pi Coding Agent", with: #"Pi\nAgent"#),
        valid.replacingOccurrences(of: #""adapter_type":"pi""#, with: #""adapter_type":"remote-plugin""#),
        valid.replacingOccurrences(of: #""status":"online""#, with: #""status":"starting""#),
        valid.replacingOccurrences(of: #""capacity":2"#, with: #""capacity":0"#),
        valid.replacingOccurrences(of: #""capacity":2"#, with: #""capacity":-1"#),
        valid.replacingOccurrences(of: #""model_ids":["model-a"]"#, with: #""model_ids":[" model-a"]"#),
        valid.replacingOccurrences(of: #""model_ids":["model-a"]"#, with: #""model_ids":["model-a","model-a"]"#),
        valid.replacingOccurrences(of: #""observed_capabilities":["rpc"]"#, with: #""observed_capabilities":["bad capability"]"#),
        valid.replacingOccurrences(of: #""observed_capabilities":["rpc"]"#, with: #""observed_capabilities":["rpc","rpc"]"#),
        valid.replacingOccurrences(of: "pi-probe", with: ""),
    ]
    for candidate in invalid {
        #expect(throws: LocalProductWireError.invalidJSON) {
            try JSONDecoder().decode(
                LocalProductSetupRuntime.self,
                from: Data(candidate.utf8)
            )
        }
    }
}

@Test
func credentialImportCandidateIsStrictBoundedAndNonSecret() throws {
    let candidateID = String(repeating: "c", count: 64)
    let candidateDigest = String(repeating: "d", count: 64)
    let endpointFingerprint = String(repeating: "e", count: 64)
    let reviewPolicyDigest = String(repeating: "f", count: 64)
    let valid =
        #"{"candidate_id":"\#(candidateID)","candidate_digest":"\#(candidateDigest)","source_application":"CC Switch","display_name":"DeepSeek","target_provider_id":"deepseek","protocol":"openai_responses","endpoint":"https://api.deepseek.com","endpoint_fingerprint":"\#(endpointFingerprint)","model_ids":["deepseek-chat"],"import_mode":"exact_provider","current":true,"credential_available":true,"review_policy_version":1,"review_policy_digest":"\#(reviewPolicyDigest)"}"#
    let candidate = try JSONDecoder().decode(
        LocalProductCredentialImportCandidate.self,
        from: Data(valid.utf8)
    )
    #expect(candidate.id == candidateID)
    #expect(candidate.candidateDigest == candidateDigest)
    #expect(candidate.targetProviderID == "deepseek")
    #expect(candidate.endpointFingerprint == endpointFingerprint)
    #expect(candidate.reviewPolicyVersion == 1)
    #expect(candidate.reviewPolicyDigest == reviewPolicyDigest)

    let unknownField = valid.replacingOccurrences(
        of: "\"credential_available\":true",
        with: "\"credential_available\":true,\"secret\":\"forbidden\""
    )
    #expect(throws: LocalProductWireError.unknownField) {
        try JSONDecoder().decode(
            LocalProductCredentialImportCandidate.self,
            from: Data(unknownField.utf8)
        )
    }

    for invalid in [
        valid.replacingOccurrences(
            of: "https://api.deepseek.com",
            with: "http://127.0.0.1:9000"
        ),
        valid.replacingOccurrences(
            of: "\"exact_provider\"",
            with: "\"silent_replace\""
        ),
        valid.replacingOccurrences(
            of: "\"candidate_digest\":\"\(candidateDigest)\"",
            with: "\"candidate_digest\":\"ABC\""
        ),
        valid.replacingOccurrences(
            of: "\"endpoint_fingerprint\":\"\(endpointFingerprint)\"",
            with: "\"endpoint_fingerprint\":\"\(String(repeating: "E", count: 64))\""
        ),
        valid.replacingOccurrences(
            of: "\"review_policy_version\":1",
            with: "\"review_policy_version\":0"
        ),
        valid.replacingOccurrences(
            of: "\"review_policy_digest\":\"\(reviewPolicyDigest)\"",
            with: "\"review_policy_digest\":\"\(String(repeating: "g", count: 64))\""
        ),
        valid.replacingOccurrences(
            of: "\"display_name\":\"DeepSeek\"",
            with: "\"display_name\":\"\(String(repeating: "x", count: 129))\""
        ),
        valid.replacingOccurrences(
            of: "\"model_ids\":[\"deepseek-chat\"]",
            with: "\"model_ids\":[\"\(String(repeating: "m", count: 257))\"]"
        ),
    ] {
        #expect(throws: LocalProductWireError.invalidJSON) {
            try JSONDecoder().decode(
                LocalProductCredentialImportCandidate.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func endpointReviewResultIsStrictApprovedAndNonSecret() throws {
    let digest = String(repeating: "a", count: 64)
    let payload = Data(
        """
        {
          "candidate_id":"\(digest)",
          "candidate_digest":"\(digest)",
          "authority_candidate_digest":"\(digest)",
          "provider_id":"custom-openai",
          "provider_account_id":"custom-openai.primary",
          "protocol":"openai_responses",
          "endpoint":"https://gateway.example.com/v1",
          "endpoint_fingerprint":"\(digest)",
          "model_digest":"\(digest)",
          "review_policy_version":1,
          "review_policy_digest":"\(digest)",
          "status":"approved",
          "revision":2,
          "approval_digest":"\(digest)",
          "approved_at":"2026-08-23T05:00:00Z",
          "expires_at":"2026-08-23T05:15:00Z"
        }
        """.utf8
    )
    let result = try LocalProductSetupWire.decodeEndpointReviewResult(payload)
    #expect(result.status == "approved")
    #expect(result.providerAccountID == "custom-openai.primary")
    for invalid in [
        String(data: payload, encoding: .utf8)!.replacingOccurrences(
            of: "\"status\":\"approved\"", with: "\"status\":\"requested\""
        ),
        String(data: payload, encoding: .utf8)!.replacingOccurrences(
            of: "\"revision\":2", with: "\"revision\":1"
        ),
        String(data: payload, encoding: .utf8)!.replacingOccurrences(
            of: "\"expires_at\"", with: "\"secret\":\"forbidden\",\"expires_at\""
        ),
    ] {
        #expect(throws: Error.self) {
            try LocalProductSetupWire.decodeEndpointReviewResult(Data(invalid.utf8))
        }
    }
}

@Test
func failureLabResultRequiresFailedTargetAndHealthyPeer() throws {
    let payload = Data(
        """
        {
          "schema_version":1,
          "scenario":"rate_limit",
          "incident_id":"incident-failure-lab",
          "agents":[
            {"agent_id":"target-agent","provider_account_id":"failurelab_tmp_0123456789abcdef0123456789abcdef","status":"failed","stage":"provider_rate_limit","code":"provider_rate_limit","retryable":true,"elapsed_milliseconds":1},
            {"agent_id":"healthy-peer","provider_account_id":"failurelab_tmp_fedcba9876543210fedcba9876543210","status":"succeeded","retryable":false,"elapsed_milliseconds":0}
          ]
        }
        """.utf8
    )
    let result = try LocalProductSetupWire.decodeFailureLabResult(payload)
    #expect(result.agents[0].retryable)
    #expect(result.agents[1].status == "succeeded")
    let pollutedPeer = String(data: payload, encoding: .utf8)!.replacingOccurrences(
        of: "\"status\":\"succeeded\",\"retryable\"",
        with: "\"status\":\"failed\",\"stage\":\"provider_auth\",\"code\":\"provider_auth\",\"retryable\""
    )
    #expect(throws: Error.self) {
        try LocalProductSetupWire.decodeFailureLabResult(Data(pollutedPeer.utf8))
    }

    let mismatchedScenario = String(data: payload, encoding: .utf8)!
        .replacingOccurrences(of: "\"scenario\":\"rate_limit\"", with: "\"scenario\":\"timeout\"")
    #expect(throws: Error.self) {
        try LocalProductSetupWire.decodeFailureLabResult(Data(mismatchedScenario.utf8))
    }

    let mismatchedRetryability = String(data: payload, encoding: .utf8)!
        .replacingOccurrences(of: "\"retryable\":true", with: "\"retryable\":false")
    #expect(throws: Error.self) {
        try LocalProductSetupWire.decodeFailureLabResult(Data(mismatchedRetryability.utf8))
    }
}

@Test
func failureLabCorruptRecordRequiresVersionedAADStage() throws {
    let payload = Data(
        """
        {
          "schema_version":1,
          "scenario":"corrupt_vault_record",
          "incident_id":"incident-failure-lab",
          "agents":[
            {"agent_id":"target-agent","provider_account_id":"failurelab_tmp_0123456789abcdef0123456789abcdef","status":"failed","stage":"vault_aad_validation","code":"corrupt_vault_record","retryable":false,"elapsed_milliseconds":1},
            {"agent_id":"healthy-peer","provider_account_id":"failurelab_tmp_fedcba9876543210fedcba9876543210","status":"succeeded","retryable":false,"elapsed_milliseconds":0}
          ]
        }
        """.utf8
    )
    let result = try LocalProductSetupWire.decodeFailureLabResult(payload)
    #expect(result.agents[0].stage == "vault_aad_validation")

    let legacyStage = String(data: payload, encoding: .utf8)!.replacingOccurrences(
        of: "vault_aad_validation", with: "vault_read"
    )
    #expect(throws: Error.self) {
        try LocalProductSetupWire.decodeFailureLabResult(Data(legacyStage.utf8))
    }
}

@Test
func providerAccountPolicyResultIsStrictAndAccountScoped() throws {
    let digest = String(repeating: "b", count: 64)
    let payload = Data(
        """
        {
          "policy_available": true,
          "policy_version": 2,
          "provider_id": "deepseek",
          "provider_account_id": "deepseek.work",
          "revision": 3,
          "policy_digest": "\(digest)",
          "maximum_concurrent_attempts": 4,
          "dispatch_window_seconds": 60,
          "maximum_dispatch_starts": 20,
          "maximum_assigned_budget_units": 12000,
          "trust_domain": "external_provider",
          "retention_mode": "zero_data_retention",
          "data_region": "apac",
          "configured_at": "2026-08-11T05:00:00.123456789Z"
        }
        """.utf8
    )

    let result = try JSONDecoder().decode(
        LocalProductProviderAccountPolicyResult.self,
        from: payload
    )
    #expect(result.providerAccountID == "deepseek.work")
    #expect(result.revision == 3)
    #expect(result.policyDigest == digest)
    #expect(result.maximumConcurrentAttempts == 4)
    #expect(result.policyVersion == 2)
    #expect(result.dataRegion == "apac")

    for invalid in [
        String(data: payload, encoding: .utf8)!.replacingOccurrences(
            of: "\"deepseek.work\"",
            with: "\"openai.work\""
        ),
        String(data: payload, encoding: .utf8)!.replacingOccurrences(
            of: "\"maximum_concurrent_attempts\": 4",
            with: "\"maximum_concurrent_attempts\": 0"
        ),
        String(data: payload, encoding: .utf8)!.replacingOccurrences(
            of: "\"configured_at\": \"2026-08-11T05:00:00.123456789Z\"",
            with: "\"configured_at\": \"2026-08-11T05:00:00.123456789Z\", \"secret\": \"forbidden\""
        ),
    ] {
        #expect(throws: Error.self) {
            try JSONDecoder().decode(
                LocalProductProviderAccountPolicyResult.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func remoteToolBackendEnrollmentIsStrictAccountPolicyBoundAndNonSecret() throws {
    let policyDigest = String(repeating: "b", count: 64)
    let endpointFingerprint = String(repeating: "c", count: 64)
    let enrollmentDigest = String(repeating: "d", count: 64)
    let payload = """
        {
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "auth_mode":"api_key",
          "credential_reference":"credential-ref-deepseek-work",
          "revision":3,
          "status":"verified",
          "reason":"none",
          "policy_available":true,
          "policy_version":2,
          "policy_revision":3,
          "policy_digest":"\(policyDigest)",
          "maximum_concurrent_attempts":4,
          "dispatch_window_seconds":60,
          "maximum_dispatch_starts":20,
          "maximum_assigned_budget_units":12000,
          "trust_domain":"external_provider",
          "retention_mode":"zero_data_retention",
          "data_region":"apac",
          "rate_cards":[],
          "remote_tool_backends":[{
            "enrollment_version":1,
            "enrollment_id":"mcp-deepseek-work",
            "backend_kind":"mcp_server",
            "adapter_id":"builtin.mcp.stdio.v1",
            "provider_account_policy_version":2,
            "provider_account_policy_revision":3,
            "provider_account_policy_digest":"\(policyDigest)",
            "policy_current":true,
            "endpoint_fingerprint":"\(endpointFingerprint)",
            "mcp_server_id":"work-tools",
            "allowed_tools":["get_issue","search_docs"],
            "revision":1,
            "status":"active",
            "maximum_concurrent_calls":2,
            "maximum_calls_per_attempt":4,
            "timeout_seconds":30,
            "maximum_result_bytes":32768,
            "maximum_budget_units":2000,
            "configured_at":"2026-08-15T05:00:00.123456789Z",
            "enrollment_digest":"\(enrollmentDigest)"
          }]
        }
        """
    let account = try JSONDecoder().decode(
        LocalProductProviderAccountDirectoryEntry.self,
        from: Data(payload.utf8)
    )
    #expect(account.remoteToolBackends.count == 1)
    #expect(account.remoteToolBackends[0].policyCurrent)
    #expect(account.remoteToolBackends[0].allowedTools == ["get_issue", "search_docs"])

    let stale = payload
        .replacingOccurrences(
            of: "\"provider_account_policy_revision\":3",
            with: "\"provider_account_policy_revision\":2"
        )
        .replacingOccurrences(
            of: "\"policy_current\":true",
            with: "\"policy_current\":false"
        )
    let staleAccount = try JSONDecoder().decode(
        LocalProductProviderAccountDirectoryEntry.self,
        from: Data(stale.utf8)
    )
    #expect(staleAccount.remoteToolBackends[0].policyCurrent == false)

    for invalid in [
        payload.replacingOccurrences(
            of: "\"provider_account_policy_revision\":3",
            with: "\"provider_account_policy_revision\":2"
        ),
        payload.replacingOccurrences(
            of: "[\"get_issue\",\"search_docs\"]",
            with: "[\"search_docs\",\"get_issue\"]"
        ),
        payload.replacingOccurrences(
            of: "\"endpoint_fingerprint\":\"\(endpointFingerprint)\"",
            with: "\"endpoint_fingerprint\":\"https://secret.example\""
        ),
        payload.replacingOccurrences(
            of: "\"enrollment_digest\":\"\(enrollmentDigest)\"",
            with: "\"enrollment_digest\":\"\(enrollmentDigest)\",\"secret\":\"forbidden\""
        ),
    ] {
        #expect(throws: Error.self) {
            try JSONDecoder().decode(
                LocalProductProviderAccountDirectoryEntry.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func remoteToolBackendEnrollmentResultIsStrictAndAccountScoped() throws {
    let policyDigest = String(repeating: "b", count: 64)
    let endpointFingerprint = String(repeating: "c", count: 64)
    let enrollmentDigest = String(repeating: "d", count: 64)
    let payload = """
        {
          "enrollment_available":true,
          "enrollment_version":1,
          "enrollment_id":"mcp-deepseek-work",
          "backend_kind":"mcp_server",
          "adapter_id":"builtin.mcp.stdio.v1",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "provider_account_policy_version":2,
          "provider_account_policy_revision":3,
          "provider_account_policy_digest":"\(policyDigest)",
          "policy_current":true,
          "endpoint_fingerprint":"\(endpointFingerprint)",
          "mcp_server_id":"work-tools",
          "allowed_tools":["get_issue","search_docs"],
          "revision":2,
          "status":"active",
          "maximum_concurrent_calls":2,
          "maximum_calls_per_attempt":4,
          "timeout_seconds":30,
          "maximum_result_bytes":32768,
          "maximum_budget_units":2000,
          "configured_at":"2026-08-15T05:00:00.123456789Z",
          "enrollment_digest":"\(enrollmentDigest)"
        }
        """
    let result = try JSONDecoder().decode(
        LocalProductRemoteToolBackendEnrollmentResult.self,
        from: Data(payload.utf8)
    )
    #expect(result.providerID == "deepseek")
    #expect(result.providerAccountID == "deepseek.work")
    #expect(result.revision == 2)
    #expect(result.allowedTools == ["get_issue", "search_docs"])

    for invalid in [
        payload.replacingOccurrences(
            of: "\"provider_account_id\":\"deepseek.work\"",
            with: "\"provider_account_id\":\"openai.work\""
        ),
        payload.replacingOccurrences(
            of: "\"endpoint_fingerprint\":\"\(endpointFingerprint)\"",
            with: "\"endpoint\":\"https://secret.example\""
        ),
        payload.replacingOccurrences(
            of: "\"enrollment_digest\":\"\(enrollmentDigest)\"",
            with: "\"enrollment_digest\":\"\(enrollmentDigest)\",\"secret\":\"forbidden\""
        ),
        payload.replacingOccurrences(
            of: "\"revision\":2",
            with: "\"revision\":-9223372036854775808"
        ),
    ] {
        #expect(throws: Error.self) {
            try JSONDecoder().decode(
                LocalProductRemoteToolBackendEnrollmentResult.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func providerModelRateCardIsStrictVersionedAndNonSecret() throws {
    let digest = String(repeating: "c", count: 64)
    let payload = """
        {
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "model_id":"deepseek-chat",
          "revision":3,
          "rate_card_digest":"\(digest)",
          "currency":"USD",
          "input_token_basis":"input_includes_cache",
          "input_microunits_per_million":270000,
          "output_microunits_per_million":1100000,
          "cache_read_microunits_per_million":70000,
          "cache_write_microunits_per_million":0,
          "rounding_mode":"ceiling_per_attempt",
          "configured_at":"2026-08-12T05:00:00.123456789Z"
        }
        """
    let result = try JSONDecoder().decode(
        LocalProductProviderModelRateCardResult.self,
        from: Data(payload.utf8)
    )
    #expect(result.providerAccountID == "deepseek.work")
    #expect(result.modelID == "deepseek-chat")
    #expect(result.revision == 3)
    #expect(result.currency == "USD")

    for invalid in [
        payload.replacingOccurrences(of: "deepseek.work", with: "openai.work"),
        payload.replacingOccurrences(
            of: "\"input_microunits_per_million\":270000",
            with: "\"input_microunits_per_million\":-1"
        ),
        payload.replacingOccurrences(
            of: "\"currency\":\"USD\"",
            with: "\"currency\":\"usd\""
        ),
        payload.replacingOccurrences(
            of: "\"configured_at\":\"2026-08-12T05:00:00.123456789Z\"",
            with: "\"configured_at\":\"2026-08-12T05:00:00.123456789Z\",\"secret\":\"forbidden\""
        ),
    ] {
        #expect(throws: Error.self) {
            try JSONDecoder().decode(
                LocalProductProviderModelRateCardResult.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func credentialVaultStatusRejectsUnknownStateAndNegativeCounts() {
    let invalidStatus = Data(
        """
        {
          "schema_version": 1,
          "status": "available",
          "storage_mode": "local_key_file",
          "migration_required_accounts": 0,
          "recovery_required_accounts": 0
        }
        """.utf8
    )
    #expect(throws: LocalProductWireError.invalidJSON) {
        _ = try JSONDecoder().decode(
            LocalProductCredentialVaultStatus.self,
            from: invalidStatus
        )
    }

    let negativeCount = Data(
        """
        {
          "schema_version": 1,
          "status": "migration_required",
          "storage_mode": "local_key_file",
          "migration_required_accounts": -1,
          "recovery_required_accounts": 0
        }
        """.utf8
    )
    #expect(throws: LocalProductWireError.invalidJSON) {
        _ = try JSONDecoder().decode(
            LocalProductCredentialVaultStatus.self,
            from: negativeCount
        )
    }
}

@Test
func credentialVaultExportResultIsStrictAndBounded() throws {
    let digest = String(repeating: "a", count: 64)
    let result = try JSONDecoder().decode(
        LocalProductCredentialVaultExportResult.self,
        from: Data(
            """
            {"schema_version":1,"file_path":"/tmp/Loom.loomvault","digest":"\(digest)","credential_count":4}
            """.utf8
        )
    )
    #expect(result.credentialCount == 4)
    for invalid in [
        "{\"schema_version\":1,\"file_path\":\"relative.loomvault\",\"digest\":\"\(digest)\",\"credential_count\":4}",
        "{\"schema_version\":1,\"file_path\":\"/tmp/Loom.loomvault\",\"digest\":\"\(digest)\",\"credential_count\":257}",
        "{\"schema_version\":1,\"file_path\":\"/tmp/Loom.loomvault\",\"digest\":\"\(digest)\",\"credential_count\":4,\"secret\":\"forbidden\"}",
    ] {
        #expect(throws: Error.self) {
            try JSONDecoder().decode(
                LocalProductCredentialVaultExportResult.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func providerConnectResultDecodesExactClosedShape() throws {
    let payload = Data(
        """
        {
          "provider_id": "codex",
          "auth_mode": "native_auth",
          "status": "started"
        }
        """.utf8
    )
    let result = try LocalProductSetupWire.decodeProviderConnectResult(payload)
    #expect(result.providerID == "codex")
    #expect(result.authMode == "native_auth")
    #expect(result.status == "started")

    let claude = try LocalProductSetupWire.decodeProviderConnectResult(Data(
        """
        {
          "provider_id": "claude-code",
          "auth_mode": "native_auth",
          "status": "started"
        }
        """.utf8
    ))
    #expect(claude.providerID == "claude-code")

    let untrustedProvider = Data(
        """
        {
          "provider_id": "arbitrary-runtime",
          "auth_mode": "native_auth",
          "status": "started"
        }
        """.utf8
    )
    #expect(throws: LocalProductWireError.invalidJSON) {
        _ = try LocalProductSetupWire.decodeProviderConnectResult(untrustedProvider)
    }

    let unknown = Data(
        """
        {
          "provider_id": "codex",
          "auth_mode": "native_auth",
          "status": "started",
          "token": "must-not-be-accepted"
        }
        """.utf8
    )
    #expect(throws: LocalProductWireError.unknownField) {
        _ = try LocalProductSetupWire.decodeProviderConnectResult(unknown)
    }
}

@Test
func builderSessionDecodesOneQuestionAndExactPreview() throws {
    let payload = Data(
        """
        {
          "schema_version": 1,
          "draft_id": "draft-1",
          "revision": 4,
          "source": "blank",
          "catalog_digest": "\(String(repeating: "b", count: 64))",
          "view_version": "\(String(repeating: "e", count: 64))",
          "content_digest": "\(String(repeating: "c", count: 64))",
          "binding_digest": "\(String(repeating: "d", count: 64))",
          "question": {
            "id": "",
            "prompt": "",
            "options": []
          },
          "preview": {
            "name": "Review Team",
            "purpose": "Review one bounded change",
            "roles": [],
            "permissions": [],
            "resources": [],
            "compatibility_gaps": [],
            "requested_concurrency": 1,
            "maximum_budget_credits": 100,
            "estimated_maximum_cost": "up to 100 credits"
          },
          "can_confirm": true
        }
        """.utf8
    )

    let session = try LocalProductSetupWire.decodeBuilderSession(payload)
    #expect(session.draftID == "draft-1")
    #expect(session.revision == 4)
    #expect(session.question.options.isEmpty)
    #expect(session.preview.roles.isEmpty)
    #expect(session.preview.permissions.isEmpty)
    #expect(session.preview.resources.isEmpty)
    #expect(session.preview.compatibilityGaps.isEmpty)
    #expect(session.canConfirm)
}

@Test
func builderParallelExecutionRouteIsStrictAndAccountScoped() throws {
    let payload = """
        {
          "runtime_profile_id":"profile-deepseek-parallel",
          "harness_adapter":"loom-native",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "model_id":"deepseek-chat",
          "auth_mode":"brokered",
          "credential_revision":7,
          "reasoning_effort":"",
          "timeout_milliseconds":60000,
          "budget_available":true,
          "budget_units":120,
          "required_capabilities":["chat"]
        }
        """
    let route = try JSONDecoder().decode(
        LocalProductBuilderExecutionRoute.self,
        from: Data(payload.utf8)
    )
    #expect(route.providerAccountID == "deepseek.work")
    #expect(route.credentialRevision == 7)
    #expect(route.modelID == "deepseek-chat")

    for invalid in [
        payload.replacingOccurrences(
            of: "\"deepseek.work\"",
            with: "\"openai.work\""
        ),
        payload.replacingOccurrences(
            of: "\"credential_revision\":7",
            with: "\"credential_revision\":0"
        ),
        payload.replacingOccurrences(
            of: "\"required_capabilities\":[\"chat\"]",
            with: "\"required_capabilities\":[\"chat\"],\"secret\":\"forbidden\""
        ),
    ] {
        #expect(throws: Error.self) {
            try JSONDecoder().decode(
                LocalProductBuilderExecutionRoute.self,
                from: Data(invalid.utf8)
            )
        }
    }
}

@Test
func setupWireRejectsUnknownFieldsAndNullCollections() {
    let unknown = Data(
        """
        {
          "schema_version": 1,
          "draft_id": "draft-1",
          "revision": 1,
          "source": "blank",
          "catalog_digest": "",
          "view_version": "",
          "content_digest": "",
          "binding_digest": "",
          "question": {"id":"team_name","prompt":"Name","options":[]},
          "preview": {
            "name":"",
            "purpose":"",
            "roles":[],
            "permissions":[],
            "resources":[],
            "compatibility_gaps":[],
            "requested_concurrency":0,
            "maximum_budget_credits":0,
            "estimated_maximum_cost":""
          },
          "can_confirm": false,
          "unexpected": true
        }
        """.utf8
    )
    #expect(throws: LocalProductWireError.unknownField) {
        _ = try LocalProductSetupWire.decodeBuilderSession(unknown)
    }

    let nullCollection = Data(
        """
        {
          "schema_version": 1,
          "view_version": "\(String(repeating: "a", count: 64))",
          "codex": {"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
          "minimax": {"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
          "runtimes": null,
          "saved_teams": [],
          "templates": [],
          "role_options": [],
          "skills": [],
          "permissions": [],
          "resources": []
        }
        """.utf8
    )
    #expect(throws: LocalProductWireError.invalidJSON) {
        _ = try LocalProductSetupWire.decodeSnapshot(nullCollection)
    }
}

@Test
func setupRoleOptionDecodesEnrollmentPairAndRejectsOneSided() throws {
    let valid = """
        {
          "id":"role-main",
          "kind":"main",
          "agent_definition_id":"agent-main",
          "runtime_profile_id":"profile-main",
          "runtime_instance_id":"runtime-pi",
          "harness_adapter":"codex",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "model_id":"deepseek-chat",
          "auth_mode":"brokered",
          "credential_revision":7,
          "reasoning_effort":"",
          "timeout_milliseconds":60000,
          "budget_available":true,
          "budget_units":120,
          "required_capabilities":["chat"],
          "remote_tool_enrollment_id":"enr-search",
          "remote_tool_enrollment_digest":"\(String(repeating: "a", count: 64))",
          "skill_revision_ids":[],
          "permission_ids":[],
          "resource_ids":[],
          "responsibility":"Coordinate"
        }
        """
    let option = try JSONDecoder().decode(
        LocalProductSetupRoleOption.self,
        from: Data(valid.utf8)
    )
    #expect(option.remoteToolEnrollmentID == "enr-search")
    #expect(option.remoteToolEnrollmentDigest == String(repeating: "a", count: 64))

    let oneSided = valid.replacingOccurrences(
        of: "\"remote_tool_enrollment_digest\":\"\(String(repeating: "a", count: 64))\",",
        with: ""
    )
    #expect(throws: LocalProductWireError.invalidJSON) {
        try JSONDecoder().decode(
            LocalProductSetupRoleOption.self,
            from: Data(oneSided.utf8)
        )
    }

    let unknown = valid.replacingOccurrences(
        of: "\"responsibility\":\"Coordinate\"",
        with: "\"responsibility\":\"Coordinate\",\"raw_endpoint\":\"https://x\""
    )
    #expect(throws: LocalProductWireError.unknownField) {
        try JSONDecoder().decode(
            LocalProductSetupRoleOption.self,
            from: Data(unknown.utf8)
        )
    }
}

@Test
func builderExecutionRouteDecodesEnrollmentPairAndRejectsOneSided() throws {
    let digest = String(repeating: "c", count: 64)
    let valid = """
        {
          "runtime_profile_id":"profile-deepseek-parallel",
          "harness_adapter":"loom-native",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "model_id":"deepseek-chat",
          "auth_mode":"brokered",
          "credential_revision":7,
          "reasoning_effort":"",
          "timeout_milliseconds":60000,
          "budget_available":true,
          "budget_units":120,
          "required_capabilities":["chat"],
          "remote_tool_enrollment_id":"enr-mcp",
          "remote_tool_enrollment_digest":"\(digest)"
        }
        """
    let route = try JSONDecoder().decode(
        LocalProductBuilderExecutionRoute.self,
        from: Data(valid.utf8)
    )
    #expect(route.remoteToolEnrollmentID == "enr-mcp")
    #expect(route.remoteToolEnrollmentDigest == digest)

    let oneSided = """
        {
          "runtime_profile_id":"profile-deepseek-parallel",
          "harness_adapter":"loom-native",
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "model_id":"deepseek-chat",
          "auth_mode":"brokered",
          "credential_revision":7,
          "reasoning_effort":"",
          "timeout_milliseconds":60000,
          "budget_available":true,
          "budget_units":120,
          "required_capabilities":["chat"],
          "remote_tool_enrollment_id":"enr-mcp"
        }
        """
    #expect(throws: LocalProductWireError.invalidJSON) {
        try JSONDecoder().decode(
            LocalProductBuilderExecutionRoute.self,
            from: Data(oneSided.utf8)
        )
    }
}

@Test
func builderStoreAllowsEnrollmentEditFields() {
    #expect(LocalProductStore.allowsBuilderEditField("main_remote_tool_enrollment"))
    #expect(LocalProductStore.allowsBuilderEditField("subagent_remote_tool_enrollment"))
    #expect(LocalProductStore.validBuilderEditTarget(
        field: "main_remote_tool_enrollment",
        roleAgentDefinitionID: ""
    ))
    #expect(!LocalProductStore.validBuilderEditTarget(
        field: "subagent_remote_tool_enrollment",
        roleAgentDefinitionID: ""
    ))
}
