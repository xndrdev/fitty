# Deployment

Login, profiles, and persistent daily chats are currently set up locally. Deployment through Coolify follows in milestone 6 of the [plan](../docs/PLAN.md).

The planned services are the Go backend, the Expo web export, and self-hosted Supabase with Auth, PostgreSQL, and Storage. Production values and credentials are managed in Coolify.

`supabase/config.toml` configures only the local Supabase CLI. This file does not configure a Coolify Supabase stack. The versioned SQL migrations apply to both environments; the PostgreSQL version and Storage schema will be checked for compatibility before the first deployment.

The local environment contains development credentials and does not replace a production configuration. Login, user ownership checks, and the restricted database account are implemented. Production accounts, HTTPS, production access configuration, and backups including image files will be set up before release.
