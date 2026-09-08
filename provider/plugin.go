package provider

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Plugin installs a Mattermost plugin from the marketplace (which includes
// the plugins prepackaged with the server) or from a download URL, enables
// it and manages its settings in the server configuration.
type Plugin struct{}

type PluginArgs struct {
	PluginID    string         `pulumi:"pluginId" provider:"replaceOnChanges"`
	Version     *string        `pulumi:"version,optional"`
	DownloadURL *string        `pulumi:"downloadUrl,optional"`
	Enabled     *bool          `pulumi:"enabled,optional"`
	Settings    map[string]any `pulumi:"settings,optional"`
}

type PluginState struct {
	PluginArgs
	Name             string `pulumi:"name"`
	InstalledVersion string `pulumi:"installedVersion"`
}

func (r *Plugin) Annotate(a infer.Annotator) {
	a.SetToken("index", "Plugin")
	a.Describe(&r, "A Mattermost plugin. Without `downloadUrl` the plugin is installed from the marketplace, which also lists the plugins prepackaged with the server; a remote marketplace install needs PluginSettings.EnableMarketplace and EnableRemoteMarketplace. With `downloadUrl` the plugin bundle is downloaded by the server, which needs PluginSettings.EnableUploads (and AllowInsecureDownloadURL for plain HTTP). The provider's account needs manage_system. Deleting the resource removes the plugin; its settings stay in the server configuration, as they do when a plugin is removed in the System Console. The resource ID is the plugin ID.")
}

func (args *PluginArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PluginID, "ID of the plugin as declared in its manifest, e.g. `mattermost-ai`.")
	a.Describe(&args.Version, "Version to install from the marketplace. The latest version offered for the server is installed when unset. Changing it reinstalls the plugin. Ignored when `downloadUrl` is set.")
	a.Describe(&args.DownloadURL, "URL of the plugin bundle (a `.tar.gz`) the server downloads instead of using the marketplace. Changing it reinstalls the plugin.")
	a.Describe(&args.Enabled, "Whether the plugin is enabled. Defaults to true.")
	a.SetDefault(&args.Enabled, true)
	a.Describe(&args.Settings, "The plugin's settings, stored under PluginSettings.Plugins.<pluginId> in the server configuration and replacing what is there. Keys are the setting keys of the plugin's manifest. Left unmanaged when unset. Secret settings are kept from the inputs on refresh because the API masks them.")
}

func (state *PluginState) Annotate(a infer.Annotator) {
	a.Describe(&state.Name, "Name of the plugin from its manifest.")
	a.Describe(&state.InstalledVersion, "Version of the plugin installed on the server.")
}

func (Plugin) Create(ctx context.Context, req infer.CreateRequest[PluginArgs]) (infer.CreateResponse[PluginState], error) {
	state := PluginState{PluginArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[PluginState]{ID: req.Inputs.PluginID, Output: state}, nil
	}
	manifest, err := installPlugin(ctx, req.Inputs)
	if err != nil {
		return infer.CreateResponse[PluginState]{}, err
	}
	if err := applyPlugin(ctx, req.Inputs); err != nil {
		return infer.CreateResponse[PluginState]{}, err
	}
	state.Name = manifest.Name
	state.InstalledVersion = manifest.Version
	return infer.CreateResponse[PluginState]{ID: req.Inputs.PluginID, Output: state}, nil
}

// Update reinstalls the plugin when its version or download URL changed and
// otherwise only applies the settings and the enabled flag.
func (Plugin) Update(ctx context.Context, req infer.UpdateRequest[PluginArgs, PluginState]) (infer.UpdateResponse[PluginState], error) {
	state := PluginState{PluginArgs: req.Inputs, Name: req.State.Name, InstalledVersion: req.State.InstalledVersion}
	if req.DryRun {
		return infer.UpdateResponse[PluginState]{Output: state}, nil
	}
	if pluginSourceChanged(req.State.PluginArgs, req.Inputs) {
		manifest, err := installPlugin(ctx, req.Inputs)
		if err != nil {
			return infer.UpdateResponse[PluginState]{}, err
		}
		state.Name = manifest.Name
		state.InstalledVersion = manifest.Version
	}
	if err := applyPlugin(ctx, req.Inputs); err != nil {
		return infer.UpdateResponse[PluginState]{}, err
	}
	return infer.UpdateResponse[PluginState]{Output: state}, nil
}

