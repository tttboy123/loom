# devkit/string_distance.py
"""String distance algorithms using only standard library."""

def levenshtein(a: str, b: str) -> int:
    """Compute Levenshtein edit distance between two strings."""
    n, m = len(a), len(b)
    # Use the shorter string as the column dimension to save memory
    if n < m:
        a, b = b, a
        n, m = m, n

    # prev_row and curr_row represent dp[i-1] and dp[i]
    prev_row = list(range(m + 1))
    curr_row = [0] * (m + 1)

    for i in range(1, n + 1):
        curr_row[0] = i
        for j in range(1, m + 1):
            cost = 0 if a[i - 1] == b[j - 1] else 1
            curr_row[j] = min(
                prev_row[j] + 1,       # deletion
                curr_row[j - 1] + 1,   # insertion
                prev_row[j - 1] + cost # substitution
            )
        prev_row, curr_row = curr_row, prev_row

    return prev_row[m]

def hamming(a: str, b: str) -> int:
    """Compute Hamming distance (number of differing positions).
    
    Raises ValueError if strings have different lengths.
    """
    if len(a) != len(b):
        raise ValueError("Strings must have equal length for Hamming distance")
    return sum(1 for ca, cb in zip(a, b) if ca != cb)

def similarity(a: str, b: str) -> float:
    """Compute similarity as 1 - lev(a,b)/max(len(a),len(b),1)."""
    max_len = max(len(a), len(b), 1)
    return 1.0 - levenshtein(a, b) / max_len
