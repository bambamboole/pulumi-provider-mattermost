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
- `mattermost:index:User` (system roles via `roles`, see the `SystemRole` enum, e.g. `system_admin`)
- `mattermost:index:TeamMember` (`schemeAdmin` grants team admin)
- `mattermost:index:ChannelMember` (`schemeAdmin` grants channel admin)
- `mattermost:index:IncomingWebhook`
- `mattermost:index:OutgoingWebhook`
- `mattermost:index:Bot`
- `mattermost:index:OAuthApp`
- `mattermost:index:SystemConfig`

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
