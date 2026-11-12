# Backend deployment-completion subscriber report

## Scope

- Worktree: `A:\21-Agent\SparkOpsAgent\.worktrees\unified-file-infrastructure`
- Cluster: 8 only
- Baseline commit: `64886e2`
- Target tests:
  - `TestOnDeployCompletedClearsCurrentVersionOnUndeploy`
  - `TestOnDeployCompletedDoesNotOverwriteNewerDeploymentState`

## Root-cause evidence

The initial exact reproduction was:

```text
go test ./internal/modules/product/application/subscriber -run '^(TestOnDeployCompletedClearsCurrentVersionOnUndeploy|TestOnDeployCompletedDoesNotOverwriteNewerDeploymentState)$' -count=1
```

It failed because the undeploy test recorded no save, while the older-event test recorded a save.

Tracing the data flow showed:

1. `NewDeployCompleted` copies the supplied `ProjectID`, `Status`, `RunType`, and creates `BaseEvent.At` through `events.NewBaseEvent`, so `OccurredAt()` is the event creation time.
2. Both events use `Status == "succeeded"` and a non-nil `EnvironmentID`, so the handler's initial gates pass.
3. The undeploy event uses `RunType == "undeploy"`, and the older-event case uses a normal run type.
4. The production repository implementation queries by both `environment_id` and `project_id`, returning `ErrEnvironmentProjectNotFound` when that pair is absent.
5. The test fake intentionally models that same contract, but both existing rows omitted `EnvironmentProject.ProjectID`. Consequently:
   - undeploy lookup returned not-found, and the handler correctly returned without saving;
   - the old normal deployment lookup returned not-found, and the handler correctly took its missing-row upsert path, producing the observed save.
6. Git history confirms the timestamp guard was added in `bc2891b` and undeploy clearing was added in `d87a51b`; the current handler contains both behaviors. No production defect was found after correcting the fixture identity.

## TDD RED/GREEN

RED was observed before the fix with the exact command above:

```text
FAIL: TestOnDeployCompletedClearsCurrentVersionOnUndeploy
      Expected value not to be nil (repo.saved)
FAIL: TestOnDeployCompletedDoesNotOverwriteNewerDeploymentState
      Expected nil, but got a saved EnvironmentProject
```

The minimal fix was to use the same generated `projectID` in each test event and persisted fake row. No production code change was warranted by the evidence.

GREEN exact command:

```text
go test ./internal/modules/product/application/subscriber -run '^(TestOnDeployCompletedClearsCurrentVersionOnUndeploy|TestOnDeployCompletedDoesNotOverwriteNewerDeploymentState)$' -count=1
```

Result: `ok`, exit code 0.

GREEN package command:

```text
go test ./internal/modules/product/application/subscriber -count=1
```

Result: `ok`, exit code 0.

Additional checks:

- `git diff --check`: passed.
- `ReadLints` for the changed test: no linter errors.

## Changes

- Updated only `backend/internal/modules/product/application/subscriber/on_deploy_completed_test.go`.
- Added matching `ProjectID` values to the two fake repository rows and their events.
- No unrelated clusters or production code were changed.

## Commit

Included in the atomic commit reported by `git log -1`.

## Concerns

- The requested behavior is satisfied by the existing production handler once the test fixture uses the repository's required composite identity.
- There is no evidence supporting a production change; adding one would risk weakening the real `(EnvironmentID, ProjectID)` lookup contract.
