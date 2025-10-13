# Production Deployment Guide

This guide covers deploying the JWT decoder plugin in production environments, including air-gapped deployments where internet access is restricted.

## Table of Contents

- [Deployment Strategies](#deployment-strategies)
- [Self-Contained Docker Image](#self-contained-docker-image)
- [Air-Gapped Environments](#air-gapped-environments)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Configuration Management](#configuration-management)
- [Monitoring and Observability](#monitoring-and-observability)

---

## Deployment Strategies

### Strategy Comparison

| Strategy | Internet Required | Startup Time | Complexity | Use Case |
|----------|------------------|--------------|------------|----------|
| **Volume Mount** | No | Fast | Low | Development, local testing |
| **Pre-built Image** | No (build time only) | Fast | Medium | Production, air-gapped |
| **Plugin Catalog** | Yes (runtime) | Slower | Low | Cloud, always-connected |

**Recommendation**: Use **pre-built image** for production deployments.

---

## Self-Contained Docker Image

Build a custom Traefik image with the plugin pre-installed, eliminating runtime internet dependency.

### Quick Start

```bash
# Build and run production environment
cd examples
./build-and-run.sh
```

### Manual Build

**1. Build Custom Image**

```bash
# From repository root
docker build -f Dockerfile.traefik -t traefik-jwt-decoder:v0.1.0 .

# Tag for your registry
docker tag traefik-jwt-decoder:v0.1.0 myregistry.com/traefik-jwt-decoder:v0.1.0
```

**2. Push to Private Registry**

```bash
# For air-gapped environments, push to internal registry
docker push myregistry.com/traefik-jwt-decoder:v0.1.0
```

**3. Deploy with Docker Compose**

```yaml
version: '3.8'

services:
  traefik:
    image: myregistry.com/traefik-jwt-decoder:v0.1.0
    command:
      - "--providers.docker=true"
      - "--providers.file.directory=/etc/traefik/dynamic"
      - "--experimental.localPlugins.traefik-jwt-decoder-plugin.moduleName=github.com/user/traefik-jwt-decoder-plugin"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "./dynamic-config.yml:/etc/traefik/dynamic/config.yml:ro"
```

### Dockerfile Explained

```dockerfile
FROM traefik:v3.0

# Create plugin directory structure
# Traefik expects: /plugins-local/src/<module-path>
RUN mkdir -p /plugins-local/src/github.com/user/traefik-jwt-decoder-plugin

# Copy plugin source into image
COPY . /plugins-local/src/github.com/user/traefik-jwt-decoder-plugin/

# Plugin is now embedded - no internet needed at runtime
```

**Key Points:**
- Plugin source copied into image at build time
- Path matches module name: `github.com/jander99/traefik-jwt-decoder-plugin`
- Use `--experimental.localPlugins` flag (not `--experimental.plugins`)
- No Git clone happens at runtime

---

## Air-Gapped Environments

For environments without internet access (DMZ, classified networks, etc.).

### Prerequisites

**On Connected Machine:**
1. Git access to plugin repository
2. Docker installed
3. Access to push to internal registry

**On Air-Gapped Machine:**
1. Access to internal registry
2. Docker installed

### Deployment Process

**Step 1: Build on Connected Machine**

```bash
# Clone repository
git clone https://github.com/user/traefik-jwt-decoder-plugin.git
cd traefik-jwt-decoder-plugin

# Build image
docker build -f Dockerfile.traefik -t traefik-jwt-decoder:v0.1.0 .

# Save image to tar file
docker save traefik-jwt-decoder:v0.1.0 -o traefik-jwt-decoder-v0.1.0.tar
```

**Step 2: Transfer to Air-Gapped Environment**

```bash
# Transfer via approved method (USB, secure file transfer, etc.)
# Example using scp (if allowed):
scp traefik-jwt-decoder-v0.1.0.tar airgapped-host:/tmp/
```

**Step 3: Load on Air-Gapped Machine**

```bash
# Load image from tar
docker load -i /tmp/traefik-jwt-decoder-v0.1.0.tar

# Tag for internal registry
docker tag traefik-jwt-decoder:v0.1.0 internal-registry.local/traefik-jwt-decoder:v0.1.0

# Push to internal registry
docker push internal-registry.local/traefik-jwt-decoder:v0.1.0
```

**Step 4: Deploy**

```bash
# Update docker-compose.yml to use internal registry
sed -i 's|traefik-jwt-decoder:latest|internal-registry.local/traefik-jwt-decoder:v0.1.0|' docker-compose.production.yml

# Deploy
docker-compose -f docker-compose.production.yml up -d
```

### Verification

```bash
# Check plugin loaded
docker logs <container> | grep "plugin"

# Should see: "Starting provider *plugin.Provider"

# Test plugin functionality
curl -H "Authorization: Bearer <JWT>" http://whoami.localhost
```

---

## Kubernetes Deployment

Deploy Traefik with JWT decoder plugin on Kubernetes.

### Helm Values

```yaml
# values.yaml for Traefik Helm chart
image:
  name: myregistry.com/traefik-jwt-decoder
  tag: v0.1.0

deployment:
  initContainers: []  # Not needed - plugin already in image

additionalArguments:
  - "--experimental.localPlugins.traefik-jwt-decoder-plugin.moduleName=github.com/jander99/traefik-jwt-decoder-plugin"
  - "--providers.file.directory=/etc/traefik/dynamic"

volumes:
  - name: dynamic-config
    configMap:
      name: traefik-dynamic-config
    mountPath: /etc/traefik/dynamic

providers:
  file:
    directory: /etc/traefik/dynamic
```

### ConfigMap for Dynamic Config

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: traefik-dynamic-config
  namespace: traefik
data:
  jwt-decoder.yml: |
    http:
      middlewares:
        jwt-decoder:
          plugin:
            traefik-jwt-decoder-plugin:
              sourceHeader: "Authorization"
              tokenPrefix: "Bearer "
              claims:
                - claimPath: "sub"
                  headerName: "X-User-Id"
                - claimPath: "email"
                  headerName: "X-User-Email"
              continueOnError: false
              maxClaimDepth: 10
              maxHeaderSize: 8192
```

### Deployment

```bash
# Install Traefik with custom image
helm install traefik traefik/traefik \
  --namespace traefik \
  --create-namespace \
  -f values.yaml

# Apply dynamic config
kubectl apply -f traefik-dynamic-config.yaml

# Verify
kubectl get pods -n traefik
kubectl logs -n traefik <traefik-pod> | grep plugin
```

### IngressRoute Example

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: my-app
  namespace: default
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`app.example.com`)
      kind: Rule
      services:
        - name: my-app-service
          port: 80
      middlewares:
        - name: jwt-decoder
          namespace: traefik
```

---

## Configuration Management

### Environment-Specific Configs

**Development:**
```yaml
continueOnError: true   # Allow requests without JWT
maxClaimDepth: 10
maxHeaderSize: 8192
```

**Staging:**
```yaml
continueOnError: true   # Log errors but continue
maxClaimDepth: 10
maxHeaderSize: 4096     # Stricter limits
```

**Production:**
```yaml
continueOnError: false  # Fail-closed
maxClaimDepth: 5        # Stricter depth limit
maxHeaderSize: 4096     # Stricter size limit
```

### Secret Management

**Never commit secrets to configuration files.** Use environment variables or secret management tools.

```yaml
# BAD - Don't do this
tokenPrefix: "Bearer MySecretToken"

# GOOD - Use environment variables
command:
  - "--experimental.localPlugins.traefik-jwt-decoder-plugin.moduleName=${PLUGIN_MODULE}"
```

For Kubernetes, use Secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: traefik-jwt-config
type: Opaque
stringData:
  source-header: "Authorization"
  token-prefix: "Bearer "
```

---

## Monitoring and Observability

### Logging

**Enable structured logging:**

```yaml
# Traefik static config
log:
  level: INFO
  format: json
  filePath: /var/log/traefik/traefik.log

accessLog:
  filePath: /var/log/traefik/access.log
  format: json
  fields:
    defaultMode: keep
    headers:
      defaultMode: keep
      names:
        X-User-Id: keep
        X-User-Email: keep
```

**Plugin logs include:**
- `[plugin-name] JWT source header not found`
- `[plugin-name] JWT parse error`
- `[plugin-name] Claim not found`
- `[plugin-name] Injected header: X-User-Id = 12345`

### Metrics

Monitor these metrics:

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| HTTP 401 rate | Failed JWT validation | >5% of requests |
| Plugin processing time | Middleware latency | >50ms p99 |
| Header injection errors | Failed header operations | >0 errors/min |
| JWT parse errors | Malformed tokens | >1% of requests |

### Health Checks

```yaml
# Docker Compose
healthcheck:
  test: ["CMD", "traefik", "healthcheck", "--ping"]
  interval: 10s
  timeout: 3s
  retries: 3
  start_period: 30s
```

### Troubleshooting

**Plugin not loading:**
```bash
# Check plugin path
docker exec <container> ls -la /plugins-local/src/github.com/jander99/

# Check logs
docker logs <container> 2>&1 | grep -i plugin
```

**JWT not extracted:**
```bash
# Enable debug logging
--log.level=DEBUG

# Check source header
docker logs <container> | grep "JWT source header"
```

**Headers not injected:**
```bash
# Check for protected header attempts
docker logs <container> | grep "protected header"

# Verify claim path
docker logs <container> | grep "Claim not found"
```

---

## Security Considerations

### Production Checklist

- [ ] Set `continueOnError: false` for fail-closed behavior
- [ ] Configure strict resource limits (maxClaimDepth, maxHeaderSize)
- [ ] Deploy behind API gateway with JWT signature verification
- [ ] Enable TLS for all external connections
- [ ] Implement rate limiting
- [ ] Set up log aggregation and alerting
- [ ] Regular security updates (rebuild image with latest Traefik base)
- [ ] Network segmentation (internal services only)
- [ ] Monitor for anomalous claim patterns

### Network Architecture

**Recommended:**
```
Internet → API Gateway (JWT verification) → Traefik + Plugin → Internal Services
           ✓ Signature check            ✓ Claim extraction   ✓ Header access
```

**Never expose directly:**
```
Internet → Traefik + Plugin (NO VERIFICATION) → Services  ❌ INSECURE
```

---

## Performance Optimization

### Image Size Optimization

```dockerfile
# Multi-stage build for smaller image
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go mod download
# Validate Go syntax (optional)
RUN go build -o /dev/null ./...

FROM traefik:v3.0-alpine
COPY --from=builder /build /plugins-local/src/github.com/jander99/traefik-jwt-decoder-plugin/
```

### Caching Strategy

```bash
# Use BuildKit for better caching
DOCKER_BUILDKIT=1 docker build -f Dockerfile.traefik -t traefik-jwt-decoder:v0.1.0 .
```

### Resource Limits

```yaml
# Docker Compose
services:
  traefik:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

---

## Versioning and Updates

### Semantic Versioning

```bash
# Build specific version
docker build -f Dockerfile.traefik -t traefik-jwt-decoder:v0.1.0 .

# Tag as latest (for convenience)
docker tag traefik-jwt-decoder:v0.1.0 traefik-jwt-decoder:latest

# Tag for production
docker tag traefik-jwt-decoder:v0.1.0 traefik-jwt-decoder:production
```

### Update Strategy

1. Build new version: `v0.2.0`
2. Test in staging environment
3. Deploy to production with blue-green or canary deployment
4. Monitor for errors
5. Rollback if needed: `docker tag traefik-jwt-decoder:v0.1.0 traefik-jwt-decoder:production`

---

## Support and Maintenance

For issues or questions:
- GitHub Issues: https://github.com/user/traefik-jwt-decoder-plugin/issues
- Security Issues: See [SECURITY.md](SECURITY.md)
- Documentation: See [README.md](../README.md)

