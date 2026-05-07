#!/usr/bin/env python3
"""devctl plugin for pinocchio web-chat: manages Go backend + Vite frontend."""

import json
import os
import shlex
import shutil
import socket
import subprocess
import sys


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def log(msg):
    sys.stderr.write(msg + "\n")
    sys.stderr.flush()


def find_free_port(preferred):
    """Try preferred port first, then find any free port."""
    if is_port_free(preferred):
        return preferred
    return pick_free_port()


def is_port_free(port):
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(0.2)
        s.bind(("127.0.0.1", port))
        s.close()
        return True
    except OSError:
        return False


def pick_free_port():
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def go_module_root(repo_root):
    """Walk up from repo_root to find the Go module root (go.mod)."""
    d = os.path.abspath(repo_root)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, "go.mod")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return repo_root


def webchat_root(repo_root):
    """Support devctl from either repo root or cmd/web-chat."""
    repo_root = os.path.abspath(repo_root or os.getcwd())
    if os.path.isfile(os.path.join(repo_root, "plugins", "webchat.py")) and os.path.isdir(os.path.join(repo_root, "web")):
        return repo_root
    candidate = os.path.join(repo_root, "cmd", "web-chat")
    if os.path.isfile(os.path.join(candidate, "plugins", "webchat.py")) and os.path.isdir(os.path.join(candidate, "web")):
        return candidate
    return repo_root


def relpath(path, root):
    return os.path.relpath(path, root)


def env_bool(name, default=False):
    raw = os.environ.get(name, "")
    if raw == "":
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def env_int(name, default):
    raw = os.environ.get(name, "")
    if raw.strip() == "":
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def env_str(name, default=""):
    return os.environ.get(name, default).strip()


def shell_join(parts):
    return " ".join(shlex.quote(str(p)) for p in parts if str(p) != "")


