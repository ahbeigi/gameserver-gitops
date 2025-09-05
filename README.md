# GitOps for Game Deployments

This document describes a GitOps workflow for rolling out game server binaries and configuration. The workflow aims to support:
- Promotion across environments.
- Multi region deployment.
- Avoid disrupting active sessions.
- Rollback.


## 1) Overview
### Controllers/CRDs ([gameserver-operator repo](https://github.com/ahbeigi/gameserver-operator))
- `GameServer` and `GSDeployment` run in each cluster.
- Key fields in `GSDeployment.spec`:
  - `image`, `portRange`, `minReplicas`, `maxReplicas`, `scaleUpThresholdPercent`, `scaleDownZeroSeconds`.
  - `updateStrategy`: custom rollout policy implemented by the controller:
    - `type: NoDisruption`
    - `drainTimeoutSeconds`
    - `maxSurge`, `maxUnavailable`
  - `parameters`:
    - `maxPlayers`

### Repos in this codebase
For brevity, three repositories have been wrapped in the current codebase (as gameserver-gitops). However, in practice they should live in separate repositories. 
1. **`infra-bootstrap`** - Bootstrap & control plane deployment
- Kustomize to install **Argo CD** (namespace + install), an **AppProject**, and a **root Application** that points to `game-envs/apps/` (where the ApplicationSet lives).
- Applied once to **management cluster** to deploy Argo CD and connect it to the config repo. (See [infra-bootstrap/README.md](infra-bootstrap/README.md) for how to deploy.)

2. **`game-binary`** - Game server source & image build
- Contains the game server code, **Dockerfile**, and CI workflow that builds and pushes immutable images to **ECR** (e.g., `:sha-<short>` and `:dev-latest`).
- The CI can also open a PR to `game-envs` to bump the **dev** overlay’s `spec.image`.

3. **`game-envs`** - Single source of truth for deployments
- Kustomize structure by **service : base/overlays**; each overlay pins the exact `spec.image` and `spec.parameters` (e.g., `maxPlayers`) for a specific **env/region**.
- Includes `apps/application-set.yaml` that generates one Argo CD **Application** per labeled cluster (`env`, `app`, `region`, `wave`).
- **Edit flow:** CI opens PRs for dev bumps; a **Promote** workflow opens PRs to **staging/prod**; Argo CD detects merges and **syncs** the matching Applications.


> See [docs/HowtoTestRollout.md](docs/HowtoTestRollout.md) for how update strategy can be tested.


## 2) Build workflow: Game server binary updates

Flow:
1. Developers update the code in `game-binary`, and merge into the main branch. CI builds and pushes image `.../game-binary:sha-<short>`.
2. Argo CD's Image Updater opens a PR in `game-envs` (dev overlay) to bump:
```
spec:
  image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/game-binary:sha-abc123
```
3. Merge to main: Argo CD syncs the application in dev environment.
4. `GSDeployment` Controller executes `updateStrategy: NoDisruption`, which means:
- Mark old servers **Draining** (so the allocator service won't schedule any new matches on those servers).
- Keep them alive until `players == 0` or `drainTimeoutSeconds` elapses (then safely terminates them).
- Create replacement servers (respect `maxSurge`, `maxUnavailable` and port ranges).

> Notes:
> - We never kill servers with active players.
> - Allocator must exclude **Draining** servers.
> - Keep both images alive during transition.


## 3) Deploy workflow: Environment promotion (dev -> staging -> prod)
- Each env/region overlay pins exact `image` and `parameters`.
- Promotion is a PR that copies values forward (preferably gated by soak time and SLO checks). See [promote.yml](game-envs/.github/workflows/promote.yml) as a PoC.
- Use CODEOWNERS for approvals and Argo CD sync windows for safe rollout hours.

Example promotion diff
```diff
- image: .../game-binary:sha-abc123
+ image: .../game-binary:sha-b19c0de
- parameters: { maxPlayers: 28 }
+ parameters: { maxPlayers: 32 }
```


## 4) Rollback capability
- Git-first: `git revert` the commit in `game-envs` that changed `image` and/or `parameters`.
- Argo sync applies the revert; controller stops creating "bad" servers and deploys "good" ones.
- Limit blast radius with small `maxSurge`, `maxUnavailable: 0`, and region waves (deploy on regions with fewer users first).


## 5) Multi-region deployments (coordinated rollouts)
- One ApplicationSet renders an Argo CD `Application` per `env x region` using cluster labels (e.g., `env=prod`, `app=shooter`, `region=us-east-1`, `wave=10`).
- Use sync waves to roll out by region batches; gate later waves on health from earlier waves.

> See `game-envs/apps/` for an example of ApplicationSet. 
