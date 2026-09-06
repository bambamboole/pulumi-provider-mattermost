package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SystemConfig manages a focused set of Mattermost server configuration
// settings. Fields left unset are not managed and are preserved on update.
type SystemConfig struct{}

type SystemConfigArgs struct {
	SiteURL                                *string `pulumi:"siteUrl,optional"`
	ListenAddress                          *string `pulumi:"listenAddress,optional"`
	MaximumLoginAttempts                   *int    `pulumi:"maximumLoginAttempts,optional"`
	EnableOAuthServiceProvider             *bool   `pulumi:"enableOAuthServiceProvider,optional"`
	EnableDynamicClientRegistration        *bool   `pulumi:"enableDynamicClientRegistration,optional"`
	EnableIncomingWebhooks                 *bool   `pulumi:"enableIncomingWebhooks,optional"`
	EnableOutgoingWebhooks                 *bool   `pulumi:"enableOutgoingWebhooks,optional"`
	EnableCommands                         *bool   `pulumi:"enableCommands,optional"`
	OutgoingIntegrationRequestsTimeout     *int64  `pulumi:"outgoingIntegrationRequestsTimeout,optional"`
	EnablePostUsernameOverride             *bool   `pulumi:"enablePostUsernameOverride,optional"`
	EnablePostIconOverride                 *bool   `pulumi:"enablePostIconOverride,optional"`
	EnableMultifactorAuthentication        *bool   `pulumi:"enableMultifactorAuthentication,optional"`
	EnforceMultifactorAuthentication       *bool   `pulumi:"enforceMultifactorAuthentication,optional"`
	EnableUserAccessTokens                 *bool   `pulumi:"enableUserAccessTokens,optional"`
	MaximumPersonalAccessTokenLifetimeDays *int    `pulumi:"maximumPersonalAccessTokenLifetimeDays,optional"`
	AllowCorsFrom                          *string `pulumi:"allowCorsFrom,optional"`
	CorsAllowCredentials                   *bool   `pulumi:"corsAllowCredentials,optional"`
	SessionIdleTimeoutInMinutes            *int    `pulumi:"sessionIdleTimeoutInMinutes,optional"`
	EnableCustomEmoji                      *bool   `pulumi:"enableCustomEmoji,optional"`
	EnableEmojiPicker                      *bool   `pulumi:"enableEmojiPicker,optional"`
	EnableEmailInvitations                 *bool   `pulumi:"enableEmailInvitations,optional"`
	DisableBotsWhenOwnerIsDeactivated      *bool   `pulumi:"disableBotsWhenOwnerIsDeactivated,optional"`
	EnableBotAccountCreation               *bool   `pulumi:"enableBotAccountCreation,optional"`
	EnableAPITeamDeletion                  *bool   `pulumi:"enableApiTeamDeletion,optional"`
	EnableAPIUserDeletion                  *bool   `pulumi:"enableApiUserDeletion,optional"`
	EnableAPIPostDeletion                  *bool   `pulumi:"enableApiPostDeletion,optional"`
	EnableAPIChannelDeletion               *bool   `pulumi:"enableApiChannelDeletion,optional"`
}

type SystemConfigState struct {
	SystemConfigArgs
}

func (r *SystemConfig) Annotate(a infer.Annotator) {
	a.SetToken("index", "SystemConfig")
	a.Describe(&r, "Selected Mattermost system configuration settings. This is a singleton resource with ID 'system'. Unset fields are left unmanaged.")
}

func (SystemConfig) Create(ctx context.Context, req infer.CreateRequest[SystemConfigArgs]) (infer.CreateResponse[SystemConfigState], error) {
	state := SystemConfigState{SystemConfigArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[SystemConfigState]{ID: "system", Output: state}, nil
	}
	if err := applySystemConfig(ctx, req.Inputs); err != nil {
		return infer.CreateResponse[SystemConfigState]{}, err
	}
	return infer.CreateResponse[SystemConfigState]{ID: "system", Output: state}, nil
}

func (SystemConfig) Update(ctx context.Context, req infer.UpdateRequest[SystemConfigArgs, SystemConfigState]) (infer.UpdateResponse[SystemConfigState], error) {
	state := SystemConfigState{SystemConfigArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[SystemConfigState]{Output: state}, nil
	}
	if err := applySystemConfig(ctx, req.Inputs); err != nil {
		return infer.UpdateResponse[SystemConfigState]{}, err
	}
	return infer.UpdateResponse[SystemConfigState]{Output: state}, nil
}

