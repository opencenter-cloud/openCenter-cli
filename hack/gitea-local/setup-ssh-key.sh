#!/bin/bash

# Gitea SSH Key Generation and Upload Script
# This script generates an SSH key and uploads it to a Gitea user account

set -e # Exit on any error

# Configuration - Update these variables as needed or set via environment
GITEA_URL="${GITEA_URL:-https://localhost:3001}"          # Your Gitea instance URL
USERNAME="${USERNAME:-newuser}"                           # Username for the SSH key
SSH_KEY_NAME="${SSH_KEY_NAME:-gitea-local-key}"           # Name for the SSH key in Gitea
SSH_KEY_COMMENT="${SSH_KEY_COMMENT:-$USERNAME@gitea-local}" # Comment for the SSH key
SSH_KEY_TYPE="${SSH_KEY_TYPE:-ed25519}"                   # SSH key type (ed25519, rsa, ecdsa)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Global variables
USER_TOKEN=""
SSH_PRIVATE_KEY_PATH=""
SSH_PUBLIC_KEY_PATH=""
SSH_PUBLIC_KEY_CONTENT=""

echo -e "${YELLOW}Starting Gitea SSH Key Setup...${NC}"

# Function to check if Gitea is accessible
check_gitea_connection() {
  echo -e "${YELLOW}Checking Gitea connection...${NC}"
  if ! curl -s -k -f "${GITEA_URL}/api/v1/version" >/dev/null; then
    echo -e "${RED}Error: Cannot connect to Gitea at ${GITEA_URL}${NC}"
    echo "Please check that:"
    echo "1. Gitea is running"
    echo "2. The URL is correct"
    echo "3. No firewall is blocking the connection"
    exit 1
  fi
  echo -e "${GREEN}✓ Gitea connection successful${NC}"
}

# Function to load user token
load_user_token() {
  local token_file=".gitea_${USERNAME}_token"

  if [ ! -f "$token_file" ]; then
    echo -e "${RED}Error: User token file '${token_file}' not found${NC}"
    echo "Please ensure you have run the configure-gitea-user-tokens.sh script first"
    echo "or create the token file manually"
    exit 1
  fi

  USER_TOKEN=$(cat "$token_file")

  if [ -z "$USER_TOKEN" ]; then
    echo -e "${RED}Error: User token is empty in '${token_file}'${NC}"
    exit 1
  fi

  echo -e "${GREEN}✓ User token loaded successfully${NC}"
}

# Function to verify user token works
verify_user_token() {
  echo -e "${YELLOW}Verifying user token...${NC}"

  response=$(curl -s -k \
    -H "Authorization: token ${USER_TOKEN}" \
    "${GITEA_URL}/api/v1/user" 2>/dev/null)

  if [ $? -ne 0 ] || [ -z "$response" ]; then
    echo -e "${RED}Error: Failed to verify user token${NC}"
    exit 1
  fi

  if echo "$response" | grep -q "\"login\":\"${USERNAME}\""; then
    echo -e "${GREEN}✓ User token verification successful${NC}"
  else
    echo -e "${RED}Error: Token does not belong to user '${USERNAME}'${NC}"
    echo "Response: $response"
    exit 1
  fi
}

