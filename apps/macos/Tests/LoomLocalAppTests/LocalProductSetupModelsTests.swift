import Foundation
import Testing
@testable import LoomLocalAppCore

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
    #expect(snapshot.runtimes.first?.executableVersion == "0.82.1")
    #expect(snapshot.runtimes.first?.modelIDs == ["model-a"])
    #expect(snapshot.savedTeams.first?.streamHead == 3)
    #expect(snapshot.templates.isEmpty)
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
