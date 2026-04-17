#!/usr/bin/env python3
from pathlib import Path
import sys
import yaml

path = Path(sys.argv[1] if len(sys.argv) > 1 else Path.home() / '.config' / 'pinocchio' / 'profiles.yaml')
if not path.exists():
    print('PROFILES_MISSING')
    raise SystemExit(0)
with path.open() as f:
    data = yaml.safe_load(f) or {}
profiles = []
if isinstance(data, dict):
    p = data.get('profiles')
    if isinstance(p, list):
        for item in p:
            if isinstance(item, dict):
                slug = item.get('slug') or item.get('name') or item.get('id')
                if slug:
                    profiles.append(str(slug))
    elif isinstance(p, dict):
        profiles.extend(sorted(str(k) for k in p.keys()))
for slug in sorted(set(profiles)):
    print(slug)
