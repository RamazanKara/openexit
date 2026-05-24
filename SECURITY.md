# Security Policy

OpenExit is designed for local-first assessment workflows.

## Reporting Vulnerabilities

Please open a private security advisory or contact the maintainers directly. Do not publish exploit details before maintainers have had a reasonable chance to respond.

## Handling Credentials

OpenExit must not store Datadog API keys, app keys, tokens, passwords, or private keys. Collectors read credentials from environment variables and redact raw source evidence before writing files.

## Supported Version

The `main` branch is the supported development target until the first stable release.
