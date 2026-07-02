# text_stats.py

def word_count(text: str) -> int:
    """Return number of words (split by whitespace, ignore empty strings)."""
    return len(text.split())

def char_count(text: str, ignore_spaces: bool = False) -> int:
    """Return character count; if ignore_spaces is True, spaces are excluded."""
    if ignore_spaces:
        return sum(1 for ch in text if ch != ' ')
    return len(text)

def word_freq(text: str) -> dict:
    """Return dictionary {word: count} (case-insensitive, all lowercase)."""
    words = text.lower().split()
    freq = {}
    for w in words:
        freq[w] = freq.get(w, 0) + 1
    return freq

def top_words(text: str, n: int) -> list[tuple]:
    """Return top n (word, count) pairs sorted by frequency (descending),
    then alphabetically for ties. Returns at most n entries."""
    freq = word_freq(text)
    sorted_items = sorted(freq.items(), key=lambda x: (-x[1], x[0]))
    return sorted_items[:n]
