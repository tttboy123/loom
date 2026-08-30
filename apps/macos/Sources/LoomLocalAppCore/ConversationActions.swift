import Foundation

public enum ConversationActionID: String, CaseIterable, Hashable, Sendable {
    case help
    case newTask
    case mission
    case missions
    case team
    case roundTable
    case continueMission
    case route
    case model
    case reasoning
    case status
    case diagnostics
    case stop
    case needsYou
    case library
    case runtimeProviders
    case folder
    case copyConversation
}

public enum ConversationActionGroup: String, CaseIterable, Hashable, Sendable {
    case conversation = "Conversation"
    case work = "Work"
    case governance = "Governance"
    case setup = "Setup"
}

public enum ConversationActionKind: Hashable, Sendable {
    case local
    case navigation
    case governedProposal
    case boundedControl
}

public struct ConversationActionDefinition: Identifiable, Hashable, Sendable {
    public let id: ConversationActionID
    public let command: String
    public let aliases: [String]
    public let title: String
    public let detail: String
    public let group: ConversationActionGroup
    public let systemImage: String
    public let argumentHint: String?
    public let keywords: [String]
    public let kind: ConversationActionKind

    public init(
        id: ConversationActionID,
        command: String,
        aliases: [String] = [],
        title: String,
        detail: String,
        group: ConversationActionGroup,
        systemImage: String,
        argumentHint: String? = nil,
        keywords: [String] = [],
        kind: ConversationActionKind
    ) {
        self.id = id
        self.command = command
        self.aliases = aliases
        self.title = title
        self.detail = detail
        self.group = group
        self.systemImage = systemImage
        self.argumentHint = argumentHint
        self.keywords = keywords
        self.kind = kind
    }
}

public enum ConversationActionSource: String, Hashable, Sendable {
    case slashCommand
    case naturalLanguage
    case commandMenu
    case modelTool
}

public struct ConversationActionRequest: Hashable, Sendable {
    public let action: ConversationActionID
    public let argument: String
    public let source: ConversationActionSource

    public init(
        action: ConversationActionID,
        argument: String = "",
        source: ConversationActionSource
    ) {
        self.action = action
        self.argument = argument
        self.source = source
    }
}

public enum ConversationActionArgumentRejection: String, Hashable, Sendable {
    case credentialLike
}

public enum ConversationActionResolution: Hashable, Sendable {
    case chat
    case request(ConversationActionRequest)
    case unknownSlashCommand(String)
    case rejectedActionArgument(
        ConversationActionID,
        ConversationActionArgumentRejection
    )
}

public struct ConversationActionContext: Hashable, Sendable {
    public let isResponding: Bool
    public let missionNeedsIntervention: Bool
    public let routeCount: Int
    public let modelCount: Int
    public let hasMessages: Bool

    public init(
        isResponding: Bool = false,
        missionNeedsIntervention: Bool = false,
        routeCount: Int = 0,
        modelCount: Int = 0,
        hasMessages: Bool = false
    ) {
        self.isResponding = isResponding
        self.missionNeedsIntervention = missionNeedsIntervention
        self.routeCount = max(0, routeCount)
        self.modelCount = max(0, modelCount)
        self.hasMessages = hasMessages
    }
}

public enum ConversationActionAvailability: Hashable, Sendable {
    case available
    case unavailable(String)
}

