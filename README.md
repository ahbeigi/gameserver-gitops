# GitOps for Game Deployments

This document describes a GitOps workflow for rolling out game server binaries and small configuration knobs safely across environments and regions—without disrupting active matches—and with fast rollback.

## 1) Architecture (what lives where)

- Controllers/CRDs (operator repo)
  - `GameServer` and `GSDeployment` run in each cluster.
  - Key fields in `GSDeployment.spec`:
    - `image`: container image for the server.
    - `portRange`, `minReplicas`, `maxReplicas`, `scaleUpThresholdPercent`, `scaleDownZeroSeconds`.
    - `updateStrategy`: custom rollout policy implemented by your controller:
      - `type: NoDisruption`
      - `drainTimeoutSeconds`
      - `maxSurge`, `maxUnavailable` (int or percent)
    - `parameters`: small inline config, starting with:
      - `maxPlayers` (integer)
- GitOps control plane
  - Argo CD manages desired state; ApplicationSet fans out to clusters by labels.
  - Argo CD Image Updater opens PRs to bump image tags.
- Repos
  1. `infra-bootstrap`: Argo CD install, AppProject, bootstrap apps.
  2. `game-binary`: game server code, Dockerfile, CI → pushes to ECR.
  3. `game-envs`: Kustomize overlays per env/region; pins exact `image` and `parameters`.

Example registry:
```
123456789012.dkr.ecr.us-east-1.amazonaws.com/game-binary:<tag>
```

---

## 2) Game server binary updates (no disruption)

Flow
1. CI builds/pushes image `.../game-binary:sha-<short>` and (optionally) updates a floating tag `:dev-latest`.
2. Image Updater opens a PR in `game-envs` (dev overlay) to bump:
   ```yaml
   spec:
     image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/game-binary:sha-abc123
   ```
3. Merge → Argo CD syncs dev.
4. Controller executes `updateStrategy: NoDisruption`:
   - Mark old servers **Draining** (no new matches).
   - Keep them until `players == 0` or `drainTimeoutSeconds` elapses (then safe terminate).
   - Create replacement servers (respect `maxSurge`/`maxUnavailable` and ports).

Notes
- Never kill servers with active players; allocator must exclude **Draining** servers.
- Keep both images alive during transition; capacity does not dip.

---

## 3) Environment promotion (dev → staging → prod)

- Each env/region overlay pins **exact** `image` and `parameters`.
- Promotion is a PR that copies values forward (optionally gated by soak time and SLO checks).
- Use CODEOWNERS for approvals and Argo CD sync windows for safe hours.

Example promotion diff
```diff
- image: .../game-binary:sha-abc123
+ image: .../game-binary:sha-b19c0de
- parameters: { maxPlayers: 28 }
+ parameters: { maxPlayers: 32 }
```

---

## 4) Rollback capability (fast & clean)

- Git-first: `git revert` the commit in `game-envs` that changed `image` and/or `parameters`.
- Argo sync applies the revert; controller stops creating “bad” servers and seeds “good” ones; draining retires old pods safely.
- Limit blast radius with small `maxSurge`, `maxUnavailable: 0`, and region waves.

---

## 5) Multi-region deployments (coordinated rollouts)

- One ApplicationSet renders an Argo CD `Application` per `env × region` using cluster labels (e.g., `env=prod`, `app=shooter`, `region=us-east-1`, `wave=10`).
- Use sync waves to roll out by region batches; gate later waves on health from earlier waves (PreSync hook or external pipeline).

> See `game-envs/apps/` for an example of ApplicationSet. 


---

## 6) How each requirement is satisfied

1. **Game server binary updates**  
   Image PR → Argo sync → controller performs **NoDisruption** rollout (Draining; `maxSurge`/`maxUnavailable`).
2. **Configuration deployments**  
   Inline `spec.parameters` change → same rollout path; controller injects values into pods (env vars/args).
3. **Environment promotion**  
   PR-based promotion of `image` and `parameters` across overlays with approvals, soak, and sync windows.
4. **Rollback capability**  
   `git revert` + Argo sync; controller seeds old-good servers and drains the rest.
5. **Multi-region deployments**  
   ApplicationSet + cluster labels + sync waves/windows; optional gates on metrics.

---

## 7) Practical bits

- Allocator service:  
  - Picks a **Ready** server, not **Draining**.  
  - Uses `GameServer.status` (e.g., `phase`, `players`, `maxPlayers`) and updates the chosen server atomically.
- Controller pod template:
  - Inject `parameters.maxPlayers` as `env: [{ name: "MAX_PLAYERS", value: "32" }]` (and mirror into `GameServer.status.maxPlayers` for allocator).
  - Readiness on `/status`.
- Observability:  
  - Export Prometheus metrics (active matches per version, drain durations, failures).  
  - Gate promotions on SLOs (crash-free %, latency).
- Access control:  
  - CODEOWNERS restricts staging/prod changes; Argo sync windows for change freeze.
- Cleanup:  
  - Periodically prune unused images and old resources; keep enough history for audit.
- Argo CD tips:  
  - `ApplyOutOfSyncOnly=true` and `ServerSideApply=true` can speed large fleets.

---

### TL;DR rollout playbook

- **Build** → image in registry → **Image Updater PR**  
- **Merge to dev** → Argo sync → **NoDisruption rollout** → soak & checks  
- **Promote** via PR to staging/prod by overlays (waves by region)  
- **Need to undo?** Revert the PR → Argo applies → old version drains back in
