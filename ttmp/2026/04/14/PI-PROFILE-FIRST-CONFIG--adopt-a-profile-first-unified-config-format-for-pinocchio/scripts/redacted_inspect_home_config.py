#!/usr/bin/env python3
from pathlib import Path
import sys
import yaml

SECRET_TOKENS = {'key','token','secret','password','auth','bearer','api_key','apikey','access_key'}

def redact(v):
    if isinstance(v, dict):
        out = {}
        for k, val in v.items():
            kl = str(k).lower()
            if any(tok in kl for tok in SECRET_TOKENS):
                out[k] = '<redacted>'
            else:
                out[k] = redact(val)
        return out
    if isinstance(v, list):
        return [redact(x) for x in v]
    if isinstance(v, str):
        return v[:6] + '…<redacted>' if len(v) > 24 else v
    return v

path = Path(sys.argv[1] if len(sys.argv) > 1 else Path.home() / '.pinocchio' / 'config.yaml')
if not path.exists():
    print('CONFIG_MISSING')
    raise SystemExit(0)
with path.open() as f:
    data = yaml.safe_load(f) or {}
print('TOP_LEVEL_KEYS', sorted(list(data.keys())) if isinstance(data, dict) else type(data).__name__)
print(yaml.safe_dump(redact(data), sort_keys=False))
