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

## Bootstrap: obtaining an admin token without user interaction

`mattermost:index:Bootstrap` gets a personal access token of a system-admin **user** from a server nobody has logged in to yet, and it also adopts a server that is already running. That user is the account everything else is managed with: unlike a bot it may create bots, and its token is an ordinary personal access token. The resource authenticates on its own:

1. With `adminToken` set (any system admin's personal access or bot token), the user named `username` is created with `email`, or adopted if it already exists; adopting sets its password. Without `adminToken`, the user logs in with `password`. If that fails and the server reports `NoAccounts` (no user exists yet), the user is signed up through the unauthenticated `POST /api/v4/users`; Mattermost promotes the first account to system admin. Otherwise the resource fails with a message asking for the password or `adminToken`.
2. `password` is optional. When unset, a 40-character password is generated and kept in state (`generatedPassword`, secret).
3. The `roles` (default `system_user`, `system_admin`) are applied.
4. Personal access tokens are enabled on the server when `ServiceSettings.EnableUserAccessTokens` is off, and a token described by `tokenDescription` (default `pulumi`) is issued.

Outputs: `token` (secret), `tokenId`, `userId`, `generatedPassword` (secret). Changing `tokenDescription` rotates the token, changing `email`, `roles` or `password` updates the user, changing `username` replaces the resource. Refreshing detects a revoked token or a deleted user and recreates; recovery then needs the configured `password` or `adminToken`, because the generated password is gone with the state entry. Deleting the resource revokes the token and keeps the user.

Use two provider instances: one without a token for the bootstrap, one fed by its output for everything else.

```typescript
const bootstrapProvider = new mattermost.Provider("mattermost-bootstrap", { baseUrl });
const bootstrap = new mattermost.Bootstrap("infrastructure", {
    username: "infrastructure",
    email: "infrastructure@example.com",
    // Only needed while the server already has accounts and the user does not exist yet:
    adminToken: config.requireSecret("existingAdminToken"),
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
- `mattermost:index:Command` (custom slash command of a team; the token Mattermost sends to the URL is kept in state as a secret)
- `mattermost:index:Bot` (system roles via `roles`; unset leaves the roles of existing bots untouched)
- `mattermost:index:AccessToken` (personal access token of a user or bot, e.g. a bot token for an integration; the value is kept in state as a secret, changing `userId` or `description` replaces it)
- `mattermost:index:OAuthApp`
- `mattermost:index:SystemConfig`
- `mattermost:index:Bootstrap` (admin user with a personal access token; see above)

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