public enum ConversationActionCatalog {
    public static let commands: [ConversationActionDefinition] = [
        command(
            .help, "help", aliases: ["commands", "?"], title: "Commands",
            detail: "Browse everything Loom can do here", group: .conversation,
            icon: "command", keywords: ["slash", "actions"], kind: .local
        ),
        command(
            .newTask, "new", aliases: ["new-task", "conversation"], title: "New task",
            detail: "Start a clean conversation", group: .conversation,
            icon: "square.and.pencil", kind: .local
        ),
        command(
            .mission, "mission", aliases: ["run"], title: "Create Mission",
            detail: "Review a single governed workflow before it runs", group: .work,
            icon: "scope", argumentHint: "objective", keywords: ["workflow"],
            kind: .governedProposal
        ),
        command(
            .missions, "missions", aliases: ["mission-list"], title: "Missions",
            detail: "Open current and past workflows", group: .work,
            icon: "list.bullet.rectangle", kind: .navigation
        ),
        command(
            .team, "team", aliases: ["agent-team"], title: "Agent Team",
            detail: "Draft a governed team and review every role", group: .work,
            icon: "person.3", argumentHint: "purpose", keywords: ["agents"],
            kind: .governedProposal
        ),
        command(
            .roundTable, "roundtable", aliases: ["rt"], title: "RoundTable",
            detail: "Open a linked, multi-Agent deliberation", group: .work,
            icon: "bubble.left.and.bubble.right", keywords: ["round table", "deliberation"],
            kind: .governedProposal
        ),
        command(
            .continueMission, "continue", aliases: ["resume"], title: "Continue Mission",
            detail: "Add guidance to a blocked Mission for review", group: .work,
            icon: "arrow.clockwise", argumentHint: "guidance", keywords: ["intervene"],
            kind: .governedProposal
        ),
        command(
            .route, "route", aliases: ["execution-route"], title: "Route",
            detail: "Choose the Harness and Provider route for new turns", group: .governance,
            icon: "point.3.connected.trianglepath.dotted", keywords: ["provider", "harness"],
            kind: .governedProposal
        ),
        command(
            .model, "model", title: "Model",
            detail: "Choose a model on the selected Route", group: .governance,
            icon: "cpu", kind: .governedProposal
        ),
        command(
            .reasoning, "reasoning", aliases: ["effort"], title: "Reasoning",
            detail: "Set reasoning effort for new turns", group: .governance,
            icon: "brain.head.profile", kind: .governedProposal
        ),
        command(
            .status, "status", title: "Status",
            detail: "See the active route, response and Mission state", group: .governance,
            icon: "waveform.path.ecg", kind: .local
        ),
        command(
            .diagnostics, "diagnostics", aliases: ["diag"], title: "Diagnostics",
            detail: "Preview privacy-safe service diagnostics", group: .governance,
            icon: "stethoscope", kind: .local
        ),
        command(
            .stop, "stop", aliases: ["cancel"], title: "Stop response",
            detail: "Review and stop the response in progress", group: .governance,
            icon: "stop.fill", kind: .boundedControl
        ),
        command(
            .needsYou, "needs-you", aliases: ["attention"], title: "Needs You",
            detail: "Open decisions and blocked work awaiting input", group: .governance,
            icon: "exclamationmark.bubble", kind: .navigation
        ),
        command(
            .library, "library", aliases: ["results"], title: "Library",
            detail: "Open accepted results and reusable artifacts", group: .governance,
            icon: "books.vertical", kind: .navigation
        ),
        command(
            .runtimeProviders, "providers", aliases: ["provider", "runtime", "runtimes"],
            title: "Runtime & Providers",
            detail: "Manage accounts and execution runtimes outside chat", group: .setup,
            icon: "server.rack", keywords: ["endpoint", "account"], kind: .navigation
        ),
        command(
            .folder, "folder", aliases: ["workspace"], title: "Choose folder",
            detail: "Select the workspace for this conversation", group: .setup,
            icon: "folder", kind: .governedProposal
        ),
        command(
            .copyConversation, "copy", aliases: ["transcript"], title: "Copy conversation",
            detail: "Copy the visible conversation to the clipboard", group: .conversation,
            icon: "doc.on.doc", kind: .local
        ),
    ]

    public static func definition(for action: ConversationActionID) -> ConversationActionDefinition {
        guard let definition = commands.first(where: { $0.id == action }) else {
            preconditionFailure("Conversation action catalog is incomplete")
        }
        return definition
    }

    public static func suggestions(for input: String) -> [ConversationActionDefinition] {
        let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmed.hasPrefix("/") else { return [] }

        let query = String(trimmed.dropFirst())
            .split(whereSeparator: { $0.isWhitespace })
            .first
            .map(String.init)?
            .lowercased() ?? ""
        guard !query.isEmpty else { return commands }

        let commandMatches = commands.filter { definition in
            ([definition.command] + definition.aliases)
                .contains(where: { $0.lowercased().hasPrefix(query) })
        }
        if !commandMatches.isEmpty {
            return commandMatches
        }

        return commands.filter { definition in
            ([definition.title, definition.detail] + definition.keywords)
                .contains(where: { $0.lowercased().contains(query) })
        }
    }

    public static func availability(
        of action: ConversationActionID,
        in context: ConversationActionContext
    ) -> ConversationActionAvailability {
        switch action {
        case .stop where !context.isResponding:
            return .unavailable("No response is running")
        case .continueMission where !context.missionNeedsIntervention:
            return .unavailable("Open a blocked Mission first")
        case .route where context.routeCount == 0:
            return .unavailable("No executable Route is configured")
        case .model where context.modelCount == 0:
            return .unavailable("The selected Route has no models")
        case .copyConversation where !context.hasMessages:
            return .unavailable("This conversation has no messages")
        default:
            return .available
        }
    }

