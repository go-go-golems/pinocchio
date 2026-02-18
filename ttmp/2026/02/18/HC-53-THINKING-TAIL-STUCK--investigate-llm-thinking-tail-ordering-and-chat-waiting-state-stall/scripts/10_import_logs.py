#!/usr/bin/env python3
import argparse
import json
import re
import shlex
import sqlite3
from pathlib import Path
from typing import Any

import yaml


LOG_RE = re.compile(
    r"^(?P<ts>\S+)\s+(?P<level>[A-Z]{3})\s+(?P<module>\S+)\s+>\s+(?P<msg>.*)$"
)


def parse_message_and_kv(msg: str) -> tuple[str, dict[str, str]]:
    text = msg.strip()
    if not text:
        return "", {}
    try:
        tokens = shlex.split(text)
    except ValueError:
        # Keep raw message if shell-like parsing fails.
        return text, {}

    message_tokens: list[str] = []
    kv: dict[str, str] = {}
    seen_kv = False
    for token in tokens:
        if "=" in token and not token.startswith("="):
            key, value = token.split("=", 1)
            if key:
                kv[key] = value
                seen_kv = True
                continue
        if not seen_kv:
            message_tokens.append(token)
    return " ".join(message_tokens).strip(), kv


def as_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def parse_malformed_event_yaml(yaml_path: Path) -> tuple[str | None, list[dict[str, Any]]]:
    lines = yaml_path.read_text(encoding="utf-8").splitlines()
    conv_id: str | None = None
    entries: list[dict[str, Any]] = []
    current: dict[str, Any] | None = None
    in_message = False
    message_indent = -1

    def finish_current() -> None:
        nonlocal current
        if current is not None:
            entries.append(current)
            current = None

    for line in lines:
        stripped = line.strip()
        indent = len(line) - len(line.lstrip(" "))
        if conv_id is None and stripped.startswith("conversationId:"):
            conv_id = stripped.split(":", 1)[1].strip().strip('"')
            continue
        if line.startswith("  - id: "):
            finish_current()
            current = {
                "id": line.split(":", 1)[1].strip().strip('"'),
                "timestamp": None,
                "eventType": None,
                "eventId": None,
                "family": None,
                "summary": None,
                "sem": None,
                "seq": None,
                "stream_id": None,
                "message_role": None,
                "message_streaming": None,
                "message_content_len": None,
                "rawPayload": {},
                "event_data": {},
                "event_metadata": {},
            }
            in_message = False
            message_indent = -1
            continue
        if current is None:
            continue

        if in_message and indent <= message_indent and stripped != "message:":
            in_message = False
            message_indent = -1

        if stripped == "message:":
            in_message = True
            message_indent = indent
            continue

        def value_after_colon(s: str) -> str:
            return s.split(":", 1)[1].strip().strip('"')

        if stripped.startswith("timestamp:"):
            try:
                current["timestamp"] = int(value_after_colon(stripped))
            except ValueError:
                pass
            continue
        if stripped.startswith("eventType:"):
            current["eventType"] = value_after_colon(stripped)
            continue
        if stripped.startswith("eventId:"):
            current["eventId"] = value_after_colon(stripped)
            continue
        if stripped.startswith("family:"):
            current["family"] = value_after_colon(stripped)
            continue
        if stripped.startswith("summary:"):
            current["summary"] = value_after_colon(stripped)
            continue
        if stripped.startswith("sem:"):
            v = value_after_colon(stripped).lower()
            if v in ("true", "false"):
                current["sem"] = 1 if v == "true" else 0
            continue
        if stripped.startswith("seq:"):
            current["seq"] = value_after_colon(stripped)
            continue
        if stripped.startswith("stream_id:"):
            raw = value_after_colon(stripped)
            current["stream_id"] = None if raw == "null" else raw
            continue

        if in_message:
            if stripped.startswith("role:"):
                current["message_role"] = value_after_colon(stripped)
                continue
            if stripped.startswith("streaming:"):
                v = value_after_colon(stripped).lower()
                if v in ("true", "false"):
                    current["message_streaming"] = 1 if v == "true" else 0
                continue
            if stripped.startswith("content:"):
                raw = value_after_colon(stripped)
                if raw and raw != "|":
                    current["message_content_len"] = len(raw)
                continue

    finish_current()
    return conv_id, entries


