#!/bin/bash

# Gitea Admin Token and User Creation Script
# This script creates an admin token and a new user in Gitea

set -e # Exit on any error

# Configuration - Update these variables
GITEA_URL="https://localhost:3001" # Your Gitea instance URL
ADMIN_USERNAME="admin"             # Admin username
ADMIN_PASSWORD="gitea"             # Admin password
TOKEN_NAME="admin-api-token"       # Name for the API token

# New user details
NEW_USERNAME="newuser"
NEW_EMAIL="newuser@example.com"
NEW_PASSWORD="newuserpassword"
NEW_FULL_NAME="New User"
NEW_USER_TOKEN_NAME="user-api-token"

# New repository details
REPO_NAME="test-repo"
REPO_DESCRIPTION="Test repository created via API"
REPO_PRIVATE=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Global variables for tokens and URLs
ADMIN_TOKEN=""
NEW_USER_TOKEN=""
repo_html_url=""

echo -e "${YELLOW}Starting Gitea Admin Operations...${NC}"

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

# Function to create admin token
create_admin_token() {
  if [ -f .gitea_admin_token ]; then
    echo -e "${GREEN}✓ Admin token found in .gitea_admin_token file.${NC}"
    ADMIN_TOKEN=$(cat .gitea_admin_token)
    return 0
  fi

  echo -e "${YELLOW}Creating admin token...${NC}"

  # Build JSON with required scopes
  # Need write:admin for user creation, write:user for token creation, read:user for verification
  token_payload=$(
    cat <<'JSON'
{
  "name": "__TOKEN_NAME__",
  "scopes": ["all"]
}
JSON
  )
  token_payload="${token_payload/__TOKEN_NAME__/${TOKEN_NAME}}"

  # Create token using Basic Auth (required for this endpoint)
  response=$(curl -s -k -X POST \
    -u "${ADMIN_USERNAME}:${ADMIN_PASSWORD}" \
    -H "Content-Type: application/json" \
    -d "${token_payload}" \
    "${GITEA_URL}/api/v1/users/${ADMIN_USERNAME}/tokens" 2>/dev/null)

  # Basic error check
  if [ $? -ne 0 ] || [ -z "$response" ]; then
    echo -e "${RED}Error: Failed to create admin token${NC}"
    echo "Please check admin username and password"
    exit 1
  fi

  # Extract token from response
  # Prefer jq if available; fall back to POSIX parsing
  if command -v jq >/dev/null 2>&1; then
    ADMIN_TOKEN=$(echo "$response" | jq -r '.sha1 // empty')
  else
    ADMIN_TOKEN=$(printf '%s' "$response" | grep -o '"sha1":"[^"]*"' | cut -d'"' -f4)
  fi

  if [ -z "$ADMIN_TOKEN" ]; then
    echo -e "${RED}Error: Could not extract token from response${NC}"
    echo "Response: $response"
    if echo "$response" | grep -qi "already exists"; then
      echo -e "${YELLOW}Token '${TOKEN_NAME}' may already exist. Please delete it from Gitea or remove the .gitea_admin_token file.${NC}"
    elif echo "$response" | grep -qi "scope"; then
      echo -e "${YELLOW}Server requires token scopes. Ensure your Gitea version supports scoped tokens and that 'scopes' were sent.${NC}"
    fi
    exit 1
  fi

  echo -e "${GREEN}✓ Admin token created successfully${NC}"
  echo -e "${GREEN}Token: ${ADMIN_TOKEN}${NC}"
  echo "$ADMIN_TOKEN" >.gitea_admin_token
  echo -e "${YELLOW}Token saved to .gitea_admin_token file${NC}"
}

# Function to create new user
create_new_user() {
  echo -e "${YELLOW}Creating new user: ${NEW_USERNAME}...${NC}"

  # Create user using the admin token
  response=$(curl -s -k -X POST \
    -H "Authorization: token ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
            \"username\":\"${NEW_USERNAME}\",
            \"email\":\"${NEW_EMAIL}\",
            \"password\":\"${NEW_PASSWORD}\",
            \"full_name\":\"${NEW_FULL_NAME}\",
            \"must_change_password\":false,
            \"send_notify\":false
        }" \
    "${GITEA_URL}/api/v1/admin/users" 2>/dev/null)

  # Check if request was successful
  if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to create user${NC}"
    exit 1
  fi

  # Check if user was created successfully
  if echo "$response" | grep -q '"id":[0-9]'; then
    echo -e "${GREEN}✓ User '${NEW_USERNAME}' created successfully${NC}"
    user_id=$(echo "$response" | grep -o '"id":[0-9]*' | cut -d':' -f2)
    echo -e "${GREEN}User ID: ${user_id}${NC}"
  elif echo "$response" | grep -q "username already exists"; then
    echo -e "${YELLOW}Username '${NEW_USERNAME}' already exists, continuing...${NC}"
  elif echo "$response" | grep -q "email already exists"; then
    echo -e "${YELLOW}Email '${NEW_EMAIL}' already exists, continuing...${NC}"
  else
    echo -e "${RED}Error: Failed to create user${NC}"
    echo "Response: $response"
    exit 1
  fi
}