# Function to generate SSH key
generate_ssh_key() {
  echo -e "${YELLOW}Generating SSH key...${NC}"

  # Set SSH key paths
  SSH_PRIVATE_KEY_PATH="$HOME/.ssh/gitea_${USERNAME}_key"
  SSH_PUBLIC_KEY_PATH="${SSH_PRIVATE_KEY_PATH}.pub"

  # Check if key already exists
  if [ -f "$SSH_PRIVATE_KEY_PATH" ] || [ -f "$SSH_PUBLIC_KEY_PATH" ]; then
    echo -e "${YELLOW}SSH key already exists at ${SSH_PRIVATE_KEY_PATH}${NC}"
    echo -e "${YELLOW}Do you want to overwrite it? (y/N)${NC}"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
      echo -e "${YELLOW}Using existing SSH key...${NC}"
      if [ ! -f "$SSH_PUBLIC_KEY_PATH" ]; then
        echo -e "${RED}Error: Public key file missing: ${SSH_PUBLIC_KEY_PATH}${NC}"
        exit 1
      fi
    else
      echo -e "${YELLOW}Removing existing SSH key...${NC}"
      rm -f "$SSH_PRIVATE_KEY_PATH" "$SSH_PUBLIC_KEY_PATH"
    fi
  fi

  # Generate new key if it doesn't exist
  if [ ! -f "$SSH_PRIVATE_KEY_PATH" ]; then
    case "$SSH_KEY_TYPE" in
      "ed25519")
        ssh-keygen -t ed25519 -f "$SSH_PRIVATE_KEY_PATH" -N "" -C "$SSH_KEY_COMMENT"
        ;;
      "rsa")
        ssh-keygen -t rsa -b 4096 -f "$SSH_PRIVATE_KEY_PATH" -N "" -C "$SSH_KEY_COMMENT"
        ;;
      "ecdsa")
        ssh-keygen -t ecdsa -b 521 -f "$SSH_PRIVATE_KEY_PATH" -N "" -C "$SSH_KEY_COMMENT"
        ;;
      *)
        echo -e "${RED}Error: Unsupported SSH key type: ${SSH_KEY_TYPE}${NC}"
        echo "Supported types: ed25519, rsa, ecdsa"
        exit 1
        ;;
    esac

    if [ $? -ne 0 ]; then
      echo -e "${RED}Error: Failed to generate SSH key${NC}"
      exit 1
    fi

    echo -e "${GREEN}✓ SSH key generated successfully${NC}"
  fi

  # Read public key content
  if [ ! -f "$SSH_PUBLIC_KEY_PATH" ]; then
    echo -e "${RED}Error: Public key file not found: ${SSH_PUBLIC_KEY_PATH}${NC}"
    exit 1
  fi

  SSH_PUBLIC_KEY_CONTENT=$(cat "$SSH_PUBLIC_KEY_PATH")
  echo -e "${GREEN}✓ SSH public key content loaded${NC}"
  echo -e "${YELLOW}Key fingerprint:${NC}"
  ssh-keygen -lf "$SSH_PUBLIC_KEY_PATH"
}

# Function to check if SSH key already exists in Gitea
check_existing_ssh_key() {
  echo -e "${YELLOW}Checking for existing SSH keys in Gitea...${NC}"

  response=$(curl -s -k \
    -H "Authorization: token ${USER_TOKEN}" \
    "${GITEA_URL}/api/v1/user/keys" 2>/dev/null)

  if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to fetch existing SSH keys${NC}"
    exit 1
  fi

  # Check if our key already exists (by title or content)
  if echo "$response" | grep -q "\"title\":\"${SSH_KEY_NAME}\""; then
    echo -e "${YELLOW}SSH key with title '${SSH_KEY_NAME}' already exists in Gitea${NC}"
    echo -e "${YELLOW}Do you want to delete and re-upload it? (y/N)${NC}"
    read -r delete_response
    if [[ "$delete_response" =~ ^[Yy]$ ]]; then
      delete_existing_ssh_key
    else
      echo -e "${YELLOW}Skipping SSH key upload...${NC}"
      return 1
    fi
  fi

  return 0
}

# Function to delete existing SSH key
delete_existing_ssh_key() {
  echo -e "${YELLOW}Deleting existing SSH key...${NC}"

  # Get the key ID
  response=$(curl -s -k \
    -H "Authorization: token ${USER_TOKEN}" \
    "${GITEA_URL}/api/v1/user/keys" 2>/dev/null)

  if command -v jq >/dev/null 2>&1; then
    key_id=$(echo "$response" | jq -r ".[] | select(.title == \"${SSH_KEY_NAME}\") | .id")
  else
    # Fallback parsing without jq - more fragile but works
    key_id=$(echo "$response" | grep -B2 -A2 "\"title\":\"${SSH_KEY_NAME}\"" | grep '"id":' | grep -o '[0-9]*' | head -1)
  fi

  if [ -z "$key_id" ]; then
    echo -e "${RED}Error: Could not find key ID for '${SSH_KEY_NAME}'${NC}"
    exit 1
  fi

  # Delete the key
  delete_response=$(curl -s -k -X DELETE \
    -H "Authorization: token ${USER_TOKEN}" \
    "${GITEA_URL}/api/v1/user/keys/${key_id}" 2>/dev/null)

  if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Existing SSH key deleted successfully${NC}"
  else
    echo -e "${RED}Error: Failed to delete existing SSH key${NC}"
    exit 1
  fi
}

