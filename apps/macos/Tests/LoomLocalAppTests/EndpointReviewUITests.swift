import Foundation
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

final class EndpointReviewUITests: XCTestCase {
    func testEndpointReviewPresentationExposesGovernedNonSecretMetadata() throws {
        let candidate = try endpointReviewCandidate()

        let presentation = endpointReviewPresentation(candidate)

        XCTAssertEqual(presentation.source, "CC Switch")
        XCTAssertEqual(presentation.provider, "Custom Gateway (custom-gateway)")
        XCTAssertEqual(presentation.protocolName, "openai_responses")
        XCTAssertEqual(presentation.origin, "https://gateway.example.com")
        XCTAssertEqual(presentation.endpoint, candidate.endpoint)
        XCTAssertEqual(
            presentation.models,
            [String(repeating: "m", count: 256), "reasoning/model-v2"]
        )
        XCTAssertTrue(presentation.credentialEgressRisk.contains(presentation.origin))
        XCTAssertTrue(presentation.credentialEgressRisk.contains("does not import"))
        XCTAssertTrue(presentation.credentialEgressRisk.contains("separate import step"))
        XCTAssertEqual(
            presentation.reviewButtonAccessibilityLabel,
            "Review custom endpoint for Custom Gateway"
        )
        XCTAssertTrue(presentation.sheetAccessibilityLabel.contains("CC Switch"))

        let visibleText = [
            presentation.source,
            presentation.provider,
            presentation.protocolName,
            presentation.origin,
            presentation.endpoint,
            presentation.models.joined(separator: " "),
            presentation.credentialEgressRisk,
            presentation.sheetAccessibilityLabel,
        ].joined(separator: " ").lowercased()
        XCTAssertFalse(visibleText.contains("sk-review-secret"))
        XCTAssertFalse(visibleText.contains("api key"))
    }

    func testEndpointReviewCandidateFreezesEveryReviewBinding() throws {
        let candidate = try endpointReviewCandidate()

        XCTAssertEqual(candidate.candidateID, String(repeating: "a", count: 64))
        XCTAssertEqual(candidate.candidateDigest, String(repeating: "b", count: 64))
        XCTAssertEqual(candidate.endpointFingerprint, String(repeating: "c", count: 64))
        XCTAssertEqual(candidate.reviewPolicyVersion, 3)
        XCTAssertEqual(candidate.reviewPolicyDigest, String(repeating: "d", count: 64))
        XCTAssertEqual(candidate.modelIDs.count, 2)
    }

    private func endpointReviewCandidate() throws
        -> LocalProductCredentialImportCandidate
    {
        let longModel = String(repeating: "m", count: 256)
        return try JSONDecoder().decode(
            LocalProductCredentialImportCandidate.self,
            from: Data(
                """
                {
                  "candidate_id": "\(String(repeating: "a", count: 64))",
                  "candidate_digest": "\(String(repeating: "b", count: 64))",
                  "source_application": "CC Switch",
                  "display_name": "Custom Gateway",
                  "target_provider_id": "custom-gateway",
                  "protocol": "openai_responses",
                  "endpoint": "https://gateway.example.com/\(String(repeating: "long-path/", count: 100))v1/responses",
                  "endpoint_fingerprint": "\(String(repeating: "c", count: 64))",
                  "model_ids": ["\(longModel)", "reasoning/model-v2"],
                  "import_mode": "custom_endpoint_review",
                  "current": true,
                  "credential_available": true,
                  "review_policy_version": 3,
                  "review_policy_digest": "\(String(repeating: "d", count: 64))"
                }
                """.utf8
            )
        )
    }
}
