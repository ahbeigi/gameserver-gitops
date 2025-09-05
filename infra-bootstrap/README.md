# infra-bootstrap
## How to use

### 1)Bootstrap Argo CD
```
kubectl apply -k argocd/
kubectl apply -f projects/shooter-project.yaml
kubectl apply -f apps/root-app.yaml
kubectl apply -f apps/gameserver-operator-app.yaml
```

### 2) Register clusters and label them
> This is a hub-spoke model for Argo CD, which assumes the Argo CD is installed on a management (hub) cluster, and can reach all spoke clusters (Dev, Staging, Prod).

```
argocd cluster add dev-us-east-1 --label env=dev,app=shooter,region=us-east-1,wave=10
argocd cluster add stg-us-east-1 --label env=staging,app=shooter,region=us-east-1,wave=20
argocd cluster add prd-us-east-1 --label env=prod,app=shooter,region=us-east-1,wave=30
```