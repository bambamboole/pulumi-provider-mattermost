package provider

import (
	"context"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

type clientKey struct{}

func client(ctx context.Context) *mm.Client {
	if c, ok := ctx.Value(clientKey{}).(*mm.Client); ok {
		return c
	}
	return infer.GetConfig[Config](ctx).client
}

func isNotFound(response *model.Response) bool {
	return response != nil && response.StatusCode == http.StatusNotFound
}
