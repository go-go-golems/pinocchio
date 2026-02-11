# Changelog

## 2026-02-11

- Initial workspace created


## 2026-02-11

Created extraction ticket and authored analysis for moving generic chat persistence stores out of pkg/webchat into a shared package.

### Related Files

- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/ttmp/2026/02/11/PIN-20260211-EXTRACT-PERSISTENCE-PKG--extract-persistence-stores-from-webchat-into-shared-package/analysis/01-analysis-extract-chat-persistence-stores-from-pkg-webchat.md — Initial extraction analysis with package options and migration strategy


## 2026-02-11

Implemented hard-cut persistence extraction to pkg/persistence/chatstore (no compatibility shims), updated webchat/web-chat imports, and validated affected pinocchio packages.

### Related Files

- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/cmd/web-chat/timeline/db.go — Timeline tooling migrated to chatstore DSN/store APIs
- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/pkg/persistence/chatstore/timeline_store.go — New shared store package root interface
- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/pkg/persistence/chatstore/turn_store_sqlite_test.go — Added turn store sqlite tests in extracted package
- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/pkg/webchat/router.go — Consumer updated to new chatstore constructors and types
- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/ttmp/2026/02/11/PIN-20260211-EXTRACT-PERSISTENCE-PKG--extract-persistence-stores-from-webchat-into-shared-package/tasks.md — All extraction tasks completed


## 2026-02-11

Verified web-chat-example has no Go module or Go sources in this workspace snapshot (docs-only), so no compile-time API migration was required there.

### Related Files

- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/web-chat-example/pkg/docs/debug-ui-storybook-widget-playbook.md — Only file present in web-chat-example during verification


## 2026-02-11

Added diary coverage and corrected downstream validation target to web-agent-example; confirmed it compiles/tests after hard-cut extraction.

### Related Files

- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/ttmp/2026/02/11/PIN-20260211-EXTRACT-PERSISTENCE-PKG--extract-persistence-stores-from-webchat-into-shared-package/diary/01-diary.md — Detailed implementation diary and correction log
- /home/manuel/workspaces/2025-10-30/implement-openai-responses-api/pinocchio/ttmp/2026/02/11/PIN-20260211-EXTRACT-PERSISTENCE-PKG--extract-persistence-stores-from-webchat-into-shared-package/tasks.md — Task 6 renamed to web-agent-example and remained complete