# Function to upload SSH key to Gitea
upload_ssh_key() {
  echo -e "${YELLOW}Uploading SSH key to Gitea...${NC}"

  # Create JSON payload
  json_payload=$(cat <<JSON
{
  "title": "${SSH_KEY_NAME}",
  "key": "${SSH_PUBLIC_KEY_CONTENT}"
}
JSON
)

  response=$(curl -s -k -X POST \
    -H "Authorization: token ${USER_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$json_payload" \
    "${GITEA_URL}/api/v1/user/keys" 2>/dev/null)

  if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to upload SSH key${NC}"
    exit 1
  fi

  # Check if upload was successful
  if echo "$response" | grep -q '"id":[0-9]'; then
    echo -e "${GREEN}✓ SSH key uploaded successfully${NC}"

    # Extract key details
    if command -v jq >/dev/null 2>&1; then
      key_id=$(echo "$response" | jq -r '.id // empty')
      fingerprint=$(echo "$response" | jq -r '.fingerprint // empty')
    else
      key_id=$(echo "$response" | grep -o '"id":[0-9]*' | cut -d':' -f2)
      fingerprint=$(echo "$response" | grep -o '"fingerprint":"[^"]*"' | cut -d'"' -f4)
    fi

    echo -e "${GREEN}Key ID: ${key_id}${NC}"
    echo -e "${GREEN}Fingerprint: ${fingerprint}${NC}"
  else
    echo -e "${RED}Error: SSH key upload failed${NC}"
    echo "Response: $response"

    # Check for common errors
    if echo "$response" | grep -qi "already exists"; then
      echo -e "${YELLOW}SSH key content already exists in Gitea${NC}"
    elif echo "$response" | grep -qi "invalid"; then
      echo -e "${YELLOW}SSH key format is invalid${NC}"
    fi
    exit 1
  fi
}

# Function to verify SSH key upload
verify_ssh_key_upload() {
  echo -e "${YELLOW}Verifying SSH key upload...${NC}"

  response=$(curl -s -k \
    -H "Authorization: token ${USER_TOKEN}" \
    "${GITEA_URL}/api/v1/user/keys" 2>/dev/null)

  if echo "$response" | grep -q "\"title\":\"${SSH_KEY_NAME}\""; then
    echo -e "${GREEN}✓ SSH key verification successful${NC}"
  else
    echo -e "${RED}Warning: Could not verify SSH key upload${NC}"
  fi
}

# Function to display usage instructions
display_usage_instructions() {
  echo ""
  echo -e "${GREEN}=========================================="
  echo -e "✓ SSH Key Setup Complete!"
  echo -e "=========================================="
  echo -e "SSH Key Details:"
  echo -e "- Name: ${SSH_KEY_NAME}"
  echo -e "- Type: ${SSH_KEY_TYPE}"
  echo -e "- Private key: ${SSH_PRIVATE_KEY_PATH}"
  echo -e "- Public key: ${SSH_PUBLIC_KEY_PATH}"
  echo -e "${NC}"
  echo "Usage Instructions:"
  echo ""
  echo "1. Clone repository using SSH:"
  echo "   git clone git@localhost:3001:${USERNAME}/test-repo.git"
  echo ""
  echo "2. If using a non-standard SSH key path, configure SSH:"
  echo "   echo 'Host localhost' >> ~/.ssh/config"
  echo "   echo '  HostName localhost' >> ~/.ssh/config"
  echo "   echo '  Port 22' >> ~/.ssh/config"
  echo "   echo '  User git' >> ~/.ssh/config"
  echo "   echo '  IdentityFile ${SSH_PRIVATE_KEY_PATH}' >> ~/.ssh/config"
  echo "   echo '  IdentitiesOnly yes' >> ~/.ssh/config"
  echo ""
  echo "3. Test SSH connection:"
  echo "   ssh -T git@localhost"
  echo ""
  echo "Notes:"
  echo "- Keep your private key secure (${SSH_PRIVATE_KEY_PATH})"
  echo "- The key has been uploaded to user '${USERNAME}' in Gitea"
  echo "- You can manage SSH keys via Gitea web interface or API"
}

# Main execution
main() {
  echo "========================================"
  echo "Gitea SSH Key Generation and Upload Script"
  echo "========================================"
  echo "Configuration:"
  echo "- Gitea URL: ${GITEA_URL}"
  echo "- Username: ${USERNAME}"
  echo "- SSH Key Name: ${SSH_KEY_NAME}"
  echo "- SSH Key Type: ${SSH_KEY_TYPE}"
  echo "========================================"

  check_gitea_connection
  load_user_token
  verify_user_token
  generate_ssh_key

  if check_existing_ssh_key; then
    upload_ssh_key
    verify_ssh_key_upload
  fi

  display_usage_instructions
}

# Run main function
main "$@"