def import_event_yaml(conn: sqlite3.Connection, yaml_path: Path) -> None:
    doc = None
    try:
        with yaml_path.open("r", encoding="utf-8") as f:
            doc = yaml.safe_load(f)
    except yaml.YAMLError:
        doc = None

    if isinstance(doc, dict):
        conv_id = doc.get("conversationId")
        entries = doc.get("entries") or []
        if not isinstance(entries, list):
            raise ValueError("YAML entries must be a list")
        parsed_entries: list[dict[str, Any]] = []
        for entry in entries:
            if isinstance(entry, dict):
                parsed_entries.append(entry)
    else:
        conv_id, parsed_entries = parse_malformed_event_yaml(yaml_path)

    rows = []
    for idx, entry in enumerate(parsed_entries, start=1):
        if not isinstance(entry, dict):
            continue
        raw_payload = entry.get("rawPayload") if isinstance(entry.get("rawPayload"), dict) else {}
        event = raw_payload.get("event") if isinstance(raw_payload.get("event"), dict) else {}
        data = event.get("data") if isinstance(event.get("data"), dict) else {}
        metadata = event.get("metadata") if isinstance(event.get("metadata"), dict) else {}
        sem = entry.get("sem", raw_payload.get("sem"))
        if isinstance(sem, bool):
            sem_val = 1 if sem else 0
        elif isinstance(sem, int):
            sem_val = sem
        else:
            sem_val = None

        message_role = entry.get("message_role")
        message_streaming = entry.get("message_streaming")
        message_content_len = entry.get("message_content_len")
        if isinstance(data, dict):
            entity = data.get("entity") if isinstance(data.get("entity"), dict) else {}
            message = entity.get("message") if isinstance(entity.get("message"), dict) else {}
            if message_role is None:
                message_role = message.get("role")
            if message_streaming is None and isinstance(message.get("streaming"), bool):
                message_streaming = 1 if message.get("streaming") else 0
            if message_content_len is None and isinstance(message.get("content"), str):
                message_content_len = len(message.get("content"))

        if not event and (entry.get("eventType") or entry.get("eventId")):
            event = {
                "type": entry.get("eventType"),
                "id": entry.get("eventId"),
                "seq": entry.get("seq"),
                "stream_id": entry.get("stream_id"),
            }
            raw_payload = {"sem": bool(sem_val), "event": event}
            if not data:
                data = entry.get("event_data", {}) if isinstance(entry.get("event_data"), dict) else {}
            if not metadata:
                metadata = entry.get("event_metadata", {}) if isinstance(entry.get("event_metadata"), dict) else {}

        rows.append(
            (
                idx,
                entry.get("id"),
                entry.get("timestamp"),
                entry.get("eventType") or event.get("type"),
                entry.get("eventId") or event.get("id"),
                entry.get("family"),
                entry.get("summary"),
                sem_val,
                entry.get("seq") or event.get("seq"),
                entry.get("stream_id") or event.get("stream_id"),
                conv_id,
                message_role,
                message_streaming,
                message_content_len,
                as_json(raw_payload),
                as_json(data),
                as_json(metadata),
            )
        )

    conn.executemany(
        """
        INSERT INTO event_log_entries(
          idx, entry_id, timestamp_ms, event_type, event_id, family, summary,
          sem, seq, stream_id, conv_id, message_role, message_streaming, message_content_len,
          raw_payload_json, event_data_json, event_metadata_json
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        rows,
    )


def import_gpt_log(conn: sqlite3.Connection, log_path: Path) -> None:
    rows = []
    with log_path.open("r", encoding="utf-8") as f:
        for line_no, line in enumerate(f, start=1):
            raw_line = line.rstrip("\n")
            m = LOG_RE.match(raw_line)
            if not m:
                rows.append((line_no, None, None, None, raw_line, as_json({}), raw_line))
                continue
            msg, kv = parse_message_and_kv(m.group("msg"))
            rows.append(
                (
                    line_no,
                    m.group("ts"),
                    m.group("level"),
                    m.group("module"),
                    msg,
                    as_json(kv),
                    raw_line,
                )
            )

    conn.executemany(
        """
        INSERT INTO gpt_log_lines(
          line_no, ts_iso, level, module_ref, message, kv_json, raw_line
        ) VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        rows,
    )


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Import exported EventViewer YAML and gpt log into sqlite for HC-53 analysis."
    )
    parser.add_argument("--db", required=True, help="SQLite database path")
    parser.add_argument("--schema", required=True, help="SQL schema path")
    parser.add_argument("--event-yaml", required=True, help="Event log YAML path")
    parser.add_argument("--gpt-log", required=True, help="gpt-5.log path")
    args = parser.parse_args()

    db_path = Path(args.db)
    db_path.parent.mkdir(parents=True, exist_ok=True)

    conn = sqlite3.connect(db_path)
    try:
        schema_sql = Path(args.schema).read_text(encoding="utf-8")
        conn.executescript(schema_sql)
        import_event_yaml(conn, Path(args.event_yaml))
        import_gpt_log(conn, Path(args.gpt_log))
        conn.commit()
    finally:
        conn.close()


if __name__ == "__main__":
    main()
