package provider

import (
	"context"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

func client(ctx context.Context) *mm.Client {
	return infer.GetConfig[Config](ctx).client
}

func isNotFound(response *model.Response) bool {
	return response != nil && response.StatusCode == http.StatusNotFound
}
