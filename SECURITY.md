# Security Policy

## Supported Versions

`aico` is pre-1.0. Security fixes are applied to the latest released version
only.

| Version | Supported |
| ------- | --------- |
| latest  | ✅        |
| older   | ❌        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, use one of the following:

- GitHub's [private vulnerability reporting](https://github.com/yldgio/aico/security/advisories/new)
  (preferred), or
- email **yldgio@gmail.com** with the details.

Please include:

- a description of the issue and its impact,
- steps to reproduce or a proof of concept,
- the version / commit affected.

You can expect an initial acknowledgement within a few days. We will keep you
informed of progress toward a fix and coordinate disclosure timing with you.

## Security model notes

`aico` is a convenience tool for launching agents in containers; container
isolation is a side effect, **not** a hardened security boundary. In particular:

- Host credentials are mounted read-only or forwarded as environment variables
  into the container; treat the container as trusted with those credentials.
- API keys are forwarded **by name** (`-e KEY`) so their values do not appear in
  process arguments.
- aico shells out to your container runtime (`docker`/`podman`); the runtime's
  own security configuration applies.

Do not rely on `aico` to sandbox untrusted code.
