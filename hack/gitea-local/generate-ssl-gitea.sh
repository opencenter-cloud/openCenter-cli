#!/bin/bash

# Gitea SSL Certificate Generator
# This script creates a self-signed SSL certificate suitable for Gitea
# Outputs: cert.pem, key.pem, and ca.pem

set -e

# Configuration
DOMAIN="${1:-localhost}"
CERT_DIR="${2:-./ssl-certs}"
VALIDITY_DAYS=365
KEY_SIZE=2048

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Gitea SSL Certificate Generator${NC}"
echo "================================"

# Create certificate directory
mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

echo -e "${YELLOW}Configuration:${NC}"
echo "Domain: $DOMAIN"
echo "Certificate directory: $(pwd)"
echo "Validity: $VALIDITY_DAYS days"
echo "Key size: $KEY_SIZE bits"
echo ""

# Generate CA private key
echo -e "${YELLOW}Step 1: Generating CA private key...${NC}"
openssl genrsa -out ca-key.pem $KEY_SIZE

# Generate CA certificate
echo -e "${YELLOW}Step 2: Generating CA certificate...${NC}"
openssl req -new -x509 -days $VALIDITY_DAYS -key ca-key.pem -out ca.pem -subj "/C=US/ST=CA/L=San Francisco/O=Gitea/OU=IT/CN=Gitea-CA"

# Generate server private key
echo -e "${YELLOW}Step 3: Generating server private key...${NC}"
openssl genrsa -out key.pem $KEY_SIZE

# Generate server certificate signing request
echo -e "${YELLOW}Step 4: Generating server certificate signing request...${NC}"
openssl req -new -key key.pem -out server.csr -subj "/C=US/ST=TX/L=San Antonio/O=Gitea/OU=PrivateCloud/CN=$DOMAIN"

# Create extensions file for server certificate
cat >server.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = $DOMAIN
DNS.2 = localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Generate server certificate signed by CA
echo -e "${YELLOW}Step 5: Generating server certificate...${NC}"
openssl x509 -req -days $VALIDITY_DAYS -in server.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out cert.pem -extfile server.ext

# Set appropriate permissions
chmod 600 key.pem ca-key.pem
chmod 644 cert.pem ca.pem

# Clean up temporary files
rm -f server.csr server.ext ca-key.pem ca.pem.srl

echo -e "${GREEN}✓ SSL certificates generated successfully!${NC}"
echo ""
echo -e "${YELLOW}Generated files:${NC}"
echo "• cert.pem - Server certificate"
echo "• key.pem  - Server private key"
echo "• ca.pem   - Certificate Authority certificate"
echo ""
echo -e "${YELLOW}Gitea configuration:${NC}"
echo "Add these settings to your app.ini file:"
echo ""
echo "[server]"
echo "PROTOCOL = https"
echo "CERT_FILE = $(pwd)/cert.pem"
echo "KEY_FILE = $(pwd)/key.pem"
echo ""
echo -e "${YELLOW}Notes:${NC}"
echo "• These are self-signed certificates - browsers will show security warnings"
echo "• For production, consider using Let's Encrypt or purchasing certificates from a CA"
echo "• The ca.pem file can be imported into browsers/systems to trust the certificate"
echo ""
echo -e "${RED}Security reminder:${NC} Keep the key.pem file secure and never share it!"
