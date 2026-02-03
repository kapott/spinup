# cross-compile

Build continueplz for all target platforms (Linux/macOS, amd64/arm64).

## Trigger
User invokes `/cross-compile` or asks to build for multiple platforms.

## Arguments
- `[version]` - Optional version tag (default: "dev")
- `--release` - Build optimized release binaries

## Instructions

1. **Verify Go installation and version**:
   ```bash
   go version
   ```

2. **Set build variables**:
   ```bash
   VERSION="${VERSION:-dev}"
   GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
   BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
   ```

3. **Build for all platforms**:

   ```bash
   # Create output directory
   mkdir -p dist

   # Build matrix
   PLATFORMS=(
       "linux/amd64"
       "linux/arm64"
       "darwin/amd64"
       "darwin/arm64"
   )

   LDFLAGS="-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}"

   for platform in "${PLATFORMS[@]}"; do
       GOOS="${platform%/*}"
       GOARCH="${platform#*/}"
       output="dist/continueplz-${VERSION}-${GOOS}-${GOARCH}"

       echo "Building ${GOOS}/${GOARCH}..."
       GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "$LDFLAGS" -o "$output" ./cmd/continueplz
   done
   ```

4. **Create checksums**:
   ```bash
   cd dist && sha256sum continueplz-* > checksums.txt
   ```

5. **Verify builds**:
   ```bash
   # Check file sizes are reasonable
   ls -lh dist/

   # Verify native binary runs
   ./dist/continueplz-*-$(go env GOOS)-$(go env GOARCH) --version
   ```

6. **Create archives for release**:
   ```bash
   for f in dist/continueplz-${VERSION}-*; do
       if [[ "$f" != *.tar.gz ]] && [[ "$f" != *.zip ]]; then
           tar -czvf "${f}.tar.gz" -C dist "$(basename $f)"
       fi
   done
   ```

## Output Format

```
## Cross-Compilation Report

### Build Info
- Version: v1.0.0
- Commit: abc1234
- Time: 2026-02-02T10:30:00Z

### Binaries
| Platform      | Architecture | Size    | Status |
|---------------|--------------|---------|--------|
| linux         | amd64        | 12.3 MB | OK     |
| linux         | arm64        | 11.8 MB | OK     |
| darwin        | amd64        | 12.1 MB | OK     |
| darwin        | arm64        | 11.9 MB | OK     |

### Checksums (SHA256)
abc123... continueplz-v1.0.0-linux-amd64
def456... continueplz-v1.0.0-linux-arm64
...

### Output Directory
dist/
├── continueplz-v1.0.0-linux-amd64
├── continueplz-v1.0.0-linux-amd64.tar.gz
├── continueplz-v1.0.0-linux-arm64
├── continueplz-v1.0.0-linux-arm64.tar.gz
├── continueplz-v1.0.0-darwin-amd64
├── continueplz-v1.0.0-darwin-amd64.tar.gz
├── continueplz-v1.0.0-darwin-arm64
├── continueplz-v1.0.0-darwin-arm64.tar.gz
└── checksums.txt
```
