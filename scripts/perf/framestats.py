import re, sys
rows = []
for line in sys.stdin:
    m = re.search(r'took ([\d.]+) ms: build ([\d.]+).*?layout ([\d.]+), paint ([\d.]+), present ([\d.]+) \((\w+)', line)
    if m:
        rows.append([float(x) for x in m.groups()[:5]] + [m.group(6)])
shown = [r for r in rows if r[5] == 'drawn']
def q(a, x):
    a = sorted(a); return a[min(len(a) - 1, int(len(a) * x))]
print(sys.argv[1], len(rows), "frames,", len(shown), "on the GPU")
for i, name in enumerate(["total", "build", "layout", "paint", "present"]):
    a = [r[i] for r in shown] or [0]
    print("  %-8s p50 %5.1f  p90 %5.1f  p99 %5.1f  max %5.1f ms" % (name, q(a, .5), q(a, .9), q(a, .99), max(a)))
