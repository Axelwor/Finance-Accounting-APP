#!/bin/bash
# Deploy script satu-command untuk Finance Accounting APP
# Usage: ./deploy.sh [migration_number]
#   ./deploy.sh              → pull + rebuild api/web
#   ./deploy.sh 000045       → pull + apply migration 000045 + rebuild
set -e

REMOTE="finance-accounting-vps"
APP_DIR="~/Finance-Accounting-APP"

echo "🚀 Deploying to $REMOTE..."
ssh "$REMOTE" "cd $APP_DIR && git pull origin main"

# Apply migration jika argument diberikan
if [ -n "$1" ]; then
  echo "📦 Applying migration $1..."
  MIGRATION_FILE=$(ssh "$REMOTE" "cd $APP_DIR && ls backend/migrations/${1}_*.up.sql 2>/dev/null | head -1")
  if [ -z "$MIGRATION_FILE" ]; then
    echo "❌ Migration $1 tidak ditemukan"
    exit 1
  fi
  ssh "$REMOTE" "cd $APP_DIR && cat '$MIGRATION_FILE' | docker exec -i finance-db psql -U finance -d finance"
  echo "✅ Migration $1 applied"
fi

echo "🐳 Rebuilding containers..."
ssh "$REMOTE" "cd $APP_DIR && docker compose up -d --build api web"

echo "⏳ Waiting for containers..."
sleep 5

echo "🔍 Verifying..."
ssh "$REMOTE" "docker ps --format '{{.Names}}: {{.Status}}'"
echo ""
echo "🌐 Health check:"
curl -s https://accounting.tikuma.net/healthz/detail | python3 -m json.tool 2>/dev/null || curl -s https://accounting.tikuma.net/healthz
echo ""
echo "✅ Deploy selesai!"
