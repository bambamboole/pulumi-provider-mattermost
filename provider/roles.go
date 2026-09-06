package provider

import (
	"sort"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SystemRole is a Mattermost system-level role that can be assigned to a user.
type SystemRole string

const (
	SystemRoleUser             SystemRole = model.SystemUserRoleId
	SystemRoleAdmin            SystemRole = model.SystemAdminRoleId
	SystemRoleGuest            SystemRole = model.SystemGuestRoleId
	SystemRoleManager          SystemRole = model.SystemManagerRoleId
	SystemRoleUserManager      SystemRole = model.SystemUserManagerRoleId
	SystemRoleReadOnlyAdmin    SystemRole = model.SystemReadOnlyAdminRoleId
	SystemRoleCustomGroupAdmin SystemRole = model.SystemCustomGroupAdminRoleId
	SystemRolePostAll          SystemRole = model.SystemPostAllRoleId
	SystemRolePostAllPublic    SystemRole = model.SystemPostAllPublicRoleId
	SystemRoleUserAccessToken  SystemRole = model.SystemUserAccessTokenRoleId
)

func (SystemRole) Values() []infer.EnumValue[SystemRole] {
	return []infer.EnumValue[SystemRole]{
		{Name: "User", Value: SystemRoleUser, Description: "Regular member of the system."},
		{Name: "Admin", Value: SystemRoleAdmin, Description: "System administrator with full access to the System Console."},
		{Name: "Guest", Value: SystemRoleGuest, Description: "Guest account restricted to the channels it is added to."},
		{Name: "Manager", Value: SystemRoleManager, Description: "System manager with access to most System Console sections."},
		{Name: "UserManager", Value: SystemRoleUserManager, Description: "User manager who can manage users, teams and channels."},
		{Name: "ReadOnlyAdmin", Value: SystemRoleReadOnlyAdmin, Description: "Read-only access to the System Console."},
		{Name: "CustomGroupAdmin", Value: SystemRoleCustomGroupAdmin, Description: "Can manage custom user groups."},
		{Name: "PostAll", Value: SystemRolePostAll, Description: "Can post to any channel, including private channels and direct messages."},
		{Name: "PostAllPublic", Value: SystemRolePostAllPublic, Description: "Can post to any public channel."},
		{Name: "UserAccessToken", Value: SystemRoleUserAccessToken, Description: "Can create personal access tokens."},
	}
}

// normalizeRoles returns a sorted, de-duplicated copy of roles, defaulting to
// system_user when empty so an unset input matches what Mattermost assigns.
func normalizeRoles(roles []SystemRole) []SystemRole {
	seen := map[SystemRole]bool{}
	out := make([]SystemRole, 0, len(roles))
	for _, role := range roles {
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		out = append(out, role)
	}
	if len(out) == 0 {
		out = append(out, SystemRoleUser)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// parseRoles converts the space-separated roles string of the API into a normalized list.
func parseRoles(roles string) []SystemRole {
	out := []SystemRole{}
	for _, role := range strings.Fields(roles) {
		out = append(out, SystemRole(role))
	}
	return normalizeRoles(out)
}

// joinRoles converts a role list into the space-separated form the API expects.
func joinRoles(roles []SystemRole) string {
	parts := make([]string, len(roles))
	for i, role := range roles {
		parts[i] = string(role)
	}
	return strings.Join(parts, " ")
}

func rolesEqual(a, b []SystemRole) bool {
	return joinRoles(normalizeRoles(a)) == joinRoles(normalizeRoles(b))
}
