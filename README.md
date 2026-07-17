# personal-site

A Go HTTP server for [oliverlambson.com](https://oliverlambson.com). Templates,
Markdown content, and static assets are embedded in the binary.

## Development

Install the live-reload tool once, then start the site on
`http://127.0.0.1:1960`:

```sh
make dev.deps
make dev
```

To make a development server reachable on another interface, set the explicit
listener override:

```sh
PERSONAL_SITE_ADDR=0.0.0.0:1960 make dev
```

Run the checks and build a statically linked local binary with:

```sh
npm ci
make lint
make test
make build
```

Production leaves `PERSONAL_SITE_ADDR` unset and listens on `127.0.0.1:1960`.

## Deployment

Pushes to `main` build and test natively on GitHub's Ubuntu 24.04 ARM64 runner,
join the personal tailnet using GitHub OIDC, and deploy the static binary over
SSH to `ollie@private.oliverlambson.com`.

The workflow requires these encrypted repository secrets:

- `TS_OAUTH_CLIENT_ID`: Tailscale federated identity client ID;
- `TS_AUDIENCE`: Tailscale federated identity audience; and
- `VPS_SSH_KEY`: the existing VPS SSH private key.

The host must already have `~/bin/deploy-personal-site` installed. The helper
performs the atomic install, health check, and rollback.