# --- handshake ---
emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "pinocchio-webchat",
        "capabilities": {
            "ops": ["config.mutate", "validate.run", "build.run", "prepare.run", "launch.plan", "command.run"],
            "commands": [
                {
                    "name": "build-frontend",
                    "help": "Build the Vite frontend for embedded serving",
                    "args_spec": [],
                },
                {
                    "name": "build-backend",
                    "help": "Build the Go web-chat binary",
                    "args_spec": [],
                },
            ],
        },
    }
)

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")
    ctx = req.get("ctx", {}) or {}
    inp = req.get("input", {}) or {}
    repo_root = ctx.get("repo_root", "")

    try:
        # ---- config.mutate ----
        if op == "config.mutate":
            backend_port = find_free_port(env_int("PINOCCHIO_WEBCHAT_BACKEND_PORT", 8092))
            vite_port = find_free_port(env_int("PINOCCHIO_WEBCHAT_VITE_PORT", 5174))
            backend_origin = f"http://127.0.0.1:{backend_port}"
            profile = env_str("PINOCCHIO_WEBCHAT_PROFILE")
            profile_registries = env_str("PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES")
            trace_level = env_str("PINOCCHIO_WEBCHAT_TRACE_LEVEL", "off") or "off"
            root = env_str("PINOCCHIO_WEBCHAT_ROOT", "/") or "/"
            debug_api = env_bool("PINOCCHIO_WEBCHAT_DEBUG_API", True)

            log(
                "config: "
                + f"backend_port={backend_port} vite_port={vite_port} "
                + f"profile={profile or '(config/default)'} trace={trace_level}"
            )

            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {
                        "config_patch": {
                            "set": {
                                "services.backend.port": backend_port,
                                "services.backend.url": backend_origin,
                                "services.vite.port": vite_port,
                                "services.vite.url": f"http://127.0.0.1:{vite_port}",
                                "webchat.profile": profile,
                                "webchat.profile_registries": profile_registries,
                                "webchat.trace_level": trace_level,
                                "webchat.debug_api": debug_api,
                                "webchat.root": root,
                                "env.VITE_BACKEND_ORIGIN": backend_origin,
                            },
                            "unset": [],
                        }
                    },
                }
            )

        # ---- validate.run ----
        elif op == "validate.run":
            errors = []
            warnings = []

            app_root = webchat_root(repo_root)
            mod_root = go_module_root(app_root)
            web_dir = os.path.join(app_root, "web")
            node_modules = os.path.join(web_dir, "node_modules")
            if not os.path.isdir(node_modules):
                errors.append(
                    {
                        "message": f"node_modules missing: run 'cd {relpath(web_dir, repo_root)} && pnpm install'",
                        "key": "frontend.node_modules",
                    }
                )

            go_mod = os.path.join(mod_root, "go.mod")
            for tool in ["go", "node", "npx"]:
                if shutil.which(tool) is None:
                    errors.append({"message": f"required tool not found on PATH: {tool}", "key": f"tool.{tool}"})
            if not os.path.isdir(node_modules) and shutil.which("pnpm") is None:
                errors.append({"message": "node_modules is missing and pnpm is not on PATH", "key": "tool.pnpm"})
            if not os.path.isfile(go_mod):
                warnings.append(
                    {
                        "message": "go.mod not found above repo_root; build.run may fail",
                        "key": "go.mod",
                    }
                )

            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {"valid": len(errors) == 0, "errors": errors, "warnings": warnings},
                }
            )

        # ---- build.run ----
        elif op == "build.run":
            dry_run = ctx.get("dry_run", False)
            app_root = webchat_root(repo_root)
            mod_root = go_module_root(app_root)
            bin_path = os.path.join(app_root, "bin", "web-chat")
            bin_dir = os.path.dirname(bin_path)
            steps = []

            if not dry_run:
                os.makedirs(bin_dir, exist_ok=True)
                log(f"building Go binary -> {bin_path} (from {mod_root})")
                result = subprocess.run(
                    ["go", "build", "-o", bin_path, "./cmd/web-chat"],
                    cwd=mod_root,
                    capture_output=True,
                    text=True,
                    timeout=120,
                )
                if result.returncode != 0:
                    emit(
                        {
                            "type": "response",
                            "request_id": rid,
                            "ok": False,
                            "error": {
                                "code": "E_BUILD_FAILED",
                                "message": f"go build failed: {result.stderr[:500]}",
                            },
                        }
                    )
                    continue
            steps.append(
                {
                    "name": "build-backend",
                    "ok": True,
                    "output": {"binary": bin_path, "dry_run": bool(dry_run)},
                }
            )

            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {"steps": steps, "artifacts": {"binary": bin_path}},
                }
            )

        # ---- prepare.run ----
        elif op == "prepare.run":
            dry_run = ctx.get("dry_run", False)
            steps = []

            app_root = webchat_root(repo_root)
            web_dir = os.path.join(app_root, "web")
            node_modules = os.path.join(web_dir, "node_modules")
            if not os.path.isdir(node_modules) and not dry_run:
                log(f"installing frontend dependencies in {web_dir}")
                result = subprocess.run(
                    ["pnpm", "install"],
                    cwd=web_dir,
                    capture_output=True,
                    text=True,
                    timeout=120,
                )
                if result.returncode != 0:
                    emit(
                        {
                            "type": "response",
                            "request_id": rid,
                            "ok": False,
                            "error": {
                                "code": "E_PREPARE_FAILED",
                                "message": f"pnpm install failed: {result.stderr[:500]}",
                            },
                        }
                    )
                    continue
                steps.append({"name": "pnpm-install", "ok": True})
            elif os.path.isdir(node_modules):
                steps.append({"name": "pnpm-install", "ok": True, "output": {"reason": "node_modules exists"}})
            elif dry_run:
                steps.append({"name": "pnpm-install", "ok": True, "output": {"reason": "dry-run; node_modules would be installed"}})

            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {"steps": steps},
                }
            )

        # ---- launch.plan ----
        elif op == "launch.plan":
            config = inp.get("config", {}) or {}
            # Config from config.mutate uses nested structure.
            services_config = config.get("services", {}) or {}
            backend_cfg = services_config.get("backend", {}) or {}
            vite_cfg = services_config.get("vite", {}) or {}
            webchat_cfg = config.get("webchat", {}) or {}
            backend_port = backend_cfg.get("port", 8092)
            vite_port = vite_cfg.get("port", 5174)
            env_config = config.get("env", {}) or {}
            backend_origin = env_config.get("VITE_BACKEND_ORIGIN", f"http://127.0.0.1:{backend_port}")
            dry_run = ctx.get("dry_run", False)

            app_root = webchat_root(repo_root)
            bin_path = os.path.join(app_root, "bin", "web-chat")
            data_dir = os.path.join(app_root, "var", "devctl")
            web_dir = os.path.join(app_root, "web")
            profile = str(webchat_cfg.get("profile", "") or "").strip()
            profile_registries = str(webchat_cfg.get("profile_registries", "") or "").strip()
            trace_level = str(webchat_cfg.get("trace_level", "off") or "off").strip()
            root = str(webchat_cfg.get("root", "/") or "/").strip()
            debug_api = bool(webchat_cfg.get("debug_api", True))

            if not dry_run:
                os.makedirs(data_dir, exist_ok=True)

            backend_args = [
                bin_path,
                "web-chat",
                "--addr",
                f":{backend_port}",
                "--root",
                root,
                "--timeline-db",
                os.path.join(data_dir, "timeline.sqlite"),
                "--turns-db",
                os.path.join(data_dir, "turns.sqlite"),
                "--geppetto-trace-level",
                trace_level,
            ]
            if debug_api:
                backend_args.append("--debug-api")
            if profile:
                backend_args.extend(["--profile", profile])
            if profile_registries:
                backend_args.extend(["--profile-registries", profile_registries])

            backend_cmd = f"mkdir -p {shlex.quote(data_dir)} && exec {shell_join(backend_args)}"

            log(
                "plan: "
                + f"backend=:{backend_port} vite=:{vite_port} data_dir={data_dir} "
                + f"profile={profile or '(config/default)'}"
            )

            services = [
                {
                    "name": "backend",
                    "command": ["bash", "--noprofile", "--norc", "-lc", backend_cmd],
                    "health": {
                        "type": "http",
                        "url": f"http://127.0.0.1:{backend_port}/api/chat/profiles",
                        "timeout_ms": 15000,
                    },
                },
                {
                    "name": "vite",
                    "cwd": relpath(web_dir, repo_root),
                    "command": ["bash", "--noprofile", "--norc", "-lc", f"exec npx vite --port {vite_port} --clearScreen false"],
                    "env": {
                        "VITE_BACKEND_ORIGIN": backend_origin,
                    },
                    "health": {
                        "type": "http",
                        "url": f"http://127.0.0.1:{vite_port}/",
                        "timeout_ms": 20000,
                    },
                },
            ]

            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {"services": services},
                }
            )

        # ---- command.run ----
        elif op == "command.run":
            cmd_name = inp.get("name", "") or inp.get("command", "")
            dry_run = ctx.get("dry_run", False)
            app_root = webchat_root(repo_root)
            mod_root = go_module_root(app_root)

            if cmd_name == "build-frontend":
                web_dir = os.path.join(app_root, "web")
                if dry_run:
                    log("dry-run: would build frontend")
                    emit({"type": "response", "request_id": rid, "ok": True, "output": {"exit_code": 0}})
                    continue
                log("building frontend...")
                result = subprocess.run(
                    ["npx", "vite", "build"],
                    cwd=web_dir,
                    capture_output=True,
                    text=True,
                    timeout=120,
                )
                if result.returncode != 0:
                    log(f"frontend build failed: {result.stderr[:500]}")
                emit(
                    {
                        "type": "response",
                        "request_id": rid,
                        "ok": True,
                        "output": {"exit_code": result.returncode},
                    }
                )

            elif cmd_name == "build-backend":
                bin_path = os.path.join(app_root, "bin", "web-chat")
                if dry_run:
                    log("dry-run: would build backend")
                    emit({"type": "response", "request_id": rid, "ok": True, "output": {"exit_code": 0}})
                    continue
                os.makedirs(os.path.dirname(bin_path), exist_ok=True)
                log(f"building Go binary -> {bin_path} (from {mod_root})")
                result = subprocess.run(
                    ["go", "build", "-o", bin_path, "./cmd/web-chat"],
                    cwd=mod_root,
                    capture_output=True,
                    text=True,
                    timeout=120,
                )
                if result.returncode != 0:
                    log(f"go build failed: {result.stderr[:500]}")
                emit(
                    {
                        "type": "response",
                        "request_id": rid,
                        "ok": True,
                        "output": {"exit_code": result.returncode},
                    }
                )

            else:
                emit(
                    {
                        "type": "response",
                        "request_id": rid,
                        "ok": False,
                        "error": {"code": "E_UNSUPPORTED", "message": f"unknown command: {cmd_name}"},
                    }
                )

        else:
            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": False,
                    "error": {"code": "E_UNSUPPORTED", "message": f"unsupported op: {op}"},
                }
            )

    except Exception as e:
        log(f"error handling {op}: {e}")
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": False,
                "error": {"code": "E_PLUGIN", "message": str(e)},
            }
        )
