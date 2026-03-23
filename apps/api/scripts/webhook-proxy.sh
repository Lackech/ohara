#!/bin/bash
# Start smee.io webhook proxy for local GitHub webhook testing.
#
# Usage:
#   ./scripts/webhook-proxy.sh                    # Uses SMEE_URL from .env
#   ./scripts/webhook-proxy.sh https://smee.io/x  # Uses provided URL
#
# First time? Go to https://smee.io and click "Start a new channel",
# then set SMEE_URL in your .env file.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$SCRIPT_DIR/../.env"

# Get URL from argument or .env
SMEE_URL="${1:-}"

if [ -z "$SMEE_URL" ] && [ -f "$ENV_FILE" ]; then
  SMEE_URL=$(grep '^SMEE_URL=' "$ENV_FILE" | cut -d'=' -f2- | tr -d '"' | tr -d "'")
fi

if [ -z "$SMEE_URL" ]; then
  echo "No SMEE_URL found."
  echo ""
  echo "1. Go to https://smee.io and click 'Start a new channel'"
  echo "2. Copy the URL and either:"
  echo "   - Add SMEE_URL=https://smee.io/xxxxx to your .env"
  echo "   - Or run: ./scripts/webhook-proxy.sh https://smee.io/xxxxx"
  echo ""
  echo "3. Set the same URL as the Webhook URL in your GitHub App settings"
  exit 1
fi

echo "Proxying webhooks:"
echo "  From: $SMEE_URL"
echo "  To:   http://localhost:3001/webhooks/github"
echo ""

npx -y smee-client -u "$SMEE_URL" -t http://localhost:3001/webhooks/github
