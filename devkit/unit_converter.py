import math

_LENGTH = {
    'm': 1.0,
    'km': 1000.0,
    'cm': 0.01,
    'ft': 0.3048,
    'mi': 1609.344,
}

_WEIGHT = {
    'kg': 1.0,
    'g': 0.001,
    'lb': 0.453592,
    'oz': 0.0283495,
}

def convert(value, from_unit, to_unit):
    f, t = from_unit.lower(), to_unit.lower()
    if f in ('c', 'f', 'k') or t in ('c', 'f', 'k'):
        if f == 'c' and t == 'f':
            return value * 9 / 5 + 32
        if f == 'f' and t == 'c':
            return (value - 32) * 5 / 9
        if f == 'c' and t == 'k':
            return value + 273.15
        if f == 'k' and t == 'c':
            return value - 273.15
        if f == 'f' and t == 'k':
            return (value - 32) * 5 / 9 + 273.15
        if f == 'k' and t == 'f':
            return (value - 273.15) * 9 / 5 + 32
    if f in _LENGTH and t in _LENGTH:
        return value * _LENGTH[f] / _LENGTH[t]
    if f in _WEIGHT and t in _WEIGHT:
        return value * _WEIGHT[f] / _WEIGHT[t]
    raise ValueError(f"Cannot convert {from_unit} to {to_unit}")

def available_units(category):
    if category == 'length':
        return list(_LENGTH.keys())
    if category == 'weight':
        return list(_WEIGHT.keys())
    if category == 'temperature':
        return ['c', 'f', 'k']
    return []
