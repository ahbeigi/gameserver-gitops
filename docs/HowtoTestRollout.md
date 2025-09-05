# How to Test a **NoDisruption** Rollout on EKS

This guide shows how to trigger and observe a zero-disruption rollout by changing `spec.parameters.maxPlayers` from **32 → 40**.

> Assumptions:
> - Operator is deployed and healthy in your EKS cluster.
> - A working game image is deployed (e.g., an ECR tag) and a `GSDeployment` named `shooter-fleet` already exists.

---

## 0) Prep & sanity checks

Set your namespace and confirm resources are present.
```
kubectl get ns games
kubectl -n games get gsd,gs,pods
```

You should see:
- One `GSDeployment` (`shooter-fleet`)
- A few `GameServer` children (≥ `minReplicas`)
- Pods in `Running` (eventually Ready once `/status` is reachable)

---

## 1) Trigger the rollout by editing `maxPlayers`

Open the resource in your editor:
```
kubectl -n games edit gsdeployment.game.example.com/shooter-fleet
```

Find (or add) this section and set to **40**:
```
spec:
    parameters:
        maxPlayers: 40
```

Save & exit. This updates the desired spec.

What the controller does (PoC behavior):
- Marks **existing** `GameServer` as **draining** (annotation `game.example.com/draining: "true"`), so the allocator shouldn’t place new matches there.
- Creates up to **`updateStrategy.maxSurge`** new servers with the updated env (`MAX_PLAYERS=40`).
- As draining servers become **idle** (`players == 0`), they’re **deleted** immediately.
- Non-draining idle servers still respect `scaleDownZeroSeconds`.


## 2) Watch the rollout

Tail the fleet:
```
kubectl -n games get gs,pods
```

Inspect draining flags and env values:
```
kubectl -n games get gs -o json | jq -r '.items[] | {name:.metadata.name, draining:.metadata.annotations["game.example.com/draining"], mp:(.spec.env[]? | select(.name=="MAX_PLAYERS") | .value)}'
```

(You should see old servers with `DRAINING=true` and `MAX_PLAYERS=32`, and new ones with `MAX_PLAYERS=40`.)

![Alt text](images/rollout.png?raw=true)


## 3) Simulate active matches (optional, to prove no disruption)

Pick an **old** (draining) GameServer name (from the table above), then simulate players so it won’t be deleted:

    OLD=<put-an-old-gs-name-here>
    kubectl -n games patch gameserver $OLD --subresource=status --type merge -p '{"status":{"players":5,"maxPlayers":32}}'

Observe that it **stays** while players > 0. When you drop players back to 0, it should be deleted promptly:

    kubectl -n games patch gameserver $OLD --subresource=status --type merge -p '{"status":{"players":0}}'

