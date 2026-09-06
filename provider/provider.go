package provider

import (
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// New builds the Mattermost provider.
func New() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithConfig(infer.Config(&Config{})).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{"provider": "index"}).
		WithResources(
			infer.Resource(Team{}),
			infer.Resource(Channel{}),
			infer.Resource(User{}),
			infer.Resource(TeamMember{}),
			infer.Resource(ChannelMember{}),
		).
		WithDisplayName("Mattermost").
		WithDescription("Manage Mattermost teams, channels, users, memberships and integrations.").
		WithPublisher("bambamboole").
		WithRepository("https://github.com/bambamboole/pulumi-provider-mattermost").
		WithHomepage("https://mattermost.com").
		WithLicense("Apache-2.0").
		WithPluginDownloadURL("https://github.com/bambamboole/pulumi-provider-mattermost/releases/download/v$%7BVERSION%7D").
		Build()
}
