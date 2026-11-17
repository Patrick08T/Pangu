# Protected experiments (internal service contract)

This package does not enable learning, publish versions, or provide a public experiment API. Callers must authorize access and supply authoritative live policy, dependency, compiler and environment state. In particular, a hash or a `GatesPassed` boolean from a candidate/client is not proof of sandbox isolation or a trusted checker.

## Preregistration and inference

`FreezeSuite` requires explicit confidence, minimum independent families, repeats, maximum candidates, primary metric, improvement, per-metric observation bounds and non-inferiority tolerances. Acceptance, cost and duration protections are mandatory. A policy adapter must also check that every policy-required metric (including rework) and task group is registered. No production thresholds are supplied here. Unknown cost is absent data, never zero.

The supported method is `hoeffding_bonferroni`, with `invalidate` execution-failure policy. For metric bounds `[L,U]`, oriented paired differences lie in `[-R,R]`, where `R=U-L`. Cases and repeats are averaged **within family**; those family means, not individual retries, are the independent units. With `n` independent families and `K = MaxCandidates * metrics * (1 + registered groups)`, the two-sided Hoeffding radius is `2R * sqrt(log(2K/(1-confidence))/(2n))`. Intervals are clipped to the known difference range. This is a conservative bounded-mean interval, not a t approximation. The independent-family assumption and the bounds must be justified by the independent evaluator before registration.

Primary overall lower bound must meet the registered improvement. Other metric/group lower bounds must meet their registered non-inferiority margins. All pairs and metrics must be present, finite and within bounds. Critical protection, authorization and isolation gates must pass on every repeat of both arms. Failed/timeout/infrastructure-error runs cannot be omitted, replaced by a successful retry, or promoted. Exact receipt retries are idempotent; a changed receipt or attempt number is not another independent sample.

## Trusted integration sequence

1. Independently compile and freeze the suite, source checks, baseline/candidate identities and policy. `CriteriaHash` hashes the JSON criteria array; expected/input hashes hash raw UTF-8 bytes. `CheckerHash` identifies the externally trusted checker implementation. Suite metadata never contains hidden bodies.
2. `CreateExperimentSuite` checks retained source cases against real submission task/run/project/input identities and freezes the suite. Source cleanup is not blocked by historical references.
3. `ReserveExperiment` atomically spends one shared holdout candidate exposure **before** execution or hidden-case loading. Renaming a suite, changing policy, invalidating an experiment or failure does not refund it. The scheduler separately enforces actual call/token resource spending; the resource budget fields here freeze identity, not a resource usage ledger.
4. Trusted execution uses `ReadExperiment`, `ReadExperimentSuite`, and `LoadExperimentCase`. The last function requires an active, already-reserved experiment. Ordinary case/list/manual-feedback APIs do not disclose registered protection/holdout content. The caller must not hand expected answers or checker internals to the candidate.
5. Catalog hashes the actual compiled definition with only runtime `Input` cleared (all project/version/flow fields retained). `ExecutionInputHash` independently binds the actual case input hash, experiment hash, arm and repeat. Dispatch must use and check both identities, a real isolated execution environment, and the immutable candidate—not caller-supplied metadata alone.
6. `RecordExperimentReceipt` checks generation and all frozen identities transactionally, verifies sources still exist and match, and stores an immutable receipt. Invalidation increments the live generation; frozen specs remain immutable, and late old-generation receipts are rejected.
7. `EvaluateStoredExperiment` loads saved receipts and recomputes the verdict. Its `current` argument must be rebuilt by the trusted service from live state. Append-only decision rows bind experiment and receipt-set hashes. Publication must lock the experiment, require active matching generation/hash and the latest passed decision, and independently revalidate live policy, sources, permissions, environment and promotion intent in the same publication transaction.

## Transaction integration

`ReserveExperimentTx`, `ReadExperimentTx`, `ReadExperimentSuiteTx`, `LoadExperimentCaseTx`, `RecordExperimentReceiptTx`, and `EvaluateStoredExperimentTx` take the caller's `pgx.Tx`; standalone wrappers remain available. Hold live policy/intent locks first, then experiment locks. Roll back on error. Do not call a standalone locking wrapper while already holding its experiment lock in another transaction.

Commit reservation **before** hidden-case loading or dispatch. `LoadExperimentCaseTx` verifies committed visibility using one non-locking read on a separate connection, so neither an outer transaction nor a released nested savepoint can disclose answers and subsequently refund the exposure by rolling back. Source checks, checker verification and authoritative locks remain in the caller transaction. This visibility probe is deliberately not another locking transaction.

## Permanent learning-exposure boundary

Suite creation takes `identity.Lock(namespace)` before all case/source/pool row locks, sharing learning's `identity → experience → source` ordering. Previously granted/read learning sources cannot later become protection or holdout data. The permanent `learning_source_exposures` ledger is checked by run, task and raw UTF-8 input hash at creation and every subsequent source revalidation, including existing experiments after historical backfill. Withdrawal, expiry, renamed suites/cases and copied inputs do not restore unseen status. An unresolved exposure or missing historical task/input identity fails closed for every hidden suite in that project. Development-only use remains allowed. A missing ledger/schema is an error, never permission to skip the check.

Evaluation database tests directly persist retained submission rows (with the real task-lineage trigger) and the trusted consent projection to exercise classification and retention behavior; the learning package separately verifies actual `GrantSource` authorization and projection creation. Neither is evidence of real-model learning benefit.

## Disclosure and limitations

Decision JSON exposes only coarse outcome/category and its identity hash, not hidden case IDs, observations, intervals, errors or expected answers. Detailed in-process intervals are for the independent evaluator only. Case bodies remain in the existing retention-controlled store and are not copied into immutable experiment tables. Sources are rechecked on use: expiry, archival, body/assertion changes or source payload purge invalidate access.

These tests establish deterministic control behavior and database atomicity, not real-model learning benefit. Isolation, trusted checking, pricing, runtime resource accounting, compiler execution, publication, observation and rollback must be verified by their respective integration layers. There is no implicit activation or deployment in this package.
