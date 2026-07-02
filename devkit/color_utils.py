"""Color format conversion utilities.

Pure standard library. No external dependencies.

Functions:
    hex_to_rgb(hex_color)  -> (R, G, B)
    rgb_to_hex(r, g, b)    -> '#rrggbb'   (lowercase, 6 digits)
    rgb_to_hsl(r, g, b)    -> (H: 0-360, S: 0-1, L: 0-1)
    blend(c1, c2, ratio)   -> (R, G, B)  linear interpolation

All channels are 8-bit integers in [0, 255]. H is degrees, S/L are fractions.
"""

from __future__ import annotations

def _clamp_byte(n: int) -> int:
    """Clamp an integer into the 8-bit channel range [0, 255]."""
    if n < 0:
        return 0
    if n > 255:
        return 255
    return n

def hex_to_rgb(hex_color: str) -> tuple[int, int, int]:
    """Convert '#RRGGBB' (or 'RRGGBB') to (R, G, B).

    Accepts an optional leading '#'. Raises ValueError on malformed input.
    """
    if not isinstance(hex_color, str):
        raise TypeError(f"hex_color must be a str, got {type(hex_color).__name__}")
    s = hex_color.strip()
    if s.startswith("#"):
        s = s[1:]
    if len(s) != 6:
        raise ValueError(f"hex_color must be 6 hex digits, got {s!r}")
    try:
        r = int(s[0:2], 16)
        g = int(s[2:4], 16)
        b = int(s[4:6], 16)
    except ValueError as exc:
        raise ValueError(f"hex_color contains non-hex chars: {hex_color!r}") from exc
    return (r, g, b)

def rgb_to_hex(r: int, g: int, b: int) -> str:
    """Convert (R, G, B) to '#rrggbb' (lowercase, always 6 digits)."""
    return "#{:02x}{:02x}{:02x}".format(
        _clamp_byte(int(r)),
        _clamp_byte(int(g)),
        _clamp_byte(int(b)),
    )

def rgb_to_hsl(r: int, g: int, b: int) -> tuple[float, float, float]:
    """Convert (R, G, B) in [0,255] to (H: 0-360, S: 0-1, L: 0-1).

    H is in degrees. When max == min (achromatic), H is defined as 0.
    """
    rf = _clamp_byte(int(r)) / 255.0
    gf = _clamp_byte(int(g)) / 255.0
    bf = _clamp_byte(int(b)) / 255.0

    cmax = max(rf, gf, bf)
    cmin = min(rf, gf, bf)
    delta = cmax - cmin

    # Lightness
    lightness = (cmax + cmin) / 2.0

    # Saturation
    if delta == 0.0:
        saturation = 0.0
    else:
        saturation = delta / (1.0 - abs(2.0 * lightness - 1.0))

    # Hue
    if delta == 0.0:
        hue = 0.0
    elif cmax == rf:
        hue = ((gf - bf) / delta) % 6.0
    elif cmax == gf:
        hue = ((bf - rf) / delta) + 2.0
    else:  # cmax == bf
        hue = ((rf - gf) / delta) + 4.0
    hue *= 60.0
    if hue < 0.0:
        hue += 360.0
    elif hue >= 360.0:
        hue -= 360.0

    return (hue, saturation, lightness)

def blend(
    c1: tuple[int, int, int],
    c2: tuple[int, int, int],
    ratio: float = 0.5,
) -> tuple[int, int, int]:
    """Linearly blend two RGB colors.

    ratio=0.0 returns c1, ratio=1.0 returns c2. Result is rounded to nearest
    int and clamped to [0, 255].
    """
    if len(c1) != 3 or len(c2) != 3:
        raise ValueError("color tuples must have length 3")
    try:
        t = float(ratio)
    except (TypeError, ValueError) as exc:
        raise TypeError(f"ratio must be numeric, got {type(ratio).__name__}") from exc
    if t < 0.0:
        t = 0.0
    elif t > 1.0:
        t = 1.0
    out = []
    for a, b in zip(c1, c2):
        v = round((1.0 - t) * a + t * b)
        out.append(_clamp_byte(v))
    return (out[0], out[1], out[2])
