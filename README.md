# Pulumi Provider Mattermost

A native Pulumi provider for Mattermost, written in Go using `pulumi-go-provider/infer`.

## Configuration

```bash
pulumi config set mattermost:baseUrl https://mattermost.example.com
pulumi config set --secret mattermost:token <personal-access-token>
```

Alternatively, use:

```bash
export MATTERMOST_BASE_URL=https://mattermost.example.com
export MATTERMOST_TOKEN=<personal-access-token>
```

The token may be omitted for a provider instance that only creates a `Bootstrap` resource (see below); every other request then fails with `mattermost: missing token`.

## Bootstrap: obtaining a system-admin token without user interaction

`mattermost:index:Bootstrap` gets a bot token with system-admin rights from a server nobody has logged in to yet, and it also adopts a server that is already running. It authenticates on its own:

1. With `adminToken` set, that token is used. Otherwise it logs in with `adminUsername`/`adminPassword`. If the login fails and the server reports `NoAccounts` (no user exists yet), it signs the admin up through the unauthenticated `POST /api/v4/users`; Mattermost promotes the first account to system admin. The admin account is kept as the human login.
2. The bot named `botUsername` (default `pulumi`) is created, or adopted if it already exists.
3. The `roles` (default `system_user`, `system_admin`, `system_post_all`) are applied to the bot account.
4. An access token described by `tokenDescription` is issued. Bot tokens are exempt from `EnableUserAccessTokens`.

Outputs: `token` (secret), `tokenId`, `botUserId`, `adminUserId`. Changing `tokenDescription` rotates the token; refreshing detects a revoked token or deleted bot and recreates. Deleting the resource revokes the token and keeps the bot and the admin account.

Use two provider instances: one without a token for the bootstrap, one fed by its output for everything else.

```typescript
const bootstrapProvider = new mattermost.Provider("mattermost-bootstrap", { baseUrl });
const bootstrap = new mattermost.Bootstrap("pulumi", {
    adminUsername: "admin",
    adminEmail: "admin@example.com",
    adminPassword: config.requireSecret("mattermostAdminPassword"),
    // adminToken: config.requireSecret("existingAdminToken"), // alternative on a running server
}, { provider: bootstrapProvider, protect: true });

const provider = new mattermost.Provider("mattermost", { baseUrl, token: bootstrap.token });
new mattermost.Team("engineering", { name: "engineering", displayName: "Engineering" }, { provider });
```

## Resources

- `mattermost:index:Team`
- `mattermost:index:Channel`
- `mattermost:index:User` (system roles via `roles`, see the `SystemRole` enum, e.g. `system_admin`)
- `mattermost:index:TeamMember` (`schemeAdmin` grants team admin)
- `mattermost:index:ChannelMember` (`schemeAdmin` grants channel admin)
- `mattermost:index:IncomingWebhook`
- `mattermost:index:OutgoingWebhook`
- `mattermost:index:Bot` (system roles via `roles`; unset leaves the roles of existing bots untouched)
- `mattermost:index:OAuthApp`
- `mattermost:index:SystemConfig`
- `mattermost:index:Bootstrap` (first admin signup or login, bot, roles and token; see above)

## Development

```bash
make test        # unit tests
make lint        # golangci-lint v2 (see .golangci.yml)
make fmt         # gofmt all Go sources
make schema      # build the provider and dump schema.json
make gen-sdk     # regenerate the checked-in TypeScript SDK in sdk/nodejs
make build-sdk   # compile the SDK into sdk/nodejs/bin
make install-local  # install the plugin into the local Pulumi plugin cache
```

`go.sum` and the generated SDK sources under `sdk/nodejs` are checked in. CI fails if `go mod tidy` or `make gen-sdk` would produce a different result, so run them after changing resources or dependencies.

## Releases

Releases follow the same flow as `pulumi-provider-coolify`:

1. `release-please` maintains the release PR, changelog, version and `vX.Y.Z` tag.
2. Pushing the release tag runs GoReleaser for Linux, macOS and Windows on amd64/arm64.
3. The release workflow generates and attaches `schema.json`.
4. The checked-in TypeScript SDK is built and published as `@bambamboole/pulumi-mattermost` when `NPM_TOKEN` is configured.

The repository expects `RELEASE_PLEASE_TOKEN` for release-please. `NPM_TOKEN` is optional; without it the release workflow performs an npm dry-run instead.

## License

Apache License 2.0, see [LICENSE](LICENSE) and [NOTICE](NOTICE).
