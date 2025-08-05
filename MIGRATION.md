# Migration Guide: Single Tenant to Multi-Tenant

This guide helps you migrate from the old single-tenant configuration to the new multi-tenant YAML configuration.

## Quick Migration

### If you currently use environment variables:

**Old way (still works):**
```bash
export CMS_BASE_URL="https://cms.strandnerd.com"
export ACCESS_TOKEN="your-access-token"
```

**New way (recommended):**
```bash
# Copy the example
cp tenants.yml.example tenants.yml

# Edit tenants.yml
```

```yaml
tenants:
  - id: "main"
    name: "Main Production"
    cms_base_url: "https://cms.strandnerd.com"
    access_token: "your-access-token"
    enabled: true
```

## Migration Steps

1. **Backup your current configuration**
   ```bash
   # If using .env file
   cp .env .env.backup
   
   # If using environment variables, document them
   env | grep -E "(CMS_BASE_URL|ACCESS_TOKEN)" > env.backup
   ```

2. **Create YAML configuration**
   ```bash
   cp tenants.yml.example tenants.yml
   ```

3. **Transfer your settings**
   Edit `tenants.yml` and replace the example values with your actual:
   - `cms_base_url`: Your CMS_BASE_URL value
   - `access_token`: Your ACCESS_TOKEN value
   - Choose a meaningful `id` and `name` for your tenant

4. **Test the new configuration**
   ```bash
   ./crawler -help  # Should show new tenant options
   ./crawler -once  # Should show "Loaded configuration for 1 tenant(s)"
   ```

5. **Remove old environment variables (optional)**
   Once confirmed working, you can remove the old environment variables.

## What Changed

### Command Line Options
- **New:** `-tenant <id>` option to run for specific tenant
- **Same:** `-once`, `-feed`, `-interval` work as before
- **Enhanced:** Better logging shows which tenant each operation relates to

### Configuration Priority
1. YAML file (`tenants.yml`) - **highest priority**
2. Environment variables - **fallback for compatibility**
3. Default values - **lowest priority**

### Backward Compatibility
- ✅ Existing environment variable setups continue to work
- ✅ No breaking changes to command line options
- ✅ Same Docker deployment process
- ✅ Same API endpoints and authentication

## Benefits of Migration

1. **Multi-tenant support**: Manage multiple CMS instances from one crawler
2. **Better organization**: Clear configuration in YAML format
3. **Environment safety**: Keep sensitive tokens out of environment variables
4. **Easier deployment**: Copy example file instead of setting many env vars
5. **Selective operation**: Run crawls for specific tenants only

## Rollback Plan

If you need to rollback:

1. **Keep using environment variables**: The old method still works
2. **Remove YAML file**: Delete `tenants.yml` to fall back to env vars
3. **Use old commands**: All existing command line usage continues to work

## Support

- Environment variable configuration remains fully supported
- Both methods can be used during transition period
- No rush to migrate - do it when convenient
