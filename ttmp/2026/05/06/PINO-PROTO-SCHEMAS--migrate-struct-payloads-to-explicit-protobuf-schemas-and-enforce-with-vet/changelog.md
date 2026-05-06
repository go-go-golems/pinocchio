# Changelog

## 2026-05-06

- Initial workspace created


## 2026-05-06

Created ticket and intern-oriented design guide for migrating remaining Struct payloads in Pinocchio/CoinVault to explicit protobuf schemas and enforcing the rule with a go/analysis vet tool.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/06/PINO-PROTO-SCHEMAS--migrate-struct-payloads-to-explicit-protobuf-schemas-and-enforce-with-vet/design/01-explicit-protobuf-payloads-and-vet-enforcement.md — Primary design and implementation guide
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/06/PINO-PROTO-SCHEMAS--migrate-struct-payloads-to-explicit-protobuf-schemas-and-enforce-with-vet/reference/01-implementation-diary.md — Chronological context for the ticket


## 2026-05-06

Uploaded the design guide, implementation diary, and tasks bundle to reMarkable at /ai/2026/05/06/PINO-PROTO-SCHEMAS.


## 2026-05-06

Updated the CoinVault design: use separate protobuf messages and separate event/UI/timeline names per widget instead of a single oneof wrapper; no backwards compatibility shims for old Struct payloads.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/06/PINO-PROTO-SCHEMAS--migrate-struct-payloads-to-explicit-protobuf-schemas-and-enforce-with-vet/design/01-explicit-protobuf-payloads-and-vet-enforcement.md — Revised CoinVault target architecture
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/06/PINO-PROTO-SCHEMAS--migrate-struct-payloads-to-explicit-protobuf-schemas-and-enforce-with-vet/tasks.md — Updated CoinVault task wording

