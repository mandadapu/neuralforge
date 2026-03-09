import urllib.request
import json

for pkg, version in [('minimatch', '10.0.1'), ('brace-expansion', '2.0.1'), ('balanced-match', '1.0.2')]:
    url = f'https://registry.npmjs.org/{pkg}/{version}'
    with urllib.request.urlopen(url) as r:
        d = json.load(r)
        print(f"{pkg}@{version}: {d['dist']['integrity']}")
