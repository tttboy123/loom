import Foundation

public enum SafeText {
    private static let bidiControls: Set<UnicodeScalar> = [
        "\u{061C}", "\u{200E}", "\u{200F}",
        "\u{202A}", "\u{202B}", "\u{202C}", "\u{202D}", "\u{202E}",
        "\u{2066}", "\u{2067}", "\u{2068}", "\u{2069}",
    ]

    public static func sanitize(_ value: String, limit: Int = 256) -> String {
        let boundedLimit = max(1, min(limit, 4_096))
        var scalars: [UnicodeScalar] = []
        var index = value.unicodeScalars.startIndex

        while index < value.unicodeScalars.endIndex {
            let scalar = value.unicodeScalars[index]
            if scalar.value == 0x1B {
                index = skipEscape(in: value.unicodeScalars, from: index)
                continue
            }
            if scalar == "\n" || scalar == "\t" {
                if scalars.last != " " {
                    scalars.append(" ")
                }
            } else if !bidiControls.contains(scalar),
                      !CharacterSet.controlCharacters.contains(scalar) {
                scalars.append(scalar)
            }
            index = value.unicodeScalars.index(after: index)
        }

        let cleaned = String(String.UnicodeScalarView(scalars))
        guard cleaned.count > boundedLimit else {
            return cleaned
        }
        return String(cleaned.prefix(boundedLimit)) + "…"
    }

    private static func skipEscape(
        in scalars: String.UnicodeScalarView,
        from start: String.UnicodeScalarView.Index
    ) -> String.UnicodeScalarView.Index {
        var cursor = scalars.index(after: start)
        guard cursor < scalars.endIndex else { return cursor }
        if scalars[cursor] == "[" {
            cursor = scalars.index(after: cursor)
            while cursor < scalars.endIndex {
                let value = scalars[cursor].value
                cursor = scalars.index(after: cursor)
                if value >= 0x40 && value <= 0x7E {
                    break
                }
            }
            return cursor
        }
        if scalars[cursor] == "]" {
            cursor = scalars.index(after: cursor)
            while cursor < scalars.endIndex {
                if scalars[cursor].value == 0x07 {
                    return scalars.index(after: cursor)
                }
                if scalars[cursor].value == 0x1B {
                    let next = scalars.index(after: cursor)
                    if next < scalars.endIndex, scalars[next] == "\\" {
                        return scalars.index(after: next)
                    }
                }
                cursor = scalars.index(after: cursor)
            }
            return cursor
        }
        return scalars.index(after: cursor)
    }
}
