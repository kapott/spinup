# security-review

Scan code for security vulnerabilities, credential leaks, and injection risks.

## Trigger
User invokes `/security-review` or asks for a security audit.

## Instructions

1. **Scan for hardcoded credentials**:
   ```bash
   grep -rn "API_KEY\|api_key\|apikey\|secret\|password\|token" --include="*.go" . | grep -v "_test.go" | grep -v "example"
   ```

2. **Check for credential patterns**:
   - AWS keys: `AKIA[0-9A-Z]{16}`
   - Generic API keys: Long base64 or hex strings in code
   - Private keys: `-----BEGIN.*PRIVATE KEY-----`

3. **Review .gitignore**:
   - `.env` must be ignored
   - `.continueplz.state` must be ignored
   - `*.pem`, `*.key` should be ignored
   - Coverage files (`coverage.out`) should be ignored

4. **Check file permissions handling**:
   - `.env` file should be 0600
   - WireGuard configs should be 0600
   - State file should be 0600

5. **Review command injection risks**:
   - Look for `exec.Command` with user input
   - Check `os/exec` usage
   - Verify arguments are properly escaped

6. **Review SQL/NoSQL injection** (if applicable):
   - Parameterized queries used
   - No string concatenation in queries

7. **Check input validation**:
   - API key format validation
   - Instance ID format validation
   - Model name validation
   - Region name validation

8. **Review error messages**:
   - No credentials in error messages
   - No full stack traces to users
   - Sensitive paths not exposed

9. **Check TLS/certificate handling**:
   - No `InsecureSkipVerify: true` in production
   - Certificate validation enabled

10. **Review logging**:
    - No credentials logged
    - API keys redacted in logs
    - Request/response bodies sanitized

## Output Format

```
## Security Review Report

### Critical Issues
None found / [List of critical issues]

### High Severity
1. [FILE:LINE] Hardcoded API key detected
   - Risk: Credential exposure in version control
   - Fix: Move to environment variable

### Medium Severity
1. [FILE:LINE] TLS verification disabled
   - Risk: Man-in-the-middle attacks
   - Fix: Enable certificate verification

### Low Severity
1. [FILE:LINE] Verbose error message may leak internal paths
   - Risk: Information disclosure
   - Fix: Sanitize error before returning to user

### Recommendations
- [ ] Add .env to .gitignore
- [ ] Implement credential redaction in logger
- [ ] Add input validation for model names

### Files Reviewed
- internal/config/config.go
- internal/provider/vast/client.go
- ...
```
