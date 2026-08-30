import LoomLocalAppCore
import SwiftUI

struct ConversationActionPalette: View {
    let suggestions: [ConversationActionDefinition]
    let selectedID: ConversationActionID?
    let context: ConversationActionContext
    let onSelect: (ConversationActionDefinition) -> Void
    let onDismiss: () -> Void

    private var groups: [(ConversationActionGroup, [ConversationActionDefinition])] {
        ConversationActionGroup.allCases.compactMap { group in
            let definitions = suggestions.filter { $0.group == group }
            return definitions.isEmpty ? nil : (group, definitions)
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 9) {
                Image(systemName: "command")
                    .foregroundStyle(LoomGraphite.accent)
                Text("Commands")
                    .font(.callout.weight(.semibold))
                Spacer()
                Text("\(suggestions.count)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
                    .accessibilityLabel("\(suggestions.count) matching commands")
                Button(action: onDismiss) {
                    Image(systemName: "xmark")
                        .frame(width: 24, height: 24)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Close commands")
                .help("Close commands")
            }
            .padding(.horizontal, 12)
            .frame(minHeight: 42)

            Divider()

            if suggestions.isEmpty {
                ContentUnavailableView(
                    "No matching commands",
                    systemImage: "command"
                )
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .accessibilityIdentifier("loom.conversation.commands.empty")
            } else {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 2) {
                            ForEach(groups, id: \.0) { group, definitions in
                                Text(group.rawValue)
                                    .font(.caption2.weight(.semibold))
                                    .foregroundStyle(.secondary)
                                    .padding(.horizontal, 12)
                                    .padding(.top, 9)
                                    .padding(.bottom, 2)

                                ForEach(definitions) { definition in
                                    actionRow(definition)
                                        .id(definition.id)
                                }
                            }
                        }
                        .padding(.bottom, 8)
                    }
                    .onChange(of: selectedID) { _, next in
                        guard let next else { return }
                        withAnimation(.easeOut(duration: 0.12)) {
                            proxy.scrollTo(next, anchor: .center)
                        }
                    }
                }
            }
        }
        .frame(maxWidth: 680)
        .frame(height: 300)
        .background(LoomGraphite.surface)
        .overlay {
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .stroke(LoomGraphite.separator, lineWidth: 1)
        }
        .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        .shadow(color: .black.opacity(0.14), radius: 14, y: 6)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Slash commands")
        .accessibilityIdentifier("loom.conversation.commands")
    }

    private func actionRow(
        _ definition: ConversationActionDefinition
    ) -> some View {
        let availability = ConversationActionCatalog.availability(
            of: definition.id,
            in: context
        )
        let unavailableReason: String? = {
            guard case .unavailable(let reason) = availability else { return nil }
            return reason
        }()
        return Button {
            onSelect(definition)
        } label: {
            HStack(alignment: .center, spacing: 10) {
                Image(systemName: definition.systemImage)
                    .font(.callout)
                    .foregroundStyle(
                        unavailableReason == nil
                            ? LoomGraphite.accent
                            : Color.secondary
                    )
                    .frame(width: 24)
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: 7) {
                        Text("/\(definition.command)")
                            .font(.callout.monospaced().weight(.medium))
                            .foregroundStyle(.primary)
                        Text(definition.title)
                            .font(.callout)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                    Text(unavailableReason ?? definition.detail)
                        .font(.caption)
                        .foregroundStyle(
                            unavailableReason == nil
                                ? Color.secondary
                                : LoomGraphite.statusWarning
                        )
                        .lineLimit(2)
                }
                Spacer(minLength: 8)
                if selectedID == definition.id {
                    Image(systemName: "return")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .accessibilityHidden(true)
                }
            }
            .padding(.horizontal, 10)
            .frame(minHeight: 50)
            .frame(maxWidth: .infinity, alignment: .leading)
            .contentShape(Rectangle())
            .background(
                selectedID == definition.id
                    ? LoomGraphite.accentMuted
                    : Color.clear,
                in: RoundedRectangle(cornerRadius: 6, style: .continuous)
            )
        }
        .buttonStyle(.plain)
        .disabled(unavailableReason != nil)
        .padding(.horizontal, 4)
        .accessibilityLabel(
            ["\(definition.title), /\(definition.command)", unavailableReason]
                .compactMap { $0 }
                .joined(separator: ", ")
        )
        .accessibilityAddTraits(selectedID == definition.id ? .isSelected : [])
        .accessibilityIdentifier("loom.conversation.command.\(definition.id.rawValue)")
    }
}

struct ConversationActionChoice: Identifiable, Equatable {
    let id: String
    let title: String
    let detail: String
    let systemImage: String
    let isSelected: Bool
    let unavailableReason: String?

    init(
        id: String,
        title: String,
        detail: String,
        systemImage: String,
        isSelected: Bool = false,
        unavailableReason: String? = nil
    ) {
        self.id = id
        self.title = title
        self.detail = detail
        self.systemImage = systemImage
        self.isSelected = isSelected
        self.unavailableReason = unavailableReason
    }
}

struct ConversationActionChoiceMenuState: Equatable {
    private(set) var selectedID: String?

