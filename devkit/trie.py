# devkit/trie.py

def create():
    return {'children': {}, 'is_end': False, 'count': 0}

def insert(trie, word):
    t = trie
    t['count'] += 1
    for ch in word:
        if ch not in t['children']:
            t['children'][ch] = {'children': {}, 'is_end': False, 'count': 0}
        t = t['children'][ch]
    t['is_end'] = True
    return trie

def search(trie, word):
    t = trie
    for ch in word:
        if ch not in t['children']:
            return False
        t = t['children'][ch]
    return t['is_end']

def starts_with(trie, prefix):
    t = trie
    for ch in prefix:
        if ch not in t['children']:
            return False
        t = t['children'][ch]
    return True

def trie_summary(trie):
    return {'count': trie['count'], 'root_children': len(trie['children'])}