    private static func command(
        _ id: ConversationActionID,
        _ command: String,
        aliases: [String] = [],
        title: String,
        detail: String,
        group: ConversationActionGroup,
        icon: String,
        argumentHint: String? = nil,
        keywords: [String] = [],
        kind: ConversationActionKind
    ) -> ConversationActionDefinition {
        ConversationActionDefinition(
            id: id,
            command: command,
            aliases: aliases,
            title: title,
            detail: detail,
            group: group,
            systemImage: icon,
            argumentHint: argumentHint,
            keywords: keywords,
            kind: kind
        )
    }
}

public enum ConversationActionRouter {
    private static let maximumArgumentLength = 4_096

    public static func resolve(_ input: String) -> ConversationActionResolution {
        let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return .chat }
        if trimmed.hasPrefix("/") {
            return resolveSlashCommand(trimmed)
        }
        return resolveNaturalLanguage(trimmed)
    }

    public static func commandMenuRequest(
        action: ConversationActionID,
        input: String
    ) -> ConversationActionRequest? {
        guard case .request(let request) = commandMenuResolution(
            action: action,
            input: input
        ) else { return nil }
        return request
    }

    public static func commandMenuResolution(
        action: ConversationActionID,
        input: String
    ) -> ConversationActionResolution {
        let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmed.hasPrefix("/") else { return .chat }
        let body = trimmed.dropFirst()
        let splitIndex = body.firstIndex(where: { $0.isWhitespace }) ?? body.endIndex
        let remainder = splitIndex == body.endIndex ? "" : String(body[splitIndex...])
        return admittedRequest(
            definition: ConversationActionCatalog.definition(for: action),
            argument: remainder,
            source: .commandMenu
        )
    }

    private static func resolveSlashCommand(_ input: String) -> ConversationActionResolution {
        let body = input.dropFirst()
        let splitIndex = body.firstIndex(where: { $0.isWhitespace }) ?? body.endIndex
        let token = String(body[..<splitIndex]).lowercased()
        guard !token.isEmpty else { return .unknownSlashCommand("") }

        guard let definition = ConversationActionCatalog.commands.first(where: {
            $0.command.lowercased() == token
                || $0.aliases.contains(where: { $0.lowercased() == token })
        }) else {
            return .unknownSlashCommand(token)
        }

        let remainder = splitIndex == body.endIndex ? "" : String(body[splitIndex...])
        return admittedRequest(
            definition: definition,
            argument: remainder,
            source: .slashCommand
        )
    }

    private static func resolveNaturalLanguage(_ input: String) -> ConversationActionResolution {
        let lowercased = input.lowercased()
        guard !looksLikeQuestion(lowercased) else { return .chat }

        let intents: [(ConversationActionID, [String])] = [
            (.continueMission, ["继续 mission", "恢复 mission", "continue mission", "resume mission"]),
            (.roundTable, ["打开 roundtable", "开始 roundtable", "发起圆桌讨论", "open roundtable", "start roundtable", "start a roundtable"]),
            (.team, ["组建 agent 团队", "创建 agent 团队", "创建 agent team", "create agent team", "create an agent team", "use agent team", "use an agent team"]),
            (.mission, ["创建 mission", "新建 mission", "启动 mission", "create mission", "create a mission", "start mission", "start a mission"]),
            (.runtimeProviders, ["打开 provider 设置", "打开 runtime 设置", "open providers", "open runtime providers"]),
            (.needsYou, ["打开 needs you", "查看待处理", "open needs you"]),
            (.diagnostics, ["查看诊断", "打开诊断", "show diagnostics", "open diagnostics"]),
            (.status, ["查看运行状态", "查看状态", "show status", "open status"]),
            (.reasoning, ["切换推理强度", "选择推理强度", "switch reasoning", "choose reasoning"]),
            (.model, ["切换模型", "选择模型", "switch model", "choose model"]),
            (.route, ["切换 route", "选择 route", "switch route", "choose route"]),
            (.missions, ["打开 missions", "查看 missions", "open missions"]),
            (.library, ["打开 library", "查看结果库", "open library"]),
            (.folder, ["选择文件夹", "切换工作区", "choose folder", "choose workspace"]),
            (.newTask, ["新建对话", "开始新对话", "new task", "start a new task"]),
            (.copyConversation, ["复制对话", "copy conversation"]),
            (.stop, ["停止回复", "停止响应", "stop response", "cancel response"]),
            (.help, ["显示命令", "打开命令", "show commands", "open commands"]),
        ]

        for (action, phrases) in intents {
            if let phrase = phrases.first(where: { matchesImperative(lowercased, phrase: $0) }) {
                let argumentStart = input.index(input.startIndex, offsetBy: phrase.count)
                let remainder = String(input[argumentStart...])
                return admittedRequest(
                    definition: ConversationActionCatalog.definition(for: action),
                    argument: remainder,
                    source: .naturalLanguage
                )
            }
        }

        return .chat
    }

    private static func matchesImperative(_ input: String, phrase: String) -> Bool {
        guard input.hasPrefix(phrase) else { return false }
        guard input.count > phrase.count else { return true }
        let boundary = input.index(input.startIndex, offsetBy: phrase.count)
        let next = input[boundary]
        return next.isWhitespace || ":：,，。.!！".contains(next)
    }

    private static func looksLikeQuestion(_ input: String) -> Bool {
        if input.contains("?") || input.contains("？") { return true }
        return [
            "how ", "how do ", "what ", "why ", "can ",
            "如何", "怎么", "为什么", "什么是", "介绍", "解释",
        ].contains(where: { input.hasPrefix($0) })
    }

    private static func boundedArgument(_ input: String) -> String {
        var trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
        while let first = trimmed.first, ":：,，".contains(first) {
            trimmed.removeFirst()
            trimmed = trimmed.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        return String(trimmed.prefix(maximumArgumentLength))
    }

    private static func admittedRequest(
        definition: ConversationActionDefinition,
        argument: String,
        source: ConversationActionSource
    ) -> ConversationActionResolution {
        let bounded = boundedArgument(argument)
        guard !looksLikeCredential(bounded) else {
            return .rejectedActionArgument(definition.id, .credentialLike)
        }
        guard definition.argumentHint != nil else {
            return .request(ConversationActionRequest(
                action: definition.id,
                source: source
            ))
        }
        return .request(ConversationActionRequest(
            action: definition.id,
            argument: bounded,
            source: source
        ))
    }

    static func looksLikeCredential(_ input: String) -> Bool {
        let tokens = input.split(whereSeparator: { $0.isWhitespace })
        let markerPrefixes = [
            "sk-", "sk_", "api_key=", "api-key=", "apikey=",
            "--api-key=", "authorization:", "x-api-key:",
        ]
        for token in tokens {
            let normalized = token.lowercased().trimmingCharacters(
                in: CharacterSet(charactersIn: "\"'([{<")
            )
            if normalized == "bearer"
                || markerPrefixes.contains(where: { normalized.hasPrefix($0) }) {
                return true
            }
            guard token.count >= 40 else { continue }
            let hasLetter = token.contains(where: { $0.isLetter })
            let hasNumber = token.contains(where: { $0.isNumber })
            let hasKeyPunctuation = token.contains("_") || token.contains("-")
            if hasLetter && hasNumber && hasKeyPunctuation {
                return true
            }
        }
        return false
    }
}