# Function to verify user creation
verify_user_creation() {
  echo -e "${YELLOW}Verifying user creation...${NC}"

  response=$(curl -s -k \
    -H "Authorization: token ${ADMIN_TOKEN}" \
    "${GITEA_URL}/api/v1/users/${NEW_USERNAME}" 2>/dev/null)

  if echo "$response" | grep -q "\"login\":\"${NEW_USERNAME}\""; then
    echo -e "${GREEN}✓ User verification successful${NC}"
  else
    echo -e "${RED}Warning: Could not verify user creation${NC}"
  fi
}

# Function to create new user token
create_new_user_token() {
  if [ -f ".gitea_${NEW_USERNAME}_token" ]; then
    echo -e "${GREEN}✓ User token found in .gitea_${NEW_USERNAME}_token file.${NC}"
    NEW_USER_TOKEN=$(cat ".gitea_${NEW_USERNAME}_token")
    return 0
  fi
  echo -e "${YELLOW}Creating token for user: ${NEW_USERNAME}...${NC}"

  # Method 1: Try creating token using admin privileges (requires write:user scope)
  response=$(curl -s -k -X POST \
    -H "Authorization: token ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
            \"name\":\"${NEW_USER_TOKEN_NAME}\",
            \"scopes\":[\"all\"]
        }" \
    "${GITEA_URL}/api/v1/users/${NEW_USERNAME}/tokens" 2>/dev/null)

  # Check if request was successful
  if [ $? -eq 0 ] && [ -n "$response" ] && ! echo "$response" | grep -qi "scope"; then
    # Extract token from response
    if command -v jq >/dev/null 2>&1; then
      NEW_USER_TOKEN=$(echo "$response" | jq -r '.sha1 // empty')
    else
      NEW_USER_TOKEN=$(printf '%s' "$response" | grep -o '"sha1":"[^"]*"' | cut -d'"' -f4)
    fi

    if [ -n "$NEW_USER_TOKEN" ]; then
      echo -e "${GREEN}✓ User token created successfully via admin API${NC}"
      echo -e "${GREEN}User Token: ${NEW_USER_TOKEN}${NC}"
      echo "$NEW_USER_TOKEN" >".gitea_${NEW_USERNAME}_token"
      echo -e "${YELLOW}User token saved to .gitea_${NEW_USERNAME}_token file${NC}"
      return 0
    fi
  fi

  # Method 2: Fallback - Create token using basic auth with user credentials
  echo -e "${YELLOW}Admin token lacks write:user scope, trying basic auth method...${NC}"

  response=$(curl -s -k -X POST \
    -u "${NEW_USERNAME}:${NEW_PASSWORD}" \
    -H "Content-Type: application/json" \
    -d "{
            \"name\":\"${NEW_USER_TOKEN_NAME}\",
            \"scopes\":[\"all\"]
        }" \
    "${GITEA_URL}/api/v1/users/${NEW_USERNAME}/tokens" 2>/dev/null)

  # Check if request was successful
  if [ $? -ne 0 ] || [ -z "$response" ]; then
    echo -e "${RED}Error: Failed to create user token${NC}"
    echo -e "${YELLOW}Note: This could be due to insufficient admin token permissions${NC}"
    echo -e "${YELLOW}Consider updating admin token with 'write:user' scope or using basic auth${NC}"
    exit 1
  fi

  # Extract token from response
  if command -v jq >/dev/null 2>&1; then
    NEW_USER_TOKEN=$(echo "$response" | jq -r '.sha1 // empty')
  else
    NEW_USER_TOKEN=$(printf '%s' "$response" | grep -o '"sha1":"[^"]*"' | cut -d'"' -f4)
  fi

  if [ -z "$NEW_USER_TOKEN" ]; then
    echo -e "${RED}Error: Could not extract user token from response${NC}"
    echo "Response: $response"
    if echo "$response" | grep -qi "already exists"; then
      echo -e "${YELLOW}Token '${NEW_USER_TOKEN_NAME}' may already exist for user ${NEW_USERNAME}. Please delete it from Gitea or remove the .gitea_${NEW_USERNAME}_token file.${NC}"
    elif echo "$response" | grep -qi "scope"; then
      echo -e "${YELLOW}Token scope issue. Try updating admin token scopes or use basic auth.${NC}"
    fi
    exit 1
  fi

  echo -e "${GREEN}✓ User token created successfully via basic auth${NC}"
  echo -e "${GREEN}User Token: ${NEW_USER_TOKEN}${NC}"
  echo "$NEW_USER_TOKEN" >".gitea_${NEW_USERNAME}_token"
  echo -e "${YELLOW}User token saved to .gitea_${NEW_USERNAME}_token file${NC}"
}

