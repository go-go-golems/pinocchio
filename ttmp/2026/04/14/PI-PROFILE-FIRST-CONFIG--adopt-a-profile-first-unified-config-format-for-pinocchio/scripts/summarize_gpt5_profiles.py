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
profiles = data.get('profiles') or {}

def summarize_profile(name, prof):
    if not isinstance(prof, dict):
        return {'slug': name, 'type': type(prof).__name__}
    inf = prof.get('inference_settings') or {}
    chat = inf.get('chat') or {}
    api_keys = chat.get('api_keys') or inf.get('api_keys') or {}
    return {
        'slug': prof.get('slug', name),
        'display_name': prof.get('display_name'),
        'api_type': chat.get('api_type'),
        'engine': chat.get('engine'),
        'has_api_keys': bool(api_keys),
        'api_key_provider_keys': sorted(api_keys.keys()) if isinstance(api_keys, dict) else '<non-map>',
        'stack_len': len(prof.get('stack') or []),
    }

items = []
if isinstance(profiles, list):
    for item in profiles:
        if isinstance(item, dict):
            slug = str(item.get('slug') or item.get('name') or item.get('id') or '')
            if slug.startswith('gpt-5'):
                items.append(summarize_profile(slug, item))
elif isinstance(profiles, dict):
    for name, prof in sorted(profiles.items()):
        if str(name).startswith('gpt-5'):
            items.append(summarize_profile(str(name), prof))
for item in items:
    print(item)
