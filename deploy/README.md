# Kubernetes & Argo CD deployment

This directory contains a minimal-but-production-ready setup for building a container image, deploying it to Kubernetes with Kustomize, and having Argo CD keep the cluster in sync with this repo.

## 1. Build and publish an image

```bash
docker build -t ghcr.io/<org>/k2marketing-ai:<tag> .
docker push ghcr.io/<org>/k2marketing-ai:<tag>
```

Update `deploy/k8s/overlays/prod/kustomization.yaml` so the `images` stanza references the registry, repository and tag you pushed.

## 2. Provide `config.json` as a Secret

Create a production config locally (never commit secrets) and turn it into a Kubernetes Secret named `k2marketing-config`:

```bash
kubectl create namespace k2marketing
kubectl -n k2marketing create secret generic k2marketing-config \
  --from-file=config.json=./config.prod.json
```

Whenever the config changes, delete and recreate the secret or use `kubectl create secret ... --dry-run=client -o yaml | kubectl apply -f -`.

The deployment mounts that secret at `/app/config.json`, allowing the binary to load the same JSON structure it expects locally.

## 3. Configure TLS and DNS (optional)

If you plan to expose the service publicly, provision a TLS certificate (for example via cert-manager or manually) and store it as a secret named `k2marketing-tls` in the same namespace. Update `deploy/k8s/base/ingress.yaml` (or patch it from your overlay) with the hostnames you own.

## 4. Deploy with Kustomize

To apply the manifests without Argo CD:

```bash
kustomize build deploy/k8s/overlays/prod | kubectl apply -f -
```

This creates the deployment, service and ingress. Watch rollout status with `kubectl rollout status deploy/k2marketing-api -n k2marketing`.

## 5. Manage with Argo CD

1. Ensure Argo CD has access to the git repository and the image registry.
2. Apply `deploy/argocd/application.yaml` to the `argocd` namespace:

   ```bash
   kubectl apply -f deploy/argocd/application.yaml -n argocd
   ```

3. Argo CD will create the `k2marketing` namespace (thanks to `CreateNamespace=true`), render the `deploy/k8s/overlays/prod` Kustomize overlay, and keep the cluster synced. Adjust `spec.source.repoURL`, `targetRevision`, or the overlay path if your repo differs.

Whenever you update manifests or push a new image/tag referenced in the overlay, Argo CD reconciles automatically (self-heal + prune is enabled). Trigger a manual sync from the Argo UI/CLI if you disable automation.

## 6. Postgres & external dependencies

The manifests assume you connect to an external Postgres instance. Point `database_url` inside `config.json` at the connection string for your managed database or a separate StatefulSet you control. S3, Gemini, Imagen and other keys stay in the same config file, so no additional secrets are required by Kubernetes beyond the single JSON blob.
