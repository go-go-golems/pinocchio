# HC-53 SQLite Analysis Scripts

## Inputs

- `/tmp/gpt-5.log`
- `~/Downloads/event-log-6ac68635-37bc-4f4d-9657-dfc96df3c5c6-20260218-222305.yaml`

## Build database

```bash
./scripts/11_build_db.sh
```

Database output:

- `sources/analysis.db`

## Run queries

```bash
./scripts/20_run_queries.sh
```

Query outputs:

- `sources/query-results/q01_event_type_counts.txt`
- `sources/query-results/q02_thinking_event_timeline.txt`
- `sources/query-results/q03_post_final_thinking_delta.txt`
- `sources/query-results/q04_timeline_thinking_streaming_state.txt`
- `sources/query-results/q05_gpt_thinking_markers.txt`
- `sources/query-results/q06_events_after_last_thinking_final.txt`
