# Tick-Storm Versioning Plan
## Comprehensive Versioning Strategy Document

**Version:** 1.0.0  
**Date:** 2025-08-16  
**Status:** APPROVED  

---

## 1. Semantic Versioning Strategy (MAJOR.MINOR.PATCH)

### Version Format
The project follows **Semantic Versioning 2.0.0** specification:
```
MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

### Version Increment Rules

#### MAJOR Version (X.0.0)
Increment when making **incompatible API/protocol changes**:
- Breaking changes to the binary frame format
- Incompatible changes to Protobuf message schemas
- Removal of supported message types
- Changes to authentication mechanism
- Incompatible configuration changes

#### MINOR Version (0.X.0)
Increment when adding **backward-compatible functionality**:
- New message types (with backward compatibility)
- New optional fields in Protobuf messages
- New configuration options with defaults
- Performance improvements
- New metrics or observability features

#### PATCH Version (0.0.X)
Increment for **backward-compatible bug fixes**:
- Security patches
- Bug fixes
- Documentation updates
- Minor performance optimizations
- Dependency updates (non-breaking)

### Pre-release Versions
Format: `MAJOR.MINOR.PATCH-PRERELEASE`

Examples:
- `1.0.0-alpha.1` - Alpha release
- `1.0.0-beta.2` - Beta release  
- `1.0.0-rc.3` - Release candidate

Pre-release precedence:
```
alpha < beta < rc < (no suffix/stable)
```

### Build Metadata
Format: `MAJOR.MINOR.PATCH+BUILD`

Examples:
- `1.0.0+20250816` - Date-based build
- `1.0.0+sha.a1b2c3d` - Git commit SHA
- `1.0.0+ci.123` - CI build number

---

## 2. Protocol Version Management

### Current Protocol Version
- **Version:** `0x01` (stored in frame header Ver field)
- **Location:** Byte position 2 in frame header

### Protocol Version Increment Strategy

#### When to Increment Protocol Version:
1. **Frame format changes** (header structure, field positions)
2. **Incompatible Protobuf schema changes** (field type changes, required field removal)
3. **Checksum algorithm changes** (e.g., CRC32C to different algorithm)
4. **Magic byte changes**

#### When NOT to Increment Protocol Version:
1. Adding new message types (backward compatible)
2. Adding optional Protobuf fields
3. Performance optimizations
4. Bug fixes

### Protocol Compatibility Matrix

| Server Protocol | Client Protocol | Compatibility |
|----------------|-----------------|---------------|
| 0x01           | 0x01            | ✅ Full       |
| 0x02           | 0x01            | ⚠️ Limited    |
| 0x01           | 0x02            | ❌ None       |

---

## 3. Backward Compatibility Policy

### Guarantees
- **Within MAJOR version:** Full backward compatibility guaranteed
- **Across MAJOR versions:** Best-effort compatibility, not guaranteed
- **Protocol compatibility:** Minimum 2 MINOR versions support

### Compatibility Rules
1. **Never remove or change existing Protobuf field numbers**
2. **Use `reserved` keyword for deprecated fields**
3. **New features must be optional with sensible defaults**
4. **Configuration changes must maintain old defaults**
5. **Error codes must remain consistent**

### Testing Requirements
- Maintain compatibility test suite
- Test against last 3 MINOR versions
- Document breaking changes in CHANGELOG.md

---

## 4. Version Branching Strategy

### Branch Naming Convention
```
main                    # Latest stable release
develop                 # Integration branch
release/v{MAJOR}.{MINOR} # Release branches
hotfix/v{VERSION}       # Hotfix branches
feature/{ticket-id}     # Feature branches
```

### Branching Workflow

#### Feature Development
```
develop → feature/FSD-XXX → develop
```

#### Release Process
```
develop → release/v1.2 → main (tag: v1.2.0)
                      ↘ develop (merge back)
```

#### Hotfix Process
```
main → hotfix/v1.2.1 → main (tag: v1.2.1)
                    ↘ develop (merge back)