    init(selectedID: String? = nil) {
        self.selectedID = selectedID
    }

    mutating func synchronize(with choices: [ConversationActionChoice]) {
        let available = choices.filter { $0.unavailableReason == nil }
        guard !available.isEmpty else {
            selectedID = nil
            return
        }
        if let selectedID, available.contains(where: { $0.id == selectedID }) {
            return
        }
        selectedID = available.first(where: \.isSelected)?.id ?? available[0].id
    }

    mutating func move(
        _ direction: ConversationActionMenuDirection,
        within choices: [ConversationActionChoice]
    ) {
        let available = choices.filter { $0.unavailableReason == nil }
        guard !available.isEmpty else {
            selectedID = nil
            return
        }
        let currentIndex = selectedID.flatMap { selected in
            available.firstIndex(where: { $0.id == selected })
        } ?? 0
        switch direction {
        case .next:
            selectedID = available[(currentIndex + 1) % available.count].id
        case .previous:
            selectedID = available[
                (currentIndex - 1 + available.count) % available.count
            ].id
        }
    }

    func selectedChoice(
        in choices: [ConversationActionChoice]
    ) -> ConversationActionChoice? {
        guard let selectedID else { return nil }
        return choices.first {
            $0.id == selectedID && $0.unavailableReason == nil
        }
    }

    mutating func clear() {
        selectedID = nil
    }
}

struct ConversationActionChoicePalette: View {
    let title: String
    let choices: [ConversationActionChoice]
    let selectedID: String?
    let onSelect: (ConversationActionChoice) -> Void
    let onDismiss: () -> Void

    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 9) {
                Text(title)
                    .font(.callout.weight(.semibold))
                Spacer()
                Button(action: onDismiss) {
                    Image(systemName: "xmark")
                        .frame(width: 24, height: 24)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Close \(title)")
                .help("Close")
            }
            .padding(.horizontal, 12)
            .frame(minHeight: 42)

            Divider()

            if choices.isEmpty {
                ContentUnavailableView(
                    "No matching options",
                    systemImage: "line.3.horizontal.decrease.circle"
                )
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .accessibilityIdentifier("loom.conversation.choices.empty")
            } else {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(spacing: 2) {
                            ForEach(choices) { choice in
                                Button {
                                    onSelect(choice)
                                } label: {
                                    HStack(spacing: 10) {
                                        Image(systemName: choice.systemImage)
                                            .foregroundStyle(
                                                choice.unavailableReason == nil
                                                    ? LoomGraphite.accent
                                                    : Color.secondary
                                            )
                                            .frame(width: 24)
                                        VStack(alignment: .leading, spacing: 2) {
                                            Text(choice.title)
                                                .font(.callout.weight(.medium))
                                                .lineLimit(2)
                                            Text(choice.unavailableReason ?? choice.detail)
                                                .font(.caption)
                                                .foregroundStyle(
                                                    choice.unavailableReason == nil
                                                        ? Color.secondary
                                                        : LoomGraphite.statusWarning
                                                )
                                                .lineLimit(2)
                                        }
                                        Spacer(minLength: 8)
                                        if choice.isSelected {
                                            Image(systemName: "checkmark")
                                                .foregroundStyle(LoomGraphite.accent)
                                                .accessibilityHidden(true)
                                        }
                                        if selectedID == choice.id {
                                            Image(systemName: "return")
                                                .font(.caption2)
                                                .foregroundStyle(.secondary)
                                                .accessibilityHidden(true)
                                        }
                                    }
                                    .padding(.horizontal, 10)
                                    .frame(minHeight: 52)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                    .contentShape(Rectangle())
                                    .background(
                                        selectedID == choice.id
                                            ? LoomGraphite.accentMuted
                                            : Color.clear,
                                        in: RoundedRectangle(
                                            cornerRadius: 6,
                                            style: .continuous
                                        )
                                    )
                                }
                                .buttonStyle(.plain)
                                .disabled(choice.unavailableReason != nil)
                                .padding(.horizontal, 4)
                                .id(choice.id)
                                .accessibilityLabel(
                                    [choice.title, choice.unavailableReason]
                                        .compactMap { $0 }
                                        .joined(separator: ", ")
                                )
                                .accessibilityAddTraits(
                                    choice.isSelected || selectedID == choice.id
                                        ? .isSelected
                                        : []
                                )
                                .accessibilityIdentifier(
                                    "loom.conversation.choice.\(choice.id)"
                                )
                            }
                        }
                        .padding(.vertical, 4)
                    }
                    .onChange(of: selectedID) { _, next in
                        guard let next else { return }
                        withAnimation(.easeOut(duration: 0.12)) {
                            proxy.scrollTo(next, anchor: .center)
                        }
                    }
                }
            }
        }
        .frame(maxWidth: 680)
        .frame(height: 300)
        .background(LoomGraphite.surface)
        .overlay {
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .stroke(LoomGraphite.separator, lineWidth: 1)
        }
        .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        .shadow(color: .black.opacity(0.14), radius: 14, y: 6)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("loom.conversation.choices")
    }
}
