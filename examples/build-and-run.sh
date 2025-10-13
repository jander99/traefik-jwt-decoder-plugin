#!/bin/bash
set -e

# build-and-run.sh - Build custom Traefik image and run production environment

echo "🔨 Building custom Traefik image with JWT decoder plugin..."
cd "$(dirname "$0")/.."

# Build the custom Traefik image
docker build -f Dockerfile.traefik -t traefik-jwt-decoder:latest .

echo "✅ Image built successfully!"
echo ""
echo "📦 Image details:"
docker images | grep traefik-jwt-decoder

echo ""
echo "🚀 Starting production environment..."
cd examples
docker-compose -f docker-compose.production.yml up -d

echo ""
echo "⏳ Waiting for Traefik to start..."
sleep 5

echo ""
echo "✅ Production environment started!"
echo ""
echo "🔗 Available endpoints:"
echo "  - Traefik Dashboard: http://localhost:8080"
echo "  - Whoami Service: http://whoami.localhost"
echo ""
echo "🧪 Test with:"
echo "  ./test-plugin.sh"
echo ""
echo "🛑 Stop with:"
echo "  docker-compose -f docker-compose.production.yml down"
