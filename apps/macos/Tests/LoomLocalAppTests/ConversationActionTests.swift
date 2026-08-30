import XCTest
@testable import LoomLocalAppCore

final class ConversationActionTests: XCTestCase {
    func testCatalogExposesOneStableCommandForEveryTopLevelCapability() {
        let commands = ConversationActionCatalog.commands

        XCTAssertEqual(commands.count, 18)
        XCTAssertEqual(Set(commands.map(\.id)).count, commands.count)
        XCTAssertEqual(Set(commands.map(\.command)).count, commands.count)
        XCTAssertEqual(
            Set(commands.map(\.id)),
            Set(ConversationActionID.allCases)
        )
        XCTAssertEqual(commands.first?.id, .help)
        XCTAssertTrue(commands.allSatisfy { !$0.title.isEmpty && !$0.detail.isEmpty })
        XCTAssertFalse(commands.contains { $0.command == "credential" })
    }

    func testSlashSuggestionsAreSearchableStableAndAliasAware() {
        XCTAssertEqual(
            ConversationActionCatalog.suggestions(for: "/").map(\.id).prefix(3),
            [.help, .newTask, .mission]
        )
        XCTAssertEqual(
            ConversationActionCatalog.suggestions(for: "/mis").map(\.id),
            [.mission, .missions]
        )
        XCTAssertEqual(
            ConversationActionCatalog.suggestions(for: "/rt").map(\.id),
            [.roundTable]
        )
        XCTAssertEqual(
            ConversationActionCatalog.suggestions(for: "/provider").map(\.id),
            [.runtimeProviders]
        )
        XCTAssertTrue(ConversationActionCatalog.suggestions(for: "hello").isEmpty)
    }

    func testSlashRouterPreservesBoundedArgumentsAndRejectsUnknownCommands() {
        XCTAssertEqual(
            ConversationActionRouter.resolve("/mission Audit the command flow"),
            .request(ConversationActionRequest(
                action: .mission,
                argument: "Audit the command flow",
                source: .slashCommand
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/rt"),
            .request(ConversationActionRequest(
                action: .roundTable,
                source: .slashCommand
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/does-not-exist private text"),
            .unknownSlashCommand("does-not-exist")
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/mission \(String(repeating: "a", count: 5_000))"),
            .request(ConversationActionRequest(
                action: .mission,
                argument: String(repeating: "a", count: 4_096),
                source: .slashCommand
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/providers never-retain-this"),
            .request(ConversationActionRequest(
                action: .runtimeProviders,
                source: .slashCommand
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/mission Ship the installed app!"),
            .request(ConversationActionRequest(
                action: .mission,
                argument: "Ship the installed app!",
                source: .slashCommand
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/mission sk-proj-sensitive-value"),
            .rejectedActionArgument(.mission, .credentialLike)
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve(
                "/mission Audit with api_key=synthetic-sensitive-value"
            ),
            .rejectedActionArgument(.mission, .credentialLike)
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/providers sk-synthetic-sensitive-value"),
            .rejectedActionArgument(.runtimeProviders, .credentialLike)
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("/mission Run a risk-based review"),
            .request(ConversationActionRequest(
                action: .mission,
                argument: "Run a risk-based review",
                source: .slashCommand
            ))
        )
    }

    func testNaturalLanguageRoutesOnlyExplicitImperativeIntents() {
        XCTAssertEqual(
            ConversationActionRouter.resolve("创建 Mission：审查统一入口"),
            .request(ConversationActionRequest(
                action: .mission,
                argument: "审查统一入口",
                source: .naturalLanguage
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("组建 Agent 团队：负责前端验收"),
            .request(ConversationActionRequest(
                action: .team,
                argument: "负责前端验收",
                source: .naturalLanguage
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("打开 RoundTable"),
            .request(ConversationActionRequest(
                action: .roundTable,
                source: .naturalLanguage
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("切换模型"),
            .request(ConversationActionRequest(
                action: .model,
                source: .naturalLanguage
            ))
        )
        XCTAssertEqual(
            ConversationActionRouter.resolve("继续 Mission：只使用 MiniMax"),
            .request(ConversationActionRequest(
                action: .continueMission,
                argument: "只使用 MiniMax",
                source: .naturalLanguage
            ))
        )

        XCTAssertEqual(ConversationActionRouter.resolve("How do I create a Mission?"), .chat)
        XCTAssertEqual(ConversationActionRouter.resolve("介绍一下 Agent Team 是什么"), .chat)
        XCTAssertEqual(ConversationActionRouter.resolve("mission critical bug"), .chat)
        XCTAssertEqual(ConversationActionRouter.resolve("普通对话"), .chat)
    }

    func testContextAvailabilityExplainsUnavailableActionsWithoutHidingThem() {
        let empty = ConversationActionContext()

        XCTAssertEqual(
            ConversationActionCatalog.availability(of: .stop, in: empty),
            .unavailable("No response is running")
        )
        XCTAssertEqual(
            ConversationActionCatalog.availability(of: .continueMission, in: empty),
            .unavailable("Open a blocked Mission first")
        )
        XCTAssertEqual(
            ConversationActionCatalog.availability(of: .route, in: empty),
            .unavailable("No executable Route is configured")
        )
        XCTAssertEqual(
            ConversationActionCatalog.availability(of: .model, in: empty),
            .unavailable("The selected Route has no models")
        )
        XCTAssertEqual(
            ConversationActionCatalog.availability(of: .copyConversation, in: empty),
            .unavailable("This conversation has no messages")
        )
        XCTAssertEqual(
            ConversationActionCatalog.availability(
                of: .stop,
                in: ConversationActionContext(isResponding: true)
            ),
            .available
        )
        XCTAssertEqual(
            ConversationActionCatalog.availability(
                of: .continueMission,
                in: ConversationActionContext(missionNeedsIntervention: true)
            ),
            .available
        )
    }

    func testMenuSelectionTracksFilteringAndKeyboardMovement() {
        var state = ConversationActionMenuState()

        state.synchronize(with: "/")
        XCTAssertEqual(state.selectedID, .help)

        state.move(.next, for: "/")
        XCTAssertEqual(state.selectedID, .newTask)
        state.move(.previous, for: "/")
        XCTAssertEqual(state.selectedID, .help)
        state.move(.previous, for: "/")
        XCTAssertEqual(state.selectedID, .copyConversation)

        state.synchronize(with: "/mis")
        XCTAssertEqual(state.selectedID, .mission)
        state.move(.next, for: "/mis")
        XCTAssertEqual(state.selectedID, .missions)

        state.synchronize(with: "/rt")
        XCTAssertEqual(state.selectedID, .roundTable)
    }

    func testMenuSelectionBuildsBoundedTypedRequestFromPartialCommand() {
        var state = ConversationActionMenuState()
        state.synchronize(with: "/mis Audit the installed app")

        XCTAssertEqual(
            state.request(for: "/mis Audit the installed app"),
            ConversationActionRequest(
                action: .mission,
                argument: "Audit the installed app",
                source: .commandMenu
            )
        )
        XCTAssertNil(state.request(for: "ordinary chat"))
        XCTAssertEqual(
            state.resolution(for: "/mis sk-ant-sensitive-value"),
            .rejectedActionArgument(.mission, .credentialLike)
        )
    }
}