# Function to create repository for new user
create_new_user_repo() {
  echo -e "${YELLOW}Creating repository '${REPO_NAME}' for user: ${NEW_USERNAME}...${NC}"

  # Create repository using the new user's token
  response=$(curl -s -k -X POST \
    -H "Authorization: token ${NEW_USER_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
            \"name\":\"${REPO_NAME}\",
            \"description\":\"${REPO_DESCRIPTION}\",
            \"private\":${REPO_PRIVATE},
            \"auto_init\":true,
            \"gitignores\":\"\",
            \"license\":\"\",
            \"readme\":\"Default\"
        }" \
    "${GITEA_URL}/api/v1/user/repos" 2>/dev/null)

  # Check if request was successful
  if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to create repository${NC}"
    exit 1
  fi

  # Check if repository was created successfully
  if echo "$response" | grep -q "\"name\":\"${REPO_NAME}\""; then
    echo -e "${GREEN}✓ Repository '${REPO_NAME}' created successfully${NC}"

    # Extract repository details
    if command -v jq >/dev/null 2>&1; then
      repo_id=$(echo "$response" | jq -r '.id // empty')
      repo_clone_url=$(echo "$response" | jq -r '.clone_url // empty')
      repo_html_url=$(echo "$response" | jq -r '.html_url // empty')
    else
      repo_id=$(echo "$response" | grep -o '"id":[0-9]*' | cut -d':' -f2)
      repo_clone_url=$(echo "$response" | grep -o '"clone_url":"[^"]*"' | cut -d'"' -f4)
      repo_html_url=$(echo "$response" | grep -o '"html_url":"[^"]*"' | cut -d'"' -f4)
    fi

    echo -e "${GREEN}Repository ID: ${repo_id}${NC}"
    echo -e "${GREEN}Clone URL: ${repo_clone_url}${NC}"
    echo -e "${GREEN}Repository URL: ${repo_html_url}${NC}"
  elif echo "$response" | grep -qi "already exists"; then
    echo -e "${YELLOW}Repository '${REPO_NAME}' already exists for user ${NEW_USERNAME}, continuing...${NC}"
    # We need to get the html_url for the summary at the end
    repo_html_url="${GITEA_URL}/${NEW_USERNAME}/${REPO_NAME}"
  else
    echo -e "${RED}Error: Failed to create repository${NC}"
    echo "Response: $response"
    # Check for common errors
    if echo "$response" | grep -qi "permission"; then
      echo -e "${YELLOW}Permission denied. Check if user token has sufficient privileges.${NC}"
    fi
    exit 1
  fi
}

# Function to verify repository creation
verify_repo_creation() {
  echo -e "${YELLOW}Verifying repository creation...${NC}"

  response=$(curl -s -k \
    -H "Authorization: token ${NEW_USER_TOKEN}" \
    "${GITEA_URL}/api/v1/repos/${NEW_USERNAME}/${REPO_NAME}" 2>/dev/null)

  if echo "$response" | grep -q "\"name\":\"${REPO_NAME}\""; then
    echo -e "${GREEN}✓ Repository verification successful${NC}"
  else
    echo -e "${RED}Warning: Could not verify repository creation${NC}"
  fi
}

# Main execution
main() {
  echo "=========================================="
  echo "Gitea Admin Token and User Creation Script"
  echo "=========================================="

  check_gitea_connection
  create_admin_token
  create_new_user
  verify_user_creation
  create_new_user_token
  create_new_user_repo
  verify_repo_creation

  echo ""
  echo -e "${GREEN}=========================================="
  echo -e "✓ All operations completed successfully!"
  echo -e "=========================================="
  echo -e "Admin token: ${ADMIN_TOKEN}"
  echo -e "New user: ${NEW_USERNAME}"
  echo -e "User email: ${NEW_EMAIL}"
  echo -e "User token: ${NEW_USER_TOKEN}"
  echo -e "Repository: ${REPO_NAME}"
  echo -e "Repository URL: ${repo_html_url}"
  echo -e "${NC}"
  echo "Summary:"
  echo "- Admin token: ${ADMIN_TOKEN}"
  echo "- User '${NEW_USERNAME}' token: ${NEW_USER_TOKEN}"
  echo "- Repository URL: ${repo_html_url}"
  echo ""
  echo "Files created:"
  echo "- Admin token saved to: .gitea_admin_token"
  echo "- User token saved to: .gitea_${NEW_USERNAME}_token"
  echo ""
  echo "Notes:"
  echo "- Repository '${REPO_NAME}' has been created for ${NEW_USERNAME}"
  echo "- Keep your tokens secure and don't commit them to version control"
  echo "- You can use these tokens for further API operations"
}

# Run main function
main
