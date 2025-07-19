#!/bin/bash

# Test script for DGAUTH environment-based authentication
# Usage: ./test-dgauth.sh username password

if [ $# -ne 2 ]; then
    echo "Usage: $0 <username> <password>"
    echo "Example: $0 admin mypassword"
    exit 1
fi

USERNAME="$1"
PASSWORD="$2"
DGAUTH_VALUE="${USERNAME}:${PASSWORD}"

echo "Testing DGAUTH authentication with: ${USERNAME}:***"
echo "Connecting to localhost:2222 with DGAUTH environment variable..."

# Use SSH with environment variable
ssh -p 2222 \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -o SetEnv="DGAUTH=${DGAUTH_VALUE}" \
    -o SendEnv="DGAUTH" \
    localhost