func (SystemConfig) Read(ctx context.Context, req infer.ReadRequest[SystemConfigArgs, SystemConfigState]) (infer.ReadResponse[SystemConfigArgs, SystemConfigState], error) {
	cfg, response, err := client(ctx).API.GetConfig(ctx)
	if isNotFound(response) {
		return infer.ReadResponse[SystemConfigArgs, SystemConfigState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[SystemConfigArgs, SystemConfigState]{}, err
	}

	importAll := systemConfigEmpty(req.Inputs)
	inputs := readSystemConfig(cfg.ServiceSettings, req.Inputs, importAll)
	return infer.ReadResponse[SystemConfigArgs, SystemConfigState]{
		ID:     "system",
		Inputs: inputs,
		State:  SystemConfigState{SystemConfigArgs: inputs},
	}, nil
}

// Delete intentionally does not mutate the Mattermost server. Removing a
// singleton configuration resource from Pulumi means "stop managing it", not
// "reset the server configuration to defaults".
func (SystemConfig) Delete(context.Context, infer.DeleteRequest[SystemConfigState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func applySystemConfig(ctx context.Context, args SystemConfigArgs) error {
	cfg, _, err := client(ctx).API.GetConfig(ctx)
	if err != nil {
		return err
	}
	applyServiceSettings(&cfg.ServiceSettings, args)
	_, _, err = client(ctx).API.UpdateConfig(ctx, cfg)
	return err
}

func applyServiceSettings(s *model.ServiceSettings, a SystemConfigArgs) {
	if a.SiteURL != nil { s.SiteURL = a.SiteURL }
	if a.ListenAddress != nil { s.ListenAddress = a.ListenAddress }
	if a.MaximumLoginAttempts != nil { s.MaximumLoginAttempts = a.MaximumLoginAttempts }
	if a.EnableOAuthServiceProvider != nil { s.EnableOAuthServiceProvider = a.EnableOAuthServiceProvider }
	if a.EnableDynamicClientRegistration != nil { s.EnableDynamicClientRegistration = a.EnableDynamicClientRegistration }
	if a.EnableIncomingWebhooks != nil { s.EnableIncomingWebhooks = a.EnableIncomingWebhooks }
	if a.EnableOutgoingWebhooks != nil { s.EnableOutgoingWebhooks = a.EnableOutgoingWebhooks }
	if a.EnableCommands != nil { s.EnableCommands = a.EnableCommands }
	if a.OutgoingIntegrationRequestsTimeout != nil { s.OutgoingIntegrationRequestsTimeout = a.OutgoingIntegrationRequestsTimeout }
	if a.EnablePostUsernameOverride != nil { s.EnablePostUsernameOverride = a.EnablePostUsernameOverride }
	if a.EnablePostIconOverride != nil { s.EnablePostIconOverride = a.EnablePostIconOverride }
	if a.EnableMultifactorAuthentication != nil { s.EnableMultifactorAuthentication = a.EnableMultifactorAuthentication }
	if a.EnforceMultifactorAuthentication != nil { s.EnforceMultifactorAuthentication = a.EnforceMultifactorAuthentication }
	if a.EnableUserAccessTokens != nil { s.EnableUserAccessTokens = a.EnableUserAccessTokens }
	if a.MaximumPersonalAccessTokenLifetimeDays != nil { s.MaximumPersonalAccessTokenLifetimeDays = a.MaximumPersonalAccessTokenLifetimeDays }
	if a.AllowCorsFrom != nil { s.AllowCorsFrom = a.AllowCorsFrom }
	if a.CorsAllowCredentials != nil { s.CorsAllowCredentials = a.CorsAllowCredentials }
	if a.SessionIdleTimeoutInMinutes != nil { s.SessionIdleTimeoutInMinutes = a.SessionIdleTimeoutInMinutes }
	if a.EnableCustomEmoji != nil { s.EnableCustomEmoji = a.EnableCustomEmoji }
	if a.EnableEmojiPicker != nil { s.EnableEmojiPicker = a.EnableEmojiPicker }
	if a.EnableEmailInvitations != nil { s.EnableEmailInvitations = a.EnableEmailInvitations }
	if a.DisableBotsWhenOwnerIsDeactivated != nil { s.DisableBotsWhenOwnerIsDeactivated = a.DisableBotsWhenOwnerIsDeactivated }
	if a.EnableBotAccountCreation != nil { s.EnableBotAccountCreation = a.EnableBotAccountCreation }
	if a.EnableAPITeamDeletion != nil { s.EnableAPITeamDeletion = a.EnableAPITeamDeletion }
	if a.EnableAPIUserDeletion != nil { s.EnableAPIUserDeletion = a.EnableAPIUserDeletion }
	if a.EnableAPIPostDeletion != nil { s.EnableAPIPostDeletion = a.EnableAPIPostDeletion }
	if a.EnableAPIChannelDeletion != nil { s.EnableAPIChannelDeletion = a.EnableAPIChannelDeletion }
}

func readSystemConfig(s model.ServiceSettings, declared SystemConfigArgs, all bool) SystemConfigArgs {
	return SystemConfigArgs{
		SiteURL: managed(declared.SiteURL, s.SiteURL, all),
		ListenAddress: managed(declared.ListenAddress, s.ListenAddress, all),
		MaximumLoginAttempts: managed(declared.MaximumLoginAttempts, s.MaximumLoginAttempts, all),
		EnableOAuthServiceProvider: managed(declared.EnableOAuthServiceProvider, s.EnableOAuthServiceProvider, all),
		EnableDynamicClientRegistration: managed(declared.EnableDynamicClientRegistration, s.EnableDynamicClientRegistration, all),
		EnableIncomingWebhooks: managed(declared.EnableIncomingWebhooks, s.EnableIncomingWebhooks, all),
		EnableOutgoingWebhooks: managed(declared.EnableOutgoingWebhooks, s.EnableOutgoingWebhooks, all),
		EnableCommands: managed(declared.EnableCommands, s.EnableCommands, all),
		OutgoingIntegrationRequestsTimeout: managed(declared.OutgoingIntegrationRequestsTimeout, s.OutgoingIntegrationRequestsTimeout, all),
		EnablePostUsernameOverride: managed(declared.EnablePostUsernameOverride, s.EnablePostUsernameOverride, all),
		EnablePostIconOverride: managed(declared.EnablePostIconOverride, s.EnablePostIconOverride, all),
		EnableMultifactorAuthentication: managed(declared.EnableMultifactorAuthentication, s.EnableMultifactorAuthentication, all),
		EnforceMultifactorAuthentication: managed(declared.EnforceMultifactorAuthentication, s.EnforceMultifactorAuthentication, all),
		EnableUserAccessTokens: managed(declared.EnableUserAccessTokens, s.EnableUserAccessTokens, all),
		MaximumPersonalAccessTokenLifetimeDays: managed(declared.MaximumPersonalAccessTokenLifetimeDays, s.MaximumPersonalAccessTokenLifetimeDays, all),
		AllowCorsFrom: managed(declared.AllowCorsFrom, s.AllowCorsFrom, all),
		CorsAllowCredentials: managed(declared.CorsAllowCredentials, s.CorsAllowCredentials, all),
		SessionIdleTimeoutInMinutes: managed(declared.SessionIdleTimeoutInMinutes, s.SessionIdleTimeoutInMinutes, all),
		EnableCustomEmoji: managed(declared.EnableCustomEmoji, s.EnableCustomEmoji, all),
		EnableEmojiPicker: managed(declared.EnableEmojiPicker, s.EnableEmojiPicker, all),
		EnableEmailInvitations: managed(declared.EnableEmailInvitations, s.EnableEmailInvitations, all),
		DisableBotsWhenOwnerIsDeactivated: managed(declared.DisableBotsWhenOwnerIsDeactivated, s.DisableBotsWhenOwnerIsDeactivated, all),
		EnableBotAccountCreation: managed(declared.EnableBotAccountCreation, s.EnableBotAccountCreation, all),
		EnableAPITeamDeletion: managed(declared.EnableAPITeamDeletion, s.EnableAPITeamDeletion, all),
		EnableAPIUserDeletion: managed(declared.EnableAPIUserDeletion, s.EnableAPIUserDeletion, all),
		EnableAPIPostDeletion: managed(declared.EnableAPIPostDeletion, s.EnableAPIPostDeletion, all),
		EnableAPIChannelDeletion: managed(declared.EnableAPIChannelDeletion, s.EnableAPIChannelDeletion, all),
	}
}

func managed[T any](declared, current *T, all bool) *T {
	if all || declared != nil {
		return current
	}
	return nil
}

func systemConfigEmpty(a SystemConfigArgs) bool {
	return a.SiteURL == nil && a.ListenAddress == nil && a.MaximumLoginAttempts == nil &&
		a.EnableOAuthServiceProvider == nil && a.EnableDynamicClientRegistration == nil &&
		a.EnableIncomingWebhooks == nil && a.EnableOutgoingWebhooks == nil && a.EnableCommands == nil &&
		a.OutgoingIntegrationRequestsTimeout == nil && a.EnablePostUsernameOverride == nil && a.EnablePostIconOverride == nil &&
		a.EnableMultifactorAuthentication == nil && a.EnforceMultifactorAuthentication == nil && a.EnableUserAccessTokens == nil &&
		a.MaximumPersonalAccessTokenLifetimeDays == nil && a.AllowCorsFrom == nil && a.CorsAllowCredentials == nil &&
		a.SessionIdleTimeoutInMinutes == nil && a.EnableCustomEmoji == nil && a.EnableEmojiPicker == nil &&
		a.EnableEmailInvitations == nil && a.DisableBotsWhenOwnerIsDeactivated == nil && a.EnableBotAccountCreation == nil &&
		a.EnableAPITeamDeletion == nil && a.EnableAPIUserDeletion == nil && a.EnableAPIPostDeletion == nil &&
		a.EnableAPIChannelDeletion == nil
}
