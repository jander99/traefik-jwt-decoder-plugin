# Traefik JWT Decoder Plugin - Docker Testing Environment

This directory contains a complete Docker Compose setup for manually testing the JWT decoder plugin with Traefik.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose 1.29+ or Docker Compose V2
- curl or similar HTTP client

## Quick Start

```bash
# Start the environment
docker-compose up -d

# Wait for Traefik to initialize (5-10 seconds)
sleep 5

# Run automated tests
./test-plugin.sh

# Stop the environment
docker-compose down
```

## What's Included

- **Traefik v3.0**: Reverse proxy with local plugin support
- **Whoami Service**: Echo service to inspect injected headers
- **Dynamic Configuration**: JWT decoder middleware configuration
- **Test Script**: Automated testing scenarios

## Manual Testing

### Test with Valid JWT Token

The test JWT contains:
- `sub`: 1234567890
- `email`: test@example.com
- `roles`: ["admin", "user"]
- `custom.tenant_id`: tenant-123

```bash
JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

curl -H "Authorization: Bearer $JWT_TOKEN" http://whoami.localhost
```

**Expected Headers:**
- `X-User-Id: 1234567890`
- `X-User-Email: test@example.com`
- `X-User-Roles: admin, user`
- `X-Tenant-Id: tenant-123`

### Test without JWT (continueOnError)

```bash
curl http://whoami.localhost
```

Should return 200 OK without injected headers (continueOnError=true).

### Test with Invalid JWT

```bash
curl -H "Authorization: Bearer invalid.token.here" http://whoami.localhost
```

Should return 200 OK without injected headers (continueOnError=true).

### Test without Bearer Prefix

```bash
curl -H "Authorization: $JWT_TOKEN" http://whoami.localhost
```

Should return 200 OK without injected headers (missing "Bearer " prefix).

## Viewing Results

### Traefik Dashboard

Visit http://localhost:8080 to access the Traefik dashboard:
- View active middlewares (jwt-decoder should be listed)
- Check router configurations
- Monitor HTTP traffic

### Traefik Logs

```bash
# View all logs
docker-compose logs traefik

# Follow logs in real-time
docker-compose logs -f traefik

# Filter for plugin-related logs
docker-compose logs traefik | grep -i "jwt\|plugin\|error"
```

### Whoami Service Logs

```bash
docker-compose logs whoami
```

## Configuration

Edit `dynamic-config.yml` to modify plugin behavior:

```yaml
http:
  middlewares:
    jwt-decoder:
      plugin:
        traefik-jwt-decoder-plugin:
          sourceHeader: "Authorization"      # Header to extract JWT from
          tokenPrefix: "Bearer "             # Prefix to strip
          claims:                            # Claim mappings
            - claimPath: "sub"
              headerName: "X-User-Id"
            - claimPath: "email"
              headerName: "X-User-Email"
            - claimPath: "roles"
              headerName: "X-User-Roles"
              arrayFormat: "comma"           # Format arrays as comma-separated
            - claimPath: "custom.tenant_id"  # Nested claim with dot notation
              headerName: "X-Tenant-Id"
          sections:
            - "payload"                      # Read from JWT payload only
          continueOnError: true              # Don't fail on JWT errors
          maxClaimDepth: 10                  # Max nesting depth
          maxHeaderSize: 8192                # Max header value size
```

After modifying, restart Traefik:

```bash
docker-compose restart traefik
```

## Troubleshooting

### Plugin Not Loading

**Symptom**: Traefik logs show plugin not found

**Solution**:
```bash
# Check volume mount
docker-compose exec traefik ls -la /plugins-local/src/github.com/user/traefik-jwt-decoder-plugin

# Verify go.mod exists
docker-compose exec traefik cat /plugins-local/src/github.com/user/traefik-jwt-decoder-plugin/go.mod

# Restart with clean state
docker-compose down -v
docker-compose up -d
```

### Headers Not Injected

**Symptom**: No X-* headers appear in whoami output

**Solution**:
1. Check Traefik logs for JWT parsing errors
2. Verify JWT token format (should be three base64url segments)
3. Ensure "Bearer " prefix is included
4. Check dynamic-config.yml syntax

### Connection Refused

**Symptom**: `curl: (7) Failed to connect to whoami.localhost`

**Solution**:
1. Ensure Docker Compose is running: `docker-compose ps`
2. Check if port 80 is available: `sudo lsof -i :80`
3. Try `http://localhost` or `http://127.0.0.1` with Host header:
   ```bash
   curl -H "Host: whoami.localhost" -H "Authorization: Bearer $JWT_TOKEN" http://localhost
   ```

### Traefik Dashboard Not Accessible

**Symptom**: Cannot access http://localhost:8080

**Solution**:
1. Check if port 8080 is in use: `sudo lsof -i :8080`
2. Verify Traefik container is running: `docker-compose ps traefik`
3. Check Traefik logs: `docker-compose logs traefik`

## Testing Workflow

1. **Start Environment**
   ```bash
   docker-compose up -d
   sleep 5
   ```

2. **Verify Plugin Load**
   ```bash
   docker-compose logs traefik | grep -i "plugin\|jwt"
   ```
   Look for: "Loading plugin traefik-jwt-decoder-plugin"

3. **Run Automated Tests**
   ```bash
   ./test-plugin.sh
   ```

4. **Check Dashboard**
   - Visit http://localhost:8080
   - Navigate to HTTP > Middlewares
   - Confirm `jwt-decoder@file` is present

5. **Manual Testing**
   - Use curl commands above
   - Inspect injected headers in whoami output

6. **Review Logs**
   ```bash
   docker-compose logs | grep -i error
   ```

7. **Clean Up**
   ```bash
   docker-compose down
   ```

## Development Workflow

When modifying plugin code:

1. Make code changes in parent directory
2. Restart Traefik to reload plugin:
   ```bash
   docker-compose restart traefik
   ```
3. Wait 5 seconds for initialization
4. Run tests to verify changes

**Note**: No rebuild required - plugin is mounted as a volume.

## JWT Token Details

The test token is an HS256 JWT (secret: `your-256-bit-secret`) with:

**Header:**
```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

**Payload:**
```json
{
  "sub": "1234567890",
  "email": "test@example.com",
  "roles": ["admin", "user"],
  "custom": {
    "tenant_id": "tenant-123"
  },
  "iat": 1516239022
}
```

Generate your own JWT at https://jwt.io

## Additional Resources

- [Traefik Local Plugins Documentation](https://doc.traefik.io/traefik/plugins/overview/)
- [Plugin Development Guide](https://doc.traefik.io/traefik/plugins/development/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
