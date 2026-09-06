# pulumi-provider-mattermost

A native [Pulumi](https://www.pulumi.com) provider for [Mattermost](https://mattermost.com), written in Go using [`pulumi-go-provider/infer`](https://github.com/pulumi/pulumi-go-provider).

## Status

Early development. The initial provider exposes `Team` and `Channel` resources.

## Configuration

| Setting | Environment variable | Description |
| --- | --- | --- |
| `baseUrl` | `MATTERMOST_BASE_URL` | Base URL of the Mattermost instance |
| `token` | `MATTERMOST_TOKEN` | Personal access token or bot token |

```bash
export MATTERMOST_BASE_URL=https://mattermost.example.com
export MATTERMOST_TOKEN=...
```

## Resources

- `mattermost:index:Team`
- `mattermost:index:Channel`

## Development

```bash
make tidy
make test
make build
```

## License

Apache-2.0
