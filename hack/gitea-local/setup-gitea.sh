#!/bin/bash

echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
echo "!!! WARNING: THIS SCRIPT IS FOR TESTING PURPOSES ONLY.       !!!"
echo "!!! DO NOT USE IN A PRODUCTION ENVIRONMENT.                  !!!"
echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
echo ""

# Gitea Container Runner Script
# Usage: ./run-gitea.sh [start|stop|destroy] [-y]
# Set CONTAINER_RUNTIME environment variable to either "podman" or "docker"
# Examples:
#   CONTAINER_RUNTIME=podman ./run-gitea.sh start
#   ./run-gitea.sh stop
#   ./run-gitea.sh destroy -y

# Default values
ACTION="start"
SKIP_CONFIRMATION=false

# Argument parsing
while [[ "$#" -gt 0 ]]; do
  case $1 in
  start | stop | destroy)
    ACTION="$1"
    shift
    ;;
  -y | --yes)
    SKIP_CONFIRMATION=true
    shift
    ;;
  *)
    echo "Unknown parameter passed: $1"
    exit 1
    ;;
  esac
done

# Default to docker if CONTAINER_RUNTIME is not set
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-docker}

# Validate container runtime
if [[ "$CONTAINER_RUNTIME" != "podman" && "$CONTAINER_RUNTIME" != "docker" ]]; then
  echo "Error: CONTAINER_RUNTIME must be either 'podman' or 'docker'"
  echo "Current value: $CONTAINER_RUNTIME"
  exit 1
fi

# Check if the specified container runtime is available
if ! command -v "$CONTAINER_RUNTIME" &>/dev/null; then
  echo "Error: $CONTAINER_RUNTIME is not installed or not in PATH"
  exit 1
fi

echo "Using container runtime: $CONTAINER_RUNTIME"
echo "Action: $ACTION"

# Gitea configuration
GITEA_IMAGE="docker.gitea.com/gitea:1.24.5"
GITEA_DATA_DIR="$(pwd)/gitea" # Full path
GITEA_HTTP_PORT="3000"
GITEA_HTTPS_PORT="3001"
GITEA_SSH_PORT="2222"
CONTAINER_NAME="gitea"

# Function to stop container
stop_container() {
  echo "Stopping Gitea container..."
  if $CONTAINER_RUNTIME stop "$CONTAINER_NAME" 2>/dev/null; then
    echo "✅ Container stopped successfully"
  else
    echo "⚠️  Container was not running or already stopped"
  fi

  echo "Removing container..."
  if $CONTAINER_RUNTIME rm "$CONTAINER_NAME" 2>/dev/null; then
    echo "✅ Container removed successfully"
  else
    echo "ℹ️  Container was already removed"
  fi
}

# Function to destroy everything
destroy_gitea() {
  echo "🔥 DESTROYING GITEA - This will remove ALL data!"

  if [ "$SKIP_CONFIRMATION" = false ]; then
    read -p "Are you sure you want to continue? This cannot be undone! (type 'yes' to confirm): " -r
    if [[ $REPLY != "yes" ]]; then
      echo "❌ Destroy cancelled"
      exit 1
    fi
  else
    echo "Skipping confirmation..."
  fi

  stop_container
  if [ -d "$GITEA_DATA_DIR" ]; then
    echo "Removing Gitea data directory: $GITEA_DATA_DIR"
    rm -rf "$GITEA_DATA_DIR"
    echo "✅ Data directory removed successfully"
  else
    echo "ℹ️  Data directory doesn't exist: $GITEA_DATA_DIR"
  fi
  echo "🔥 Gitea completely destroyed!"
}

# Handle different actions
case "$ACTION" in
"stop")
  stop_container
  exit 0
  ;;
"destroy")
  destroy_gitea
  exit 0
  ;;
"start")
  # Continue with start logic below
  ;;
esac

# Create data directory if it doesn't exist
echo "Ensuring data directory exists: $GITEA_DATA_DIR"
mkdir -p "$GITEA_DATA_DIR/gitea/conf"
if [ ! -d "$GITEA_DATA_DIR" ]; then
  echo "❌ Failed to create data directory: $GITEA_DATA_DIR"
  exit 1
fi
echo "✅ Data directory ready: $GITEA_DATA_DIR"
cp ./hack/gitea-local/gitea.db ./gitea/gitea/gitea.db
cp ./hack/gitea-local/gitea.app.ini ./gitea/gitea/conf/app.ini

# Generate self-signed certificate
echo "Generating self-signed SSL certificate..."
mkdir -p "$GITEA_DATA_DIR/gitea/certs"
./hack/gitea-local/generate-ssl-gitea.sh localhost $GITEA_DATA_DIR/gitea/certs/
if [ $? -ne 0 ]; then
  echo "❌ Failed to generate self-signed certificate"
  exit 1
fi
echo "✅ Certificate generated successfully"

# Stop and remove existing container if it exists
echo "Stopping and removing existing container (if any)..."
$CONTAINER_RUNTIME stop "$CONTAINER_NAME" 2>/dev/null || true
$CONTAINER_RUNTIME rm "$CONTAINER_NAME" 2>/dev/null || true

# Run Gitea container
echo "Starting Gitea container..."
$CONTAINER_RUNTIME run -d \
  --name "$CONTAINER_NAME" \
  -e GITEA__database__DB_TYPE=sqlite3 \
  -e GITEA__database__USER=gitea \
  -e GITEA__database__PASSWD=gitea \
  -e GITEA__database__PATH=/data/gitea/gitea.db \
  -e GITEA__server__SSH_DOMAIN=localhost \
  -e GITEA__server__ROOT_URL=https://localhost:$GITEA_HTTPS_PORT \
  -e GITEA__repository__ROOT=/data/gitea/git \
  -e ENABLE_SWAGGER=true \
  -e MAX_RESPONSE_ITEMS=100 \
  -v "$GITEA_DATA_DIR":/data \
  -v /etc/localtime:/etc/localtime:ro \
  -p "$GITEA_HTTP_PORT":3000 \
  -p "$GITEA_HTTPS_PORT":3001 \
  -p "$GITEA_SSH_PORT":22 \
  "$GITEA_IMAGE"

# Check if container started successfully
if [ $? -eq 0 ]; then
  echo "✅ Gitea container started successfully!"
  echo "🌐 Web interface (HTTP): http://localhost:$GITEA_HTTP_PORT"
  echo "🔒 Web interface (HTTPS): https://localhost:$GITEA_HTTPS_PORT"
  echo "🔑 SSH clone URL: ssh://git@localhost:$GITEA_SSH_PORT/user/repo.git"
  echo "📁 Data directory: $GITEA_DATA_DIR"
  echo ""
  echo "Container info:"
  $CONTAINER_RUNTIME ps --filter "name=$CONTAINER_NAME"
else
  echo "❌ Failed to start Gitea container"
  exit 1
fi

sleep 5

echo "Container logs:"
$CONTAINER_RUNTIME logs "$CONTAINER_NAME"

echo ""
echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
echo "!!! WARNING: THIS SCRIPT IS FOR TESTING PURPOSES ONLY.       !!!"
echo "!!! DO NOT USE IN A PRODUCTION ENVIRONMENT.                  !!!"
echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
