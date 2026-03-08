# Changelog

## 2026-03-08

- Initial workspace created


## 2026-03-08

Recorded alias-shim cleanup commit 2755362, closed the replayable simplify-webchat slices, and added follow-up tasks for deferred router/server cleanup and upstream adaptation.

### Related Files

- /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/08/GP-031--assess-merge-conflicts-between-unify-chat-backend-and-simplify-webchat/reference/01-diary.md — Diary step 16 records alias-shim removal verification and commit 2755362.


## 2026-03-08

Added an os-openai-app-server workspace impact assessment covering nested worktree state, downstream consumer compatibility, simplify-only rollback points, and doc drift before switching the workspace pinocchio checkout to task/unify-chat-backend.

### Related Files

- /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/08/GP-031--assess-merge-conflicts-between-unify-chat-backend-and-simplify-webchat/design/02-os-openai-app-server-workspace-impact-assessment-for-adopting-task-unify-chat-backend.md — Detailed workspace impact assessment for adopting task/unify-chat-backend.


## 2026-03-08

Removed the remaining router/server compatibility helpers from `pkg/webchat`, published a Glazed help migration guide for the cutover, and updated the GP-031 assessment/playbook/tasks to record that the simplify-webchat reconciliation is now complete.

### Related Files

- /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/doc/topics/webchat-compatibility-surface-migration-guide.md — New canonical migration guide for `NewFromRouter`, `RegisterMiddleware`, `Mount`, `Handle`, `HandleFunc`, and `Handler` removal.
- /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/08/GP-031--assess-merge-conflicts-between-unify-chat-backend-and-simplify-webchat/design/01-merge-assessment.md — Updated to record that the compatibility-surface cleanup is now complete on the current branch.
