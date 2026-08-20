# Changelog

## 2026-08-20

- Initial workspace created


## 2026-08-20

Completed the restart-safe message identity research package: verified the restart collision, mapped identity propagation, specified the injectable UUIDv4 generator design, documented phased implementation and tests, and recorded the investigation diary.

### Related Files

- pkg/chatapp/chat.go — Identified the process-local nextID allocator to replace
- pkg/chatapp/projections.go — Documented entity-key overwrite behavior
- pkg/chatapp/runtime_inference.go — Mapped root identity propagation into runtime correlation
- pkg/chatapp/service.go — Identified UUID generation precedent and service configuration boundary


## 2026-08-20

Validated the ticket with docmgr doctor and uploaded the design guide plus investigation diary as a single PDF bundle to reMarkable at /ai/2026/08/20/PINOCCHIO-202.


## 2026-08-20

Step 4: replaced the process-local root message counter with validated, injectable UUIDv4 allocation; added deterministic test injection and restart/failure regressions (commit 6f4e946).

### Related Files

- /home/manuel/workspaces/2026-08-12/deploy-dev-indexer/pinocchio/pkg/chatapp/message_id.go — Restart-safe root identity implementation
- /home/manuel/workspaces/2026-08-12/deploy-dev-indexer/pinocchio/pkg/chatapp/message_id_test.go — Identity and atomic-failure regressions

