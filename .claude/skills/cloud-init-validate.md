# cloud-init-validate

Validate cloud-init YAML syntax and check for common issues.

## Trigger
User invokes `/cloud-init-validate [file]` or asks to validate cloud-init config.

## Arguments
- `[file]` - Path to cloud-init file (default: generated from template)

## Instructions

1. **Parse YAML syntax**:
   ```bash
   python3 -c "import yaml; yaml.safe_load(open('[file]'))" 2>&1
   ```

2. **Validate cloud-init schema** (if cloud-init installed):
   ```bash
   cloud-init schema --config-file [file] 2>&1
   ```

3. **Check required sections**:
   - `#cloud-config` header present
   - `packages` list is valid
   - `write_files` entries have required fields (path, content, permissions)
   - `runcmd` commands are valid shell

4. **Validate write_files entries**:
   - Path is absolute
   - Permissions are valid octal (e.g., '0600', '0755')
   - Content doesn't have unsubstituted template variables

5. **Validate runcmd entries**:
   - Commands don't reference undefined variables
   - Commands use absolute paths or are in PATH
   - Proper quoting for variables

6. **Check template variables**:
   - All `{{ .Variable }}` placeholders have corresponding data
   - No unescaped special characters

7. **Security checks**:
   - Private keys not exposed in logs
   - Sensitive data has restricted permissions
   - API keys use environment variables not hardcoded

8. **Simulate template rendering** (if template):
   - Render with sample data
   - Verify output is valid YAML

## Output Format

```
## Cloud-init Validation Report

### Syntax
- YAML: [VALID/INVALID]
- Schema: [VALID/INVALID/SKIPPED]

### Structure
- [x] Header present (#cloud-config)
- [x] packages: 4 packages defined
- [x] write_files: 3 files defined
- [x] runcmd: 15 commands defined

### Issues
1. [WARN] write_files[0]: permissions '600' should be '0600'
2. [ERROR] runcmd[5]: unsubstituted variable {{ .APIKey }}
3. [WARN] Sensitive file /etc/wireguard/wg0.conf should have 0600 permissions

### Template Variables
Required: .WireGuard.ServerPrivateKey, .WireGuard.ClientPublicKey, .Model, ...
Missing: None

### Security
- [x] No hardcoded API keys
- [x] Private keys protected
- [WARN] Consider encrypting sensitive runcmd content
```
