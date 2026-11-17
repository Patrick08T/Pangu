# Evaluator integration handoff (implementation in progress)

Task 13 writer: independent child be69aeb1-59d7-4b78-b4ff-b55acca156cf; Lead claimed shared task revision 2 on its behalf. Only new `evolution*.go` and tests belong to that writer. Lead owns existing worker registration, maintenance and runtime integration.

The Team tool cannot address this independent child (`active teammate ... not found`). This shared-file note records coordinator decisions rather than implying a message was delivered.

## Approved runtime schema

Migration `../database/migrations/0063_evolution_runtime.sql` is complete and frozen. Lead is applying/checking it only against the disposable database; do not edit it. It provides the four requested tables and a monotonically incrementing `github_hooks.revision`. Plans permit body erasure only (`plan=NULL, erased_at=...`); digest/identity remain immutable. Runtime/node outputs are protected private bodies, not ordinary API responses. Cleanup must erase all these bodies with the corresponding retained submission; never claim backup erasure.

## Service contracts

`Service.PrepareCase(ctx, Scope, CaseKey{IntentID,Generation,Arm,CaseID,Repeat}) (*ExecutionPlan,error)` returns persistent RequestID. Definition and Memories alone may reach model inputs; Experiment/Expected/Criteria must not. `RecordCaseResult(ctx,scope,key,requestID)` invokes trusted EvidenceReader. MachineResult now requires actual Cost/Currency/CostMeasured/TokensMeasured/DurationMeasured, plus all previously specified independently measured hashes and StopConfirmed. Unknown prices/usage cannot become zero. Existing `usage_calls` records frozen prices and `estimate_micros`.

Catalog owner is implementing `CandidateRequest.Kind` and explicit single `BenefitMemoryVersionID`: memory_benefit uses identical role/flow on both arms, baseline no memory, candidate exact one memory. Role change remains independently compiled. Owner knows terminal Prepare replay must not re-execute completed calls.

API expects optional `Backend.StartEvolution(ctx, scope evolution.Scope, req evolution.CandidateRequest) (*evolution.Intent,error)`. API reconstructs candidate from approved baseline plus Instruction; it does not accept raw CandidatePayload or Passed. Until real driver exists Reserve only reports preparing, never claimed execution.

## Protected ordinary routes

RunSummary.Purpose is server-only (`json:"-"`); GetRun uses trusted submission purpose, private history persists `internal_purpose` and restores it. API blocks nonbusiness run detail and all derived routes, request bodies, approval detail/actions, and filters lists. Evaluator submissions must persist purpose=evaluation. Do not leak case input/output through a new ordinary surface.

## Entry and observation adapters

EntryAdapter ReadTx always returns authoritative revision/owner/enabled and RemoteState confirmed|pending|unknown. CompareAndSwapTx only changes local state plus durable remote intent inside the same transaction; no remote I/O. Normalize known permission/conflict/notfound errors to evolution.ErrForbidden/ErrConflict/ErrNotFound. Keys already include intentID:generation:entryKey:direction. Scope.ActorID is frozen policy actor, never target owner. Existing generic schedule reconciliation must not race evolution-owned pending targets; Lead wires routing/exclusion.

Business EvidenceReader must establish candidate execution after adoption, matching entry/version, independent and comparable baseline task/input family, actual authoritative feedback/source proof and measured metrics. Terminal business success is not itself a quality score. Missing or incomparable evidence fails closed. Synthetic boundary readers in service tests are not evidence of real improvements.

## Integration review blockers (Lead)

- Do not assign measured CompiledHash/RoleHash/MemoryHash from the expected arm. Independently compute actual Definition hash and canonical actual memory refs after validating each actual body hash. Role identity must be coupled to actual compiled artifact/role; copying arm expected values does not establish measurement. Catalog owner is also checking applicability for every actual injected flow node; do not silently filter a frozen combination.
- Snapshot now includes server-only EntryKey/EntryRevision; task15 SubmitEntry freezes workbench:<bare-id> from adopted binding, operation=entry, purpose=business. Explicit version submissions lack provenance and must not count as default-entry observation.
- Prepublication publication=nil reconciling must use Service.CancelEvaluation, not rollout.Reconcile. Missing runtime pgx.ErrNoRows is unknown, preserves claims; arbitrary SQL errors propagate.
- Full architecture currently flags TestEvolutionEntryCASPreservesUserEditsAndPersistsRemoteIntent at 81 lines and TestEvolutionRemoteReceiptCannotOverridePausedEntry at 85 lines (>70). Split fixture/helpers without weakening behavior.
- Task20 now owns only internal/schedule/remote.go + remote_evolution*.go: ordinary reconciliation shares session schedule-remote:<id> mutex, short pre/post CAS transactions and skips exact current evolution outbox ownership. Lead will review integrated protocol.
- Task21 is assigned to evolution-rollout: cached observation source hashes must be re-read through trusted Evidence.ReadObservationTx before assessing/stable promotion; a feedback edit, retention expiry or revoked actor cannot leave historical samples authoritative.
- Lead review still sees expected-arm hashes assigned at evolution_evidence.go:57-59 and prepublication reconciling routed to rolloutReconcile; these are completion blockers, not waived because execution tests pass.

## Shared verification environment

Use the existing disposable PostgreSQL on 55439 only. No additional service or original DB is needed. Go is `C:\Users\pengtaotao\Downloads\Pangu\.tools\go\bin\go.exe`. From the repository root run `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; . ./.artifacts/experience-implementation-20261111/test-env.ps1` before Go. The script sets the private test URL and workspace-local Go caches/temp; do not print credentials. The long default fixture TMP can trigger Git's `$GIT_DIR too big`; use a shorter workspace-local temporary directory for Git-heavy suites. Snapshot EntryKey and EntryRevision are already committed to the shared working tree (not git-committed). Lead still owns cron exclusion and worker/maintenance wiring; unsupported rework/hard-failure observations must fail closed.

