# Changelog

## [0.11.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.10.0...v0.11.0) (2026-09-08)


### Features

* manage mcpDynamicToolLoading on Agent ([d79d03c](https://github.com/bambamboole/pulumi-provider-mattermost/commit/d79d03cbfd0a7d86d270848c965e2713d59987e8))


### Bug Fixes

* keep an agent's botUserId known while other inputs change ([18268f6](https://github.com/bambamboole/pulumi-provider-mattermost/commit/18268f6be86a1e6298f8d9c49b1f0366c220af4a))

## [0.10.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.9.0...v0.10.0) (2026-09-08)


### Features

* add AgentsConfig and Agent resources for the Agents plugin ([2306292](https://github.com/bambamboole/pulumi-provider-mattermost/commit/23062928859e31f81ad62d0452ccaedb475b18c5))
* repair the Bootstrap token instead of recreating the user ([50378fd](https://github.com/bambamboole/pulumi-provider-mattermost/commit/50378fd94ec9c1fc6b7d9ffaaf5959c4a0c27b48))

## [0.9.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.8.0...v0.9.0) (2026-09-08)


### Features

* add Plugin resource for marketplace and URL installs ([8d524f3](https://github.com/bambamboole/pulumi-provider-mattermost/commit/8d524f392865627d1265e3ea0c36a74d716bb17f))

## [0.8.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.7.0...v0.8.0) (2026-09-07)


### Features

* add Command resource for custom slash commands ([df3fe48](https://github.com/bambamboole/pulumi-provider-mattermost/commit/df3fe483dc1cb6af032ad77f83f7a2b6a51c5cd5))

## [0.7.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.6.0...v0.7.0) (2026-09-07)


### Features

* add AccessToken resource for user and bot tokens ([013aab5](https://github.com/bambamboole/pulumi-provider-mattermost/commit/013aab568a0ff6bcb7603ced25f6faa5bee9d4bc))

## [0.6.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.5.0...v0.6.0) (2026-09-07)


### ⚠ BREAKING CHANGES

* the bot inputs (adminUsername, adminEmail, adminPassword, botUsername, botDisplayName, botDescription) and outputs (botUserId, adminUserId) are replaced by username, email, password, userId and generatedPassword.

### Features

* bootstrap an admin user with a personal access token ([1b1e666](https://github.com/bambamboole/pulumi-provider-mattermost/commit/1b1e66624eb46ff38ba9dfefa28ef27bd767998a))

## [0.5.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.4.0...v0.5.0) (2026-09-07)


### Features

* manage bot system roles ([40a8686](https://github.com/bambamboole/pulumi-provider-mattermost/commit/40a8686940d2fd1b1c94d99e90254a2cf3ed40d6))

## [0.4.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.3.0...v0.4.0) (2026-09-06)


### Features

* add Bootstrap resource for a system-admin bot token ([d515c36](https://github.com/bambamboole/pulumi-provider-mattermost/commit/d515c36e65693d3a914ed16e70648ce94f5e5db6)), closes [#10](https://github.com/bambamboole/pulumi-provider-mattermost/issues/10)

## [0.3.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.2.0...v0.3.0) (2026-09-06)


### Features

* manage user system roles and team/channel admins ([70fff6d](https://github.com/bambamboole/pulumi-provider-mattermost/commit/70fff6dc1b454ef0e6be80ecd02d25944a016341))

## [0.2.0](https://github.com/bambamboole/pulumi-provider-mattermost/compare/v0.1.0...v0.2.0) (2026-09-06)


### Features

* add bot resource ([6d10e8c](https://github.com/bambamboole/pulumi-provider-mattermost/commit/6d10e8cd5231c762126155fbc110e6b5c226bd27))
* add channel member resource ([40902da](https://github.com/bambamboole/pulumi-provider-mattermost/commit/40902daf4eb1a3f84efa3fb92bae0cebc3983520))
* add incoming webhook resource ([9bee06d](https://github.com/bambamboole/pulumi-provider-mattermost/commit/9bee06dd42b0980249bd44bd7dff088e6397a35e))
* add OAuth app resource ([09f4a4f](https://github.com/bambamboole/pulumi-provider-mattermost/commit/09f4a4fe4abce12c70da8887b1bf5075263eedfa))
* add outgoing webhook resource ([2155fff](https://github.com/bambamboole/pulumi-provider-mattermost/commit/2155fff8022b0d392b41dc771ce976ac22d1a46b))
* add system config resource ([fc73998](https://github.com/bambamboole/pulumi-provider-mattermost/commit/fc739984bd18f5b64678c62b11b471c8960cbd0c))
* add team member resource ([feb720c](https://github.com/bambamboole/pulumi-provider-mattermost/commit/feb720c3b2a27365eb4230c6318fc02a157acfcc))
* add user resource ([7c18ee6](https://github.com/bambamboole/pulumi-provider-mattermost/commit/7c18ee6ad1857616a98ac1441e3592c0d6d6ffda))
* register bot and OAuth app resources ([cbf5a25](https://github.com/bambamboole/pulumi-provider-mattermost/commit/cbf5a25a6a0f490769ab22163e2d494bc2048dc9))
* register system config resource ([4c6d64e](https://github.com/bambamboole/pulumi-provider-mattermost/commit/4c6d64ecc1ef33530becc613a323c544bcea8316))
* register users and membership resources ([7a13b97](https://github.com/bambamboole/pulumi-provider-mattermost/commit/7a13b97026479e4de48b965216a5533acb28f5c4))
* register webhook resources ([0eb1696](https://github.com/bambamboole/pulumi-provider-mattermost/commit/0eb16960549c2e6d7368aa97a1a081a2ffbcab55))
* scaffold native Mattermost provider ([56876ea](https://github.com/bambamboole/pulumi-provider-mattermost/commit/56876ea143825bb2787215b14ca31645e0d6e28b))


### Bug Fixes

* align resources with Mattermost client API ([ceb7f6e](https://github.com/bambamboole/pulumi-provider-mattermost/commit/ceb7f6ee3cb01febdbb733b9a87e618e16a0a20e))
* align system config with Mattermost client version ([f6c43fd](https://github.com/bambamboole/pulumi-provider-mattermost/commit/f6c43fd8d1016fb30fc94f09530ddbc9fed4377e))
* convert Mattermost channel type ([e89acb1](https://github.com/bambamboole/pulumi-provider-mattermost/commit/e89acb13dc7b2823b63567950b4e44481d3560cc))
* drop reserved id output and update through patch endpoints ([577f641](https://github.com/bambamboole/pulumi-provider-mattermost/commit/577f64199f07026a0cb40c1e4e969685df3b80cb))
