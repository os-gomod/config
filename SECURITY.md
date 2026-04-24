# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | ✅ |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**DO NOT** open a public issue for security vulnerabilities.

Instead, please report security vulnerabilities by:

1. Emailing: smartcybersecure@gmail.com
2. Include a description of the vulnerability
3. Include steps to reproduce
4. Include potential impact assessment

We will acknowledge receipt within 24 hours and aim to provide a resolution within 72 hours.

## Security Features

### Secret Redaction

All configuration values with keys matching sensitive patterns are automatically redacted:

- `.password`, `.passwd`, `.pwd`
- `.token`, `.access_token`, `.refresh_token`
- `.secret`, `.client_secret`
- `.key`, `.api_key`, `.apikey`
- `.credential`, `.credentials`
- `.auth`, `.authorization`
- `.private`

Redaction applies to:
- Event bus publications
- Audit log entries
- Config snapshots
- Error messages

### Encryption at Rest

The `secure` package provides AES-256-GCM encryption for sensitive values:
- Symmetric key encryption/decryption
- Key rotation without downtime
- Integration with HashiCorp Vault

### Audit Trail

All configuration mutations are logged with:
- Operation type (set, delete, bind, reload)
- Actor identity
- Distributed trace ID
- Timestamp
- Before/after values (redacted for secrets)

### Access Control

Multi-tenant isolation via namespace prefixing ensures tenants can only access their own configuration keys.

## Dependencies

We actively monitor and update dependencies for known vulnerabilities. Security updates are released as patch versions.