public enum ConversationActionMenuDirection: Hashable, Sendable {
    case next
    case previous
}

public struct ConversationActionMenuState: Hashable, Sendable {
    public private(set) var selectedID: ConversationActionID?

    public init(selectedID: ConversationActionID? = nil) {
        self.selectedID = selectedID
    }

    public mutating func synchronize(with input: String) {
        let suggestions = ConversationActionCatalog.suggestions(for: input)
        guard !suggestions.isEmpty else {
            selectedID = nil
            return
        }
        if let selectedID, suggestions.contains(where: { $0.id == selectedID }) {
            return
        }
        selectedID = suggestions[0].id
    }

    public mutating func move(
        _ direction: ConversationActionMenuDirection,
        for input: String
    ) {
        let suggestions = ConversationActionCatalog.suggestions(for: input)
        guard !suggestions.isEmpty else {
            selectedID = nil
            return
        }
        let currentIndex = selectedID.flatMap { selected in
            suggestions.firstIndex(where: { $0.id == selected })
        } ?? 0
        switch direction {
        case .next:
            selectedID = suggestions[(currentIndex + 1) % suggestions.count].id
        case .previous:
            selectedID = suggestions[
                (currentIndex - 1 + suggestions.count) % suggestions.count
            ].id
        }
    }

    public func request(for input: String) -> ConversationActionRequest? {
        guard case .request(let request) = resolution(for: input) else {
            return nil
        }
        return request
    }

    public func resolution(for input: String) -> ConversationActionResolution {
        guard let selectedID,
              ConversationActionCatalog.suggestions(for: input)
                .contains(where: { $0.id == selectedID }) else {
            return .chat
        }
        return ConversationActionRouter.commandMenuResolution(
            action: selectedID,
            input: input
        )
    }

    public mutating func clear() {
        selectedID = nil
    }
}
