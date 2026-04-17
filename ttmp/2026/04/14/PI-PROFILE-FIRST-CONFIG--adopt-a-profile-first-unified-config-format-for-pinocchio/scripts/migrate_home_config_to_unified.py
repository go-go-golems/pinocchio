#!/usr/bin/env python3
from pathlib import Path
import shutil
import sys
import yaml

home = Path.home()
src = Path(sys.argv[1] if len(sys.argv) > 1 else home / '.pinocchio' / 'config.yaml')
if not src.exists():
    print(f'missing config: {src}', file=sys.stderr)
    raise SystemExit(1)

with src.open() as f:
    data = yaml.safe_load(f) or {}
if not isinstance(data, dict):
    print(f'expected mapping in {src}, got {type(data).__name__}', file=sys.stderr)
    raise SystemExit(1)

legacy_profile = data.get('profile-settings') or {}
legacy_repositories = data.get('repositories') or []

out = {}
if legacy_repositories:
    out['app'] = {'repositories': legacy_repositories}

profile = {}
legacy_active = (legacy_profile.get('profile') or '').strip() if isinstance(legacy_profile, dict) else ''
legacy_registries = legacy_profile.get('profile-registries') if isinstance(legacy_profile, dict) else None
if legacy_active:
    profile['active'] = legacy_active
if legacy_registries:
    profile['registries'] = legacy_registries
if profile:
    out['profile'] = profile

backup = src.with_name(src.name + '.bak-before-unified-write')
shutil.copy2(src, backup)
with src.open('w') as f:
    yaml.safe_dump(out, f, sort_keys=False, default_flow_style=False)

summary = {
    'path': str(src),
    'backup': str(backup),
    'wrote_keys': list(out.keys()),
    'repository_count': len(out.get('app', {}).get('repositories', [])),
    'has_profile_active': 'active' in out.get('profile', {}),
    'registry_count': len(out.get('profile', {}).get('registries', [])),
}
print(yaml.safe_dump(summary, sort_keys=False))