// Read supports import: with empty inputs every setting of the plugin is
// imported and the installed version becomes the declared one. Otherwise
// settings are only refreshed while they are managed, and a declared version
// that differs from the installed one is reported as drift.
func (Plugin) Read(ctx context.Context, req infer.ReadRequest[PluginArgs, PluginState]) (infer.ReadResponse[PluginArgs, PluginState], error) {
	plugins, _, err := client(ctx).API.GetPlugins(ctx)
	if err != nil {
		return infer.ReadResponse[PluginArgs, PluginState]{}, err
	}
	manifest, active := findPlugin(plugins, req.ID)
	if manifest == nil {
		return infer.ReadResponse[PluginArgs, PluginState]{}, nil
	}

	importAll := req.Inputs.PluginID == ""
	inputs := req.Inputs
	inputs.PluginID = req.ID
	inputs.Enabled = &active
	if importAll || (inputs.Version != nil && inputs.DownloadURL == nil) {
		version := manifest.Version
		inputs.Version = &version
	}
	if importAll || inputs.Settings != nil {
		cfg, _, err := client(ctx).API.GetConfig(ctx)
		if err != nil {
			return infer.ReadResponse[PluginArgs, PluginState]{}, err
		}
		inputs.Settings = readPluginSettings(cfg.PluginSettings.Plugins[req.ID], req.Inputs.Settings)
	}

	state := PluginState{PluginArgs: inputs, Name: manifest.Name, InstalledVersion: manifest.Version}
	return infer.ReadResponse[PluginArgs, PluginState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

func (Plugin) Delete(ctx context.Context, req infer.DeleteRequest[PluginState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.RemovePlugin(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func pluginSourceChanged(old, next PluginArgs) bool {
	return stringValue(old.Version) != stringValue(next.Version) || stringValue(old.DownloadURL) != stringValue(next.DownloadURL)
}

func installPlugin(ctx context.Context, args PluginArgs) (*model.Manifest, error) {
	api := client(ctx).API
	if url := stringValue(args.DownloadURL); url != "" {
		manifest, _, err := api.InstallPluginFromURL(ctx, url, true)
		if err != nil {
			return nil, fmt.Errorf("mattermost: installing plugin %s from %s: %w", args.PluginID, url, err)
		}
		return checkInstalledPlugin(args.PluginID, manifest)
	}

	version := stringValue(args.Version)
	if version == "" {
		latest, err := latestMarketplaceVersion(ctx, args.PluginID)
		if err != nil {
			return nil, err
		}
		version = latest
	}
	manifest, _, err := api.InstallMarketplacePlugin(ctx, &model.InstallMarketplacePluginRequest{Id: args.PluginID, Version: version})
	if err != nil {
		return nil, fmt.Errorf("mattermost: installing plugin %s %s from the marketplace: %w", args.PluginID, version, err)
	}
	return checkInstalledPlugin(args.PluginID, manifest)
}

// checkInstalledPlugin guards against a download URL that serves a different
// plugin than the one declared: the resource would otherwise manage settings
// and the enabled flag of a plugin that was never installed.
func checkInstalledPlugin(pluginID string, manifest *model.Manifest) (*model.Manifest, error) {
	if manifest == nil {
		return nil, fmt.Errorf("mattermost: installing plugin %s: the server returned no manifest", pluginID)
	}
	if manifest.Id != pluginID {
		return nil, fmt.Errorf("mattermost: installing plugin %s: the bundle contains plugin %s", pluginID, manifest.Id)
	}
	return manifest, nil
}

func latestMarketplaceVersion(ctx context.Context, pluginID string) (string, error) {
	plugins, _, err := client(ctx).API.GetMarketplacePlugins(ctx, &model.MarketplacePluginFilter{Filter: pluginID, PluginId: pluginID, PerPage: 200})
	if err != nil {
		return "", fmt.Errorf("mattermost: looking up plugin %s in the marketplace: %w", pluginID, err)
	}
	for _, plugin := range plugins {
		if plugin != nil && plugin.BaseMarketplacePlugin != nil && plugin.Manifest != nil && plugin.Manifest.Id == pluginID {
			return plugin.Manifest.Version, nil
		}
	}
	return "", fmt.Errorf("mattermost: plugin %s is not offered by the marketplace; set version or downloadUrl", pluginID)
}

// applyPlugin writes the settings before switching the plugin on, so it
// starts with the declared configuration.
func applyPlugin(ctx context.Context, args PluginArgs) error {
	api := client(ctx).API
	if args.Settings != nil {
		cfg, _, err := api.GetConfig(ctx)
		if err != nil {
			return fmt.Errorf("mattermost: reading the server configuration: %w", err)
		}
		if cfg.PluginSettings.Plugins == nil {
			cfg.PluginSettings.Plugins = map[string]map[string]any{}
		}
		cfg.PluginSettings.Plugins[args.PluginID] = maps.Clone(args.Settings)
		if _, _, err := api.UpdateConfig(ctx, cfg); err != nil {
			return fmt.Errorf("mattermost: writing the settings of plugin %s: %w", args.PluginID, err)
		}
	}
	if args.Enabled == nil || *args.Enabled {
		if _, err := api.EnablePlugin(ctx, args.PluginID); err != nil {
			return fmt.Errorf("mattermost: enabling plugin %s: %w", args.PluginID, err)
		}
		return nil
	}
	if _, err := api.DisablePlugin(ctx, args.PluginID); err != nil {
		return fmt.Errorf("mattermost: disabling plugin %s: %w", args.PluginID, err)
	}
	return nil
}

func findPlugin(plugins *model.PluginsResponse, pluginID string) (*model.Manifest, bool) {
	if plugins == nil {
		return nil, false
	}
	for _, plugin := range plugins.Active {
		if plugin != nil && plugin.Id == pluginID {
			return &plugin.Manifest, true
		}
	}
	for _, plugin := range plugins.Inactive {
		if plugin != nil && plugin.Id == pluginID {
			return &plugin.Manifest, false
		}
	}
	return nil, false
}

// readPluginSettings returns the settings stored on the server. Values the
// API masked because the plugin marks them secret are taken from the
// declared settings, so a refresh does not report them as drift.
func readPluginSettings(server, declared map[string]any) map[string]any {
	settings := map[string]any{}
	for key, value := range server {
		if text, ok := value.(string); ok && text == model.FakeSetting {
			if declaredValue, ok := lookupFold(declared, key); ok {
				value = declaredValue
			}
		}
		settings[key] = value
	}
	return settings
}

func lookupFold(settings map[string]any, key string) (any, bool) {
	if value, ok := settings[key]; ok {
		return value, true
	}
	for candidate, value := range settings {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return nil, false
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
