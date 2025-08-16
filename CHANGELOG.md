# Changelog

All notable changes to Tick-Storm will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- TCP connection management with IPv4/IPv6 support
- Binary framing protocol with CRC32C checksums
- Protobuf message serialization/deserialization
- Authentication mechanism with AUTH frame
- Message type definitions and routing
- Connection state management and metrics
- Graceful shutdown support
- TCP optimizations (TCP_NODELAY, KeepAlive)
- Comprehensive error handling
- Structured JSON logging
- Prometheus metrics integration

### Changed
- N/A (Initial development)

### Deprecated
- N/A (Initial development)

### Removed
- N/A (Initial development)

### Fixed
- N/A (Initial development)

### Security
- Mandatory authentication on first frame
- Rate limiting for authentication attempts
- Connection limit enforcement (100k max)
- Input validation for all protocol messages

## [0.1.0] - 2025-08-16 (Pre-release)

### Added
- Initial project structure and documentation
- Core TCP server implementation
- Protocol framing layer
- Authentication module
- Basic connection handling
- Project documentation (PRD, Tech Spec, Architecture)
- Versioning plan and maintenance guidelines
- Security hardening guidelines

### Notes
This is a pre-release version for internal testing and development.

---

## Version History

- **v0.1.0** - Initial pre-release with core TCP functionality
- **v0.0.1** - Project initialization

[Unreleased]: https://github.com/furkansarikaya/tick-storm/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/furkansarikaya/tick-storm/releases/tag/v0.1.0
