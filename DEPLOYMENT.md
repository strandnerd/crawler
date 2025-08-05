# Multi-tenant Deployment Guide

This guide explains how to deploy the crawler with multi-tenant configuration.

## Configuration Files

### Development

1. Copy the example configuration:
   ```bash
   cp tenants.yml.example tenants.yml
   ```

2. Edit `tenants.yml` with your tenant configurations
3. The `tenants.yml` file is git-ignored for security

### Production Deployment

#### Option 1: Manual Configuration (Recommended for Production)

1. **On the production server**, create the tenants configuration file directly:
   ```bash
   cd /opt/strandnerd-crawler
   
   # Create tenants.yml from the example (without replacing existing file)
   if [ ! -f tenants.yml ]; then
       cp tenants.yml.example tenants.yml
       echo "Created tenants.yml from example. Please edit with your production settings."
   else
       echo "tenants.yml already exists. Not overwriting."
   fi
   ```

2. Edit the production `tenants.yml` file with your actual tenant configurations:
   ```bash
   nano tenants.yml
   ```

3. **Important**: The Docker Compose configuration automatically binds the local `tenants.yml` file into the container:
   ```yaml
   volumes:
     - ./tenants.yml:/app/tenants.yml:ro
   ```
   This means:
   - The container will use your local `tenants.yml` file directly
   - Changes to `tenants.yml` require a container restart to take effect
   - The file must exist before starting the container
   - The file is mounted read-only (`:ro`) for security

#### Option 2: Environment Variables (Legacy)

If you prefer to use environment variables instead of YAML:

1. Set environment variables directly or use a `.env` file:
   ```bash
   # Single tenant
   export CMS_BASE_URL="https://cms.strandnerd.com"
   export ACCESS_TOKEN="your-production-token"
   
   # Or multi-tenant
   export TENANT_1_CMS_BASE_URL="https://cms.strandnerd.com"
   export TENANT_1_ACCESS_TOKEN="your-main-token"
   export TENANT_2_CMS_BASE_URL="https://dev-cms.strandnerd.com"
   export TENANT_2_ACCESS_TOKEN="your-dev-token"
   ```

#### Option 3: Docker Secrets (Advanced)

For containerized deployments with secrets management:

1. Store tenant configuration as Docker secrets
2. Mount secrets into the container
3. Use init scripts to generate `tenants.yml` from secrets

## File Safety

The deployment process ensures:

- ✅ `tenants.yml.example` is always updated with new deployments
- ✅ Existing `tenants.yml` is never overwritten automatically
- ✅ Production configurations remain safe during updates
- ✅ New installations get a template to work from

## Security Best Practices

1. **Never commit `tenants.yml`** - it's in `.gitignore` for security
2. **Use strong access tokens** with minimal required permissions
3. **Regularly rotate access tokens** and update configurations
4. **Use environment-specific URLs** (dev, staging, prod)
5. **Monitor tenant access** and disable unused tenants

## Deployment Checklist

- [ ] Copy `tenants.yml.example` to `tenants.yml` if not exists
- [ ] Configure tenant URLs and access tokens
- [ ] Test tenant connectivity: `./crawler -once -tenant <id>`
- [ ] Verify all tenants are working: `./crawler -once`
- [ ] Set up monitoring for multi-tenant operations
- [ ] Document tenant-specific settings and responsibilities

## Troubleshooting

### Tenant Not Found
```
Tenant 'xyz' not found or not enabled
```
- Check tenant ID matches exactly in `tenants.yml`
- Verify tenant is marked as `enabled: true`

### Configuration File Issues
```
no such file or directory: tenants.yml
```
- Ensure `tenants.yml` exists in the same directory as `docker-compose.yml`
- Copy from example: `cp tenants.yml.example tenants.yml`
- Verify file permissions are readable

### Volume Binding Issues
```
Error: failed to mount local volume
```
- Check that `tenants.yml` exists before starting container
- Verify Docker has permission to read the file
- Use absolute paths if relative paths fail

### Configuration Not Loading
```
Loaded configuration for 0 tenant(s)
```
- Check YAML syntax with `docker-compose config`
- Verify file is properly mounted: `docker exec container cat /app/tenants.yml`
- Check container logs for parsing errors

### Configuration Priority
The crawler loads configuration in this order:
1. YAML file (`tenants.yml`)
2. Environment variables (legacy fallback)
3. Default values

### Checking Configuration
```bash
# Test configuration loading
./crawler -help

# Test specific tenant
./crawler -once -tenant main

# Test all tenants
./crawler -once
```
