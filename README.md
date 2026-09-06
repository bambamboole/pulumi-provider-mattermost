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

## Resources

- `mattermost:index:Team`
- `mattermost:index:Channel`
- `mattermost:index:User`
- `mattermost:index:TeamMember`
- `mattermost:index:ChannelMember`
- `mattermost:index:IncomingWebhook`
- `mattermost:index:OutgoingWebhook`
- `mattermost:index:Bot`
- `mattermost:index:OAuthApp`
- `mattermost:index:SystemConfig`

## Development

```bash
make test
make build
```

## Releases

Releases follow the same flow as `pulumi-provider-coolify`:

1. `release-please` maintains the release PR, changelog, version and `vX.Y.Z` tag.
2. Pushing the release tag runs GoReleaser for Linux, macOS and Windows on amd64/arm64.
3. The release workflow generates and attaches `schema.json`.
4. A TypeScript SDK is generated from the provider schema and published as `@bambamboole/mattermost` when `NPM_TOKEN` is configured.

The repository expects `RELEASE_PLEASE_TOKEN` for release-please. `NPM_TOKEN` is optional; without it the release workflow performs an npm dry-run instead.