```

### Long-term Support (LTS)
- Every 3rd MINOR version becomes LTS
- LTS versions receive security patches for 12 months
- Example: v1.3.x, v1.6.x, v1.9.x are LTS

---

## 5. Release Tagging Conventions

### Git Tag Format
```
v{MAJOR}.{MINOR}.{PATCH}[-PRERELEASE]
```

### Tagging Rules
1. **Production releases:** `v1.2.3`
2. **Pre-releases:** `v1.2.3-rc.1`
3. **Nightly builds:** Not tagged, use commit SHA
4. **Signed tags:** Required for production releases

### Tag Annotations
Include in tag message:
- Release date
- Brief summary of changes
- Link to full changelog

Example:
```bash
git tag -s v1.2.3 -m "Release v1.2.3 (2025-08-16)

Highlights:
- Improved connection handling
- Fixed memory leak in subscription manager
- Added new metrics

Full changelog: https://github.com/tick-storm/releases/v1.2.3"
```

---

## 6. Breaking Change Communication Process

### Timeline
1. **T-60 days:** Announce deprecation in documentation
2. **T-30 days:** Add deprecation warnings to logs
3. **T-14 days:** Send notification to registered users
4. **T-7 days:** Final reminder
5. **T-0:** Release with breaking change

### Communication Channels
- GitHub Release Notes
- CHANGELOG.md
- Server startup logs
- Runtime deprecation warnings
- Email notifications (for registered deployments)

### Deprecation Message Format
```
[DEPRECATION] Feature X will be removed in v2.0.0 (target date: 2025-10-01).
Migration guide: https://docs.tick-storm.io/migration/v2
Alternative: Use feature Y instead.
```

---

## 7. Version Numbering for Pre-release Builds

### Development Builds
Format: `{MAJOR}.{MINOR}.{PATCH}-dev.{TIMESTAMP}+{COMMIT}`

Example: `1.2.3-dev.20250816143022+a1b2c3d`

### CI/CD Builds
Format: `{MAJOR}.{MINOR}.{PATCH}-ci.{BUILD_NUMBER}`

Example: `1.2.3-ci.456`

### Nightly Builds
Format: `{MAJOR}.{MINOR}.{PATCH}-nightly.{DATE}`

Example: `1.2.3-nightly.20250816`

### Pull Request Builds
Format: `{MAJOR}.{MINOR}.{PATCH}-pr.{PR_NUMBER}.{BUILD}`

Example: `1.2.3-pr.789.1`

---

## 8. Version Information in Runtime

### Version Exposure
1. **Binary version:** `./tick-storm --version`
2. **HTTP endpoint:** `GET /version`
3. **Metrics:** `tick_storm_build_info` gauge
4. **Logs:** Printed on startup

### Version Struct
```go
type Version struct {
    Semantic  string `json:"semantic"`   // e.g., "1.2.3"
    Protocol  uint8  `json:"protocol"`   // e.g., 0x01
    GitCommit string `json:"git_commit"` // e.g., "a1b2c3d"
    BuildTime string `json:"build_time"` // e.g., "2025-08-16T14:30:00Z"
    GoVersion string `json:"go_version"` // e.g., "1.22.0"
}
```

---

## 9. Changelog Management

### CHANGELOG.md Format
Follow [Keep a Changelog](https://keepachangelog.com/) format:

```markdown
## [Unreleased]

## [1.2.3] - 2025-08-16
### Added
- New feature X

### Changed
- Improved Y performance

### Deprecated
- Feature Z (removal in v2.0.0)

### Removed
- Legacy feature W

### Fixed
- Bug in connection handling

### Security
- Patched CVE-2025-XXXX
```

---

## 10. Implementation Checklist

- [x] Document versioning strategy
- [x] Define protocol version rules
- [x] Establish backward compatibility policy
- [x] Document branching strategy
- [x] Define release tagging conventions
- [x] Establish breaking change communication process
- [x] Define pre-release version numbering
- [ ] Implement version info endpoint
- [ ] Set up automated changelog generation
- [ ] Configure CI/CD for version tagging
- [ ] Create compatibility test suite

---

## Approval

This versioning plan has been reviewed and approved by:

- **Technical Lead:** Furkan SARIKAYA (2025-08-16)
- **Project Manager:** Approved via Linear FSD-6
- **DevOps Team:** Pending CI/CD implementation

---

## References

- [Semantic Versioning 2.0.0](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Protocol Buffers Versioning](https://developers.google.com/protocol-buffers/docs/proto3#updating)
- [Git Flow](https://nvie.com/posts/a-successful-git-branching-model/)
