# Docker Compose + Portainer Deployment

This folder contains everything needed to run the app on a single VPS with Docker Compose and manage it through Portainer's Git-backed stacks. The compose file builds the Go API container from the repository, runs PostgreSQL 16, and binds in your private `config.json` plus any service account files.

## 1. Prepare the VPS

1. Install Docker Engine + Compose plugin (Ubuntu example):
   ```bash
   sudo apt update && sudo apt install -y ca-certificates curl gnupg
   curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker.gpg
   echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
     | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
   sudo apt update && sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
   sudo usermod -aG docker $USER
   ```
2. Create folders for secrets and config outside of the repo clone so they never accidentally end up in git:
   ```bash
   sudo mkdir -p /opt/k2marketing/{config,secrets}
   sudo chown -R $USER:$USER /opt/k2marketing
   ```
3. Use the sample file to craft a production config:
   ```bash
   cp deploy/config.docker.json.example /opt/k2marketing/config/config.json
   nano /opt/k2marketing/config/config.json
   ```
   - Update `database_url` if you change database credentials.
   - Point `ai.gemini.service_account` (and `ai.imagen.service_account` if needed) to `/secrets/<your-file>.json`.
4. Drop your Google service-account JSON (and any other secret files) into `/opt/k2marketing/secrets/`. In the config you can now reference `/secrets/<file>` paths.

## 2. Running the stack manually with Docker Compose

1. Copy the default env template and tweak it (the `.env` file is already ignored by git):
   ```bash
   cp deploy/.env.example deploy/.env
   nano deploy/.env
   ```
   - Set `APP_PORT` to whichever port you want to expose externally (often 80/443 behind a reverse proxy).
   - Set `CONFIG_PATH` and `SECRETS_DIR` to the absolute paths created under `/opt/k2marketing`.
   - If you change the DB user/password, update both `.env` and `/opt/k2marketing/config/config.json`.
2. Start (or update) the stack:
   ```bash
   docker compose --env-file deploy/.env up -d --build
   ```
3. Verify everything:
   - `docker compose ps`
   - `docker compose logs -f app`
   - `curl http://SERVER_IP:${APP_PORT}/health`
4. On each deploy, pull the latest code and rebuild:
   ```bash
   git pull origin main
   docker compose --env-file deploy/.env up -d --build
   ```
   Compose rebuilds the `app` image using the repo's Dockerfile and reuses the existing Postgres volume (`pg-data`).

## 3. Managing deployments via Portainer (Git-based stack)

1. Install Portainer CE once on the VPS (or another Docker host):
   ```bash
   docker volume create portainer_data
   docker run -d \
     -p 8000:8000 -p 9443:9443 \
     --name portainer \
     --restart=unless-stopped \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v portainer_data:/data \
     portainer/portainer-ce:2.20.3
   ```
2. Log in to `https://SERVER_IP:9443`, complete the setup wizard, and add your local environment if it is not detected automatically.
3. Create the same `/opt/k2marketing/{config,secrets}` folders on the Docker host Portainer will deploy to and place `config.json` + secret files there (as described in section 1). Portainer mount bindings reference host paths, so these files stay outside of git.
4. Add a new **Stack** → **From git repository**:
   - **Repository URL**: `https://github.com/<your-org>/k2MarketingAi.git`
   - **Repository reference**: `main`
   - **Compose path**: `docker-compose.yml`
   - Under _Environment variables_, add the same keys from `deploy/.env.example` (APP_PORT, CONFIG_PATH, SECRETS_DIR, DB_USER, DB_PASSWORD, DB_NAME, GOOGLE_APPLICATION_CREDENTIALS).
   - (Optional) Enable **Automatic updates** → **GitOps** and choose your preferred poll interval so Portainer surfaces “Out of sync” when a new commit lands on `main`.
5. Click **Deploy the stack**. Portainer will clone the repo, run `docker compose`, and start both services.
6. Checking for updates:
   - After you push to `main`, Portainer marks the stack as “Update available”. Open the stack and press **Pull and redeploy** (or rely on the auto-update schedule) to rebuild the container.
   - Use the Portainer UI to watch logs, restart services, or see resource usage.

## 4. Operational notes

- `config.json` and `/opt/k2marketing/secrets` are never part of git—back them up securely.
- PostgreSQL data lives in the named volume `pg-data`. Back up with `docker run --rm --volumes-from $(docker compose ps -q db) ...` or via `pg_dump`.
- When running without S3, uploads fall back to the container's temp directory. For durable media, configure the `media` section to point at S3/MinIO/etc.
- Put a reverse proxy (Caddy, Nginx, Traefik) or a managed load balancer in front of the `app` service if you need HTTPS. Point the proxy to `APP_PORT` defined in the env file.
- To rotate credentials, update `/opt/k2marketing/config/config.json` (and secret files if needed) and redeploy the stack so the container picks up the new mounts.
