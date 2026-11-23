#!/usr/bin/env bash
# Runs on the server, called by .github/workflows/ci-cd.yml over SSH:
#   printf '%s\n' "$TOKEN" | bash remote-deploy.sh DEPLOY_PATH IMAGE_PREFIX IMAGE_TAG GHCR_USER
# The registry token comes on stdin so it never shows in the process list.
set -euo pipefail
DEPLOY_PATH=$1 IMAGE_PREFIX=$2 IMAGE_TAG=$3 GHCR_USER=$4
read -r token

cd "$DEPLOY_PATH"
test -f .env || { echo "Missing $DEPLOY_PATH/.env - create it from deploy/.env.example first" >&2; exit 1; }
export IMAGE_PREFIX IMAGE_TAG

printf '%s\n' "$token" | docker login ghcr.io -u "$GHCR_USER" --password-stdin
trap 'docker logout ghcr.io >/dev/null 2>&1 || true' EXIT
docker compose pull

# --wait returns once the backend healthcheck passes, and fails the deploy if it never does.
docker compose up -d --remove-orphans --wait --wait-timeout 180

# Remember the deployed images so a manual "docker compose up -d" keeps them.
set_env() {
  if grep -q "^$1=" .env; then sed -i "s#^$1=.*#$1=$2#" .env; else echo "$1=$2" >> .env; fi
}
set_env IMAGE_PREFIX "$IMAGE_PREFIX"
set_env IMAGE_TAG "$IMAGE_TAG"

docker image prune -f
docker compose ps
