# Fitty

Fitty is being built as a private fitness companion for iOS and the web: one chat per day for food, exercise, walking pad sessions, and everyday questions. The OpenAI API supports calorie and nutrition tracking, advice, and photo analysis.

## Project status

As of September 17, 2026, login, profiles, daily chats, and text and photo tracking are available:

- Private email/password login with Supabase Auth and persistent sessions.
- Daily chat with free text input, date navigation, and paginated history.
- Daily drafts containing text and photos remain on this device after reloading, restarting the app, and signing in again; pending sends can be resumed without duplicates.
- Messages and profiles are stored in PostgreSQL through Go and restored after reloading or signing in again.
- Profiles with a name, time zone, goals, and dietary preferences.
- Optional calorie and macronutrient targets with progress indicators; past days retain their historical targets, which also provide daily context for the AI.
- Responsive desktop and mobile interface; a shared Expo codebase for iOS and the web.
- AI responses, food and activity entries, corrections through chat, and daily totals from the database.
- Direct editing and removal of entries from the daily overview, with protection against stale changes and repeated requests.
- Deletion of entire daily chats after confirmation, including entries, change histories, and private photos.
- Clearly marked estimates, separate activity calories, and retries after processing errors.
- Up to four photos per message, camera capture, image selection, and pasting from the browser clipboard; private storage, previews, and enlarged views.
- A separate private progress photo history with capture dates, viewing angles, and comparison of two photos.

**Verification status:** Text tracking and photos have been tested with PostgreSQL, private Storage, and real `gpt-5-mini` responses. Image tests cover food estimates, label calculations, corrections based on a previous photo, workout displays, menu advice, and an unreadable image. Desktop and mobile browsers have been tested; camera/HEIC testing on a physical iPhone and Coolify deployment remain pending. Without an OpenAI key, Fitty remains usable as a diary.

## Stack

| Area | Technology |
| --- | --- |
| iOS and web | React Native, Expo, TypeScript, Expo Router |
| Application logic | Go with pgx |
| Database | PostgreSQL within Supabase |
| Authentication | Supabase Auth |
| Images | Supabase Storage with a private bucket |
| AI | OpenAI Responses API through Go, configurable model |
| Hosting, planned | Self-hosted through Coolify |

## Run locally

Requirements: Node.js **24.21.0 LTS** (see `.node-version`), npm 11, Go **1.27**, and Docker.

```bash
npm ci
npm run db:start
npm run db:migrate
npm run setup:local
```

On the current machine, use the existing network instead of `npm run db:start`:

```bash
npm run db:start -- --network-id fitty-local
```

Setup creates a private development account and ignored `.env` files. Credentials are stored exclusively in **`.local/zugang.txt`**. Running setup again reuses the same local account and rewrites the local configuration. It is intended only for the local Supabase instance.

Start these in two terminals:

```bash
npm run dev:api
```

```bash
npm run dev:web
```

**Interface: http://localhost:8788** · Health check: http://127.0.0.1:8787/healthz · Supabase Studio: http://127.0.0.1:54323

## Enable AI

Add the following to the ignored `server/.env` file:

```dotenv
OPENAI_API_KEY=your_api_key
FITTY_OPENAI_MODEL=gpt-5-mini
```

Then restart the Go API. Never put the key in the client configuration or commit it. `gpt-5-mini` has been tested locally with four real example messages; the model remains configurable. Verify model access on each new installation.

New messages are processed automatically after saving. Messages previously saved without AI can be processed individually using **Analyze** (the German UI label is **“Auswerten”**). Advice and planned meals do not create tracking entries; verifiable quantity changes update existing entries. Daily totals come from stored entries, not from the response text.

Analysis sends the profile, targets effective on the selected day, daily chat, relevant tracking data, and any image data to OpenAI. API usage is billed separately from the ChatGPT subscription.

## Set daily targets

Open **Set daily targets** (**“Tagesziele festlegen”**) in the daily overview or **Your daily targets** (**“Deine Tagesziele”**) in the profile. Calories, protein, carbohydrates, and fat are individually optional; an empty field means no target. **Save daily targets** (**“Tagesziele speichern”**) applies changes from today in your saved profile time zone. Past days retain their historical targets.

The daily card shows intake, targets, and progress. Activity calories remain separate. Fitty uses the configured targets as context for advice; it does not calculate target recommendations or change them through chat. Technical details are in the [development guide](docs/DEVELOPMENT.md#personal-daily-targets).

## Use photos

Choose **Photos** (**“Fotos”**) or **Camera** (**“Kamera”**) in the chat; in the browser, images can also be pasted into the message field. Add text such as “My lunch,” “I ate 150 g of this,” or a question about a menu. Image-only messages are also supported; Fitty asks a follow-up question if consumption is unclear.

`npm run setup:local` also stores the private Storage credentials as `FITTY_SUPABASE_SERVICE_ROLE_KEY` in `server/.env`. For an existing installation, apply the migrations first, rerun local setup, and restart Go; the existing OpenAI key is preserved. The service key must never enter the client configuration.

The app converts images up to 20 MiB to JPEG with a maximum of 2,048 pixels per side. Go validates every upload, removes metadata, and stores at most 8 MiB per image. Unused uploads are cleaned up after 24 hours. Details and limitations are in the [development guide](docs/DEVELOPMENT.md#photos-and-image-analysis).

## Compare progress photos

Open **Progress photos** (**“Fortschrittsfotos”**) in the navigation, capture or select a photo, and set its capture date and viewing angle. Save new photos regularly and select two from the history as **Photo A** (**“Foto A”**) and **Photo B** (**“Foto B”**). The comparison shows both complete images; tap to enlarge them. A filter helps you find, for example, two front-view photos.

The images remain private to your account and are not sent to the AI. They have their own history, independent of daily chats. Individual progress photos can be removed after confirmation. Unsaved photo selections remain only in the open view. Details are in the [development guide](docs/DEVELOPMENT.md#progress-photos).

## Check and build

```bash
npm run check
npm run test:local
npm run test:photos
npm run test:tracking
npm run build:web
npm run bundle:ios
npm run db:lint
```

`test:tracking` tests the workers without model calls; stop the running Go server first. `test:local` and `test:photos` require Supabase and the Go API to be running and explicitly save without AI analysis. They use temporary accounts and remove their test data afterward. `bundle:ios` creates a JavaScript/Hermes bundle, not a signed, installable app.

## Documentation

- [Implementation plan](docs/PLAN.md): requirements and milestones.
- [Architecture](docs/ARCHITECTURE.md): data model, access, AI, and images.
- [Development](docs/DEVELOPMENT.md): configuration, iPhone connectivity, and verification status.
- [Deployment](deploy/README.md): planned Coolify hosting.
