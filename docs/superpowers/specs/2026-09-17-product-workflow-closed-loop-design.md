# Product Workflow Closed Loop Design

> Date: 2026-09-17  
> Scope: B — editor + blank create + run form + run detail  
> Jobs: sparkops-build / sparkops-deploy / sparkops-approval / freestyle / notify

## Goal

Deliver a KodeRover-style custom pipeline experience on product workflows, backed by the existing `product_workflow` engine and Zadig V4 stage/job model.

## Architecture

- Definition: `ProductWorkflow.stages[]` with jobs `{name,type,spec}`
- Runtime: `ProductWorkflowRun.stage_runs[]` (unchanged persistence)
- Editor UI: vertical stage columns + job cards (not xyflow DAG) for closer KodeRover feel
- APIs: GET/POST/PATCH/DELETE workflow; POST runs with optional `input`; GET run detail page

## Job Specs (P0)

| Type | Spec | Execution |
|------|------|-----------|
| sparkops-build | project_builds[{project_id,branch}] | async build trigger |
| sparkops-deploy | environment_id, source=fromjob, job_name | async deploy |
| sparkops-approval | timeout_minutes? | wait for approve/reject |
| freestyle | script, timeout_minutes? | sync success (log script) |
| notify | channel, message | sync success |

## Routes

- `GET/PATCH/DELETE /api/v1/products/:productId/workflows/:workflowId`
- `POST /api/v1/products/:productId/workflows` (blank create)
- `POST .../runs` body `{ input?: { branch?, environment_overrides? } }`
- Frontend: `/products/workflows`, `/products/[id]/workflows/[wfId]/edit`, `/.../runs/[runId]`

## Out of scope (P1+)

Jenkins/test/scan/blue-green, IM approval, cancel/retry, real freestyle K8s runner.
