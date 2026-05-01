# Security Policy

## Reporting Security Issues

Report vulnerabilities through the fork's GitHub security advisory flow or by contacting the project maintainers privately.

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix, if known

## Security Considerations

task-ledger stores issue data as plain JSON files under `.task-ledger/issues/**/issue.json`. Those files are intended to be committed with the repository.

Do not store passwords, API keys, credentials, customer secrets, or sensitive incident details in issue titles, descriptions, notes, comments, labels, or dependency metadata. This project does not encrypt issue data at rest and relies on normal filesystem and git repository access controls.

The retained CLI does not require SQLite, a daemon, a network service, or a generated JSONL index. Normal commands read and write local project files only.

## Git And Hooks

Git hooks are optional and run with your local user permissions. Review any hook scripts before enabling them. The default JSON-file backend does not install a background sync process.

## Dependency Integrity

Use:

```bash
go mod verify
go test -buildvcs=false ./...
```

## Supported Versions

Security fixes target the current `main` branch.
