import re
from collections import Counter
from urllib.parse import urlparse

sql = open(r'D:\project\Syntopica\demo\seed\seed.sql', encoding='utf-8').read()

m = re.search(r'INSERT INTO feeds \(([^)]+)\) VALUES', sql)
cols = [c.strip() for c in m.group(1).split(',')]
idx = cols.index('url')
print('url index:', idx)

def split_vals(row):
    vals, cur, inq, esc = [], '', False, False
    for ch in row:
        if esc:
            cur += ch; esc = False; continue
        if ch == '\\':
            cur += ch; esc = True; continue
        if ch == "'":
            cur += ch; inq = not inq; continue
        if ch == ',' and not inq:
            vals.append(cur.strip()); cur = ''; continue
        cur += ch
    vals.append(cur.strip())
    return vals

hosts = Counter()
samples = []
for block in re.finditer(r'INSERT INTO feeds \(.*?\) VALUES\n(.*?);', sql, re.S):
    for row in block.group(1).split('\n'):
        row = row.strip().rstrip(',')
        if not row.startswith('('):
            continue
        vals = split_vals(row)
        if len(vals) <= idx:
            continue
        v = vals[idx].strip()
        if v.startswith("'") and v.endswith("'"):
            v = v[1:-1]
        if not v:
            continue
        try:
            h = urlparse(v).netloc
            hosts[h] += 1
            if len(samples) < 8:
                samples.append(v)
        except Exception:
            pass

print('host distribution:')
for h, c in hosts.most_common():
    print(f'  {c:4d}  {h}')
print('sample urls:')
for s in samples:
    print('  ', s)
