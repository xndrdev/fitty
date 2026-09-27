# Local development

As of September 26, 2026. Login, profiles, personal daily targets, daily chats, AI text processing, private photo attachments with image analysis, and a separate progress photo comparison are available. Mobile keyboard handling has been revised following initial iPhone feedback. Coolify hosting and full acceptance testing on a physical iPhone will follow. Detailed verification status is documented below.

## Versions

| Tool | Version |
| --- | --- |
| Node.js / npm | 24.21.0 LTS / 11.12.1 |
| Expo | SDK 57, package 57.0.23 |
| React / React Native | 19.2.3 / 0.86.3 |
| TypeScript | 6.0.x |
| Go / pgx | 1.27 / 5.11.0 |
| Supabase CLI / JavaScript client | 2.117.0 / 2.116.0 |
| Local PostgreSQL | Major version 17 |

`react-native-reanimated` and `react-native-worklets` are pinned through root package overrides to the versions expected by Expo. Update them together when upgrading Expo. Session management uses the current Supabase library's coordination mechanism without the now-obsolete custom `lock` option.

## Installation and startup

Activate the Node version from `.node-version`, then run:

```bash
npm ci
npm run db:start
npm run db:migrate
npm run setup:local
```

On this machine, start with `npm run db:start -- --network-id fitty-local`; see the Docker section below.

`setup:local` creates a private `fitty@example.test` account with a random password, sets a dedicated password for the restricted database role, and writes:

- `.local/zugang.txt`: credentials for the preview.
- `.local/account.json`: credentials to reuse during the next local setup.
- `server/.env`: Go configuration, including the database password.
- `apps/client/.env`: public URLs and the public Supabase key.

These files are excluded from Git and created with file mode 0600. Setup is limited to the local instance, briefly uses the local admin key to create the account, and does not store that key in the app. Running it again rewrites the local configuration; reapply any custom LAN adjustments afterward. Existing `OPENAI_API_KEY` and `FITTY_OPENAI_MODEL` lines are preserved. Do not use it to manage production configuration.

Then run these in separate terminals:

```bash
npm run dev:api
```

```bash
npm run dev:web
```

Interface: **http://localhost:8788**. Sign in using the credentials from `.local/zugang.txt`. Messages appear with a timestamp after successful storage; a new day is created when the first message is sent. Today (the German UI label is “Heute”) follows the profile time zone. An already open day does not switch automatically at midnight while you are typing.

| Command | Purpose |
| --- | --- |
| `npm run dev` | Expo development server for iPhone and the web |
| `npm run db:stop` | Stop Supabase while retaining data |
| `npm run db:status` | Show local URLs and keys; treat the output as confidential |
| `npm run db:migrate` | Apply new SQL migrations |
| `npm run db:reset` | Delete local data and rebuild the schema |
| `npm run setup:local` | Set up local access and configuration |
| `npm run test:local` | Test auth/chat against Go and Supabase, explicitly without AI jobs |
| `npm run test:tracking` | Test tracking workers with controlled responses against PostgreSQL |
| `npm run test:targets` | Test optional targets, decimal input, and progress calculations |
| `npm run test:progress-photos` | Test calendar dates and merging paginated photo histories |
| `npm run test:keyboard` | Test keyboard dismissal, focused-field scrolling, and composer menu lifecycle |

After an intentional `db:reset`, rerun setup to recreate the local account. Do not use a reset for normal updates.

## Configuration

`npm run dev:api` loads `server/.env` through a Node startup helper. Existing process environment variables take precedence. A compiled Go binary reads only its process environment. Expo loads `apps/client/.env`; restart the development server after changes.

| Variable | Usage |
| --- | --- |
| `FITTY_ADDR` | Bind address, default `127.0.0.1:8787` |
| `FITTY_ALLOWED_ORIGINS` | Comma-separated browser origins; locally `http://localhost:8788,http://127.0.0.1:8788` |
| `FITTY_DATABASE_URL` | PostgreSQL connection for `fitty_api`; required and secret |
| `FITTY_SUPABASE_URL` | Supabase Auth and Storage as seen by the Go server |
| `FITTY_SUPABASE_KEY` | Public/anon key for authentication checks |
| `FITTY_SUPABASE_SERVICE_ROLE_KEY` | Secret, used only by the Go server for the private image bucket; photo uploads are disabled when absent |
| `OPENAI_API_KEY` | Secret, server-side only; no AI worker when absent |
| `FITTY_OPENAI_MODEL` | Responses API model, default `gpt-5-mini` |
| `EXPO_PUBLIC_API_URL` | Go API as seen by the client |
| `EXPO_PUBLIC_SUPABASE_URL` | Supabase Auth and signed image retrieval as seen by the client |
| `EXPO_PUBLIC_SUPABASE_KEY` | Public/anon key, never a service-role or OpenAI key |

`/healthz` reports whether the Go process is running and AI is configured. Go checks the database connection at startup; a later successful health check is not a database, Auth, or Storage readiness test.

## Authentication and persistence

The global setting `auth.enable_signup = false` disables public registration. In this CLI version, `auth.email.enable_signup = true` is required to keep the email/password provider available for login. Integration tests verify both disabled registration and enabled email login.

Go validates each access token through `GET /auth/v1/user` on the configured Supabase Auth service. It does not accept unverified JWT claims. Invalid sessions return HTTP 401; an unreachable Auth instance returns HTTP 503. All domain queries are bound to the verified user ID. Sessions persist through the Supabase library in the browser and AsyncStorage on iOS; signing out ends the local session.

Migrations create the `fitty` schema with `profiles`, `days`, and `messages`, without exposing it through PostgREST. The `fitty_api` role has only the required read/write permissions; clients and the `anon`/`authenticated` roles have no direct access. A composite foreign key binds each message to a day and user. Only one day is allowed per user/date.

| Endpoint | Function |
| --- | --- |
| `GET /v1/profile` | Load the user's profile or defaults |
| `PUT /v1/profile` | Save name, time zone, free-text goals, and preferences |
| `GET /v1/targets` | Load today's numeric targets, today's date, effective date, and version |
| `PUT /v1/targets` | Save numeric targets effective from today with a version check |
| `GET /v1/progress-photos` | Load the user's progress photo history, optionally filtered by viewing angle |
| `PUT /v1/progress-photos/{id}?date=YYYY-MM-DD&view=front` | Upload a progress photo with date/viewing angle; repeatable with the same UUID and data |
| `GET /v1/progress-photos/{id}/url` | Issue a private image path for the user's progress photo |
| `DELETE /v1/progress-photos/{id}` | Remove the user's progress photo; repeatable |
| `GET /v1/days?before=YYYY-MM-DD` | Up to 30 of the user's days, in descending order |
| `GET /v1/days/{date}/messages?before={id}` | Up to 100 messages, displayed chronologically |
| `POST /v1/days/{date}/messages` | Save a message and analyze it automatically when AI is available; optionally save only with `analyze:false` |
| `GET /v1/days/{date}/summary` | Entries, daily totals, effective targets, processing status, and AI availability |
| `PUT /v1/days/{date}/entries/{id}` | Directly edit the user's entry with a version check |
| `DELETE /v1/days/{date}/entries/{id}` | Remove the user's entry from the daily overview with a version check |
| `POST /v1/messages/{id}/analysis/retry` | Process the user's previously unanalyzed message or retry after an error |
| `POST /v1/messages/{id}/analysis/skip` | Deliberately skip a failed analysis |
| `PUT /v1/days/{date}/attachments/{id}` | Upload a binary image; repeatable with the same UUID and identical image data |
| `GET /v1/attachments/{id}/url` | Issue an image path valid for ten minutes after checking ownership |
| `DELETE /v1/attachments/{id}` | Remove the user's upload that is not yet attached to a message |
| `GET /v1/days/{date}/deletion` | Load the current day state and deletion scope |
| `DELETE /v1/days/{date}` | Delete a confirmed daily chat together with its entries and photos |

Both lists return `next_before` when more results exist. Messages are limited to 8,000 characters. The same operation is stored only once through `(user_id, client_id)`, even with concurrent retries. Different content or a different day using the same ID returns HTTP 409. The interface retains an unconfirmed ID when resending; input remains locked until confirmation. Text, photo selections, and unconfirmed send operations are stored on this device per account and date. A pending operation remains repeatable with the same ID after a restart. There is no automatic background sending or cross-device draft synchronization.

`POST /v1/days/{date}/messages` also accepts `attachment_ids` containing at most four of the user's completed uploads for the same day. At least one image is required when there is no text. Retrying a message ID requires identical text, day, and ordered image selection.

## Personal daily targets

Migration `20260917150000_daily_targets.sql` adds `profiles.targets_version`, `daily_targets`, and `daily_target_changes`. Restart the Go API after `npm run db:migrate`. The existing profile endpoint retains its contract; its save operations do not change numeric targets.

In the profile, **Save daily targets** (**“Tagesziele speichern”**) saves the four optional targets independently of **Save profile** (**“Profil speichern”**). Decimal commas and decimal points are allowed; thousands separators are not. Empty fields are stored as `null`; `0` is not a target value. Values must be positive and have at most two decimal places. Technical upper bounds are 20,000 kcal and 2,000 g per macronutrient; these are validation limits, not recommendations. Calories and macros are not derived from one another. Bars show only configured targets; excess amounts remain visible as text while the bar stops at 100 percent. Activity calories do not change the food target.

`GET /v1/targets` returns `{version,today,effective_from,targets}`. `today` comes from the saved profile time zone; `effective_from` is `null` if no target set exists yet. `targets` always contains `calories`, `protein_g`, `carbs_g`, and `fat_g`, each as a number or `null`.

`PUT /v1/targets` expects `{request_id,version,effective_from,targets}` with a UUID, the loaded version, the loaded `today` as the effective date, and all four fields. Success returns HTTP 200 using the current GET format. An invalid value object returns HTTP 400. A stale version, a date that has changed in the meantime, or a request ID used with different data returns HTTP 409. The interface preserves the user's input and requires loading and explicitly accepting the new state before saving again.

Changes apply from today. Historical days load the latest target set effective on or before their date; multiple changes on a day replace that day's set. Clearing all four targets removes them from today onward without deleting earlier targets. `GET /v1/days/{date}/summary` returns this set as `targets` and its date as `targets_effective_from`, even when that day has no messages. Deleting a daily chat preserves targets and their history.

After an uncertain network response, the same save operation remains repeatable in the open form with the same UUID and values. Already confirmed retries make no further changes and return the currently effective state, including after subsequent changes or a date change. Fields and navigation remain locked until resolved. Unsaved targets and unconfirmed target save operations exist only in memory; an app restart reloads the server state. Persistent daily chat drafts are preserved when opening the profile.

Go loads the targets effective on the chat date as `daily_targets` for the AI. The instructions use them alongside `daily_totals` for advice, but do not treat targets as consumed quantities or allow automatic target changes. The model receives no derived target values for empty fields.

## Progress photos

Migration `20260917170000_progress_photos.sql` adds `progress_photos` and `progress_photo_tombstones`. Restart the Go API after `npm run db:migrate`. The existing private bucket and its server configuration are used. The `<user>/progress/` object directory separates progress photos from chat images; direct database access by the client remains blocked.

In **Progress photos** (**“Fortschrittsfotos”**), capture or select a photo. The capture date defaults to today in the profile time zone; past dates from 1900 onward are allowed. Viewing angles are `front`, `side`, `back`, and `other`. The image is uploaded only when **Save progress photo** (**“Fortschrittsfoto speichern”**) is selected. The same preparation/size limits apply as for chat images: selected files up to 20 MiB, client-side JPEG with a maximum of 2,048 pixels per side; server-side up to 8 MiB, at most 12 megapixels/4,096 pixels per side, and JPEG re-encoding without EXIF metadata.

`PUT` sends the binary image and returns `{photo:{id,date,view,mime_type,byte_size,width,height}}`. The UUID, date, viewing angle, and normalized image data remain identical on retry; changed metadata or a retired ID returns HTTP 409. A new upload with an invalid date, a future capture date, an unknown viewing angle, or an unreadable image returns HTTP 400; oversized files return HTTP 413. An already reserved photo can still be confirmed after a time zone change, even if its original capture date now appears to be tomorrow. On Storage errors, the reservation is retained and HTTP 503 allows a deliberate retry.

`GET /v1/progress-photos` returns `{photos:[],next_before,photos_enabled,pending_deletions}` with at most 24 completed photos, ordered by date and UUID descending. `view` optionally filters by one of the four viewing angles. `before` accepts the URL-encoded cursor from `next_before`, which is `null` when no more photos exist. Pending or removed images are not shown. The client allows two different photos to be selected as A and B, including across pages or filters. Both show the date/viewing angle and complete image; tapping opens an enlarged view. The comparison selection itself is not persisted.

`GET /{id}/url` returns `{path,expires_in:600}`. New links are issued only after checking ownership and only for completed, non-deleted photos. Previously issued links may continue working until expiry or physical file deletion. Already downloaded copies and backups are unaffected by deletion.

The **Permanently remove photo** (**“Foto endgültig entfernen”**) confirmation sends `DELETE /{id}` without a body. HTTP 200 with `{status:"removed"}` confirms removal from view and the persistent cleanup task. Repeating the same user-owned ID succeeds even after physical deletion; other users' or unknown IDs return HTTP 404. Correct the date or viewing angle by removing and re-adding the photo. Cleanup checks deletion tasks every five seconds; interrupted uploads older than 24 hours and photos from deleted accounts are checked at startup and hourly afterward. A Storage outage preserves the tasks for later retries. Completed photos are not deleted because of their age. Retired UUIDs remain until account deletion.

Unsaved selections and pending upload/delete requests exist only in the open view. After restarting, check the saved photo history; uploads do not resume automatically. In the open form, an uncertain upload response remains repeatable with exactly the same image bytes, ID, date, and viewing angle; navigation is locked until confirmation. An unsent photo can be discarded after confirmation. Existing daily chat drafts survive switching sections. Progress photos are not sent to OpenAI and do not count as daily tracking.

## Persistent daily drafts

Text and up to four prepared JPEG photos are saved automatically while editing, per account and date on the respective device. The status distinguishes saving in progress, completed storage, and errors. After reloading or signing in again, drafts are available in the history under Drafts on this device (“Entwürfe auf diesem Gerät”). The app still opens today by default; drafts for other days can be selected from the list.

In the browser, IndexedDB (`fitty-drafts`, store `drafts`) saves metadata and image blobs atomically in one record. Reopening creates new preview URLs from the stored image data. Unchanged photos are reused from the local blob cache while typing. On iOS, AsyncStorage holds metadata; photo files reside under `Paths.document/fitty-drafts/<user>/<date>/`. Relative file paths are resolved against the current document directory when loading. Removed native files are cleaned up only after a confirmed metadata commit. Releasing a preview deletes only temporary cache files or blob URLs.

Each state has a random revision. Saving replaces only the loaded revision; even an empty draft remains as a content-free version record. This prevents two browser tabs from silently overwriting each other. On conflicts, the local text remains visible. Use saved draft (“Gespeicherten Entwurf übernehmen”) loads the current state; Save my draft (“Meinen Entwurf speichern”) replaces it after the explicitly labeled choice. A saved pending send/delete operation must first be loaded and completed. On small displays, the conflict notice scrolls within its own area.

Before the first upload, the complete send operation is saved with its message ID, content, and ordered image selection. Uploads or a lost response can be retried with the same identifiers. Opening the app or reconnecting does not send automatically. The draft is cleared only after server confirmation and successful local completion. If another tab has already completed the operation and started a new draft, that draft is preserved. A confirmed daily chat deletion is also stored locally before the server request and can be confirmed identically after restart.

Discard draft (“Entwurf verwerfen”) requires separate confirmation and removes that day's local text and photo selection. An already pending send operation remains locked until resolved. A confirmed daily chat deletion also removes the associated local draft. Signing out preserves saved drafts; another account does not load them. Unsaved changes are stored before signing out. Unattached server-side upload remnants are removed where possible; otherwise the existing 24-hour cleanup applies.

**Limitations:** Drafts do not synchronize between devices. They are local app/browser data, not separately encrypted storage. Clearing browser data, uninstalling, or losing the device may remove them. If storage is full or unavailable, the current text remains visible and Fitty offers to save again; keep the app open until confirmation. The browser warns before leaving with unsaved changes. An immediate process termination may lose changes made since the last completed save. In an open session, the interface can continue editing drafts without an API connection; fully offline startup, including the session, profile, app files, and history, is not yet implemented. Direct entry editing dialogs still exist only in memory.

`npm run test:drafts` tests native storage with controlled AsyncStorage/filesystem adapters: concurrent writes, empty version records, relative paths after restart, account isolation, missing images, copy errors, and uncertain metadata commits. The test is part of `npm run check`. The implementation uses the existing SDK 57 dependencies and is based on the [official Expo FileSystem documentation](https://docs.expo.dev/versions/latest/sdk/filesystem/) and installed API definitions.

## Direct entry changes

Migration `20260917090000_manual_entries.sql` adds `tracking_entries.version` and `manual_entry_changes`. Restart Go after the migration. Each entry in the daily overview includes its current `version`. AI corrections, direct changes, and deletions increment this version.

`PUT /v1/days/{date}/entries/{id}` expects `{request_id, version, entry}`. `request_id` is a UUID, `version` is the entry version loaded when opening the dialog, and `entry` is the complete value object without `id` or `version`. The type (`food`/`activity`) remains immutable. Numbers are non-negative and have at most two decimal places. Food requires calories and all three macros; activities require at least duration or distance. An empty optional numeric field means `null`; `0` is a known value. The quantity description does not automatically recalculate nutritional values: all values apply to the entire specified quantity. The source remains explicitly selectable; editing an estimate does not automatically make it exact.

`DELETE` on the same path expects `{request_id, version}`. Confirmation removes the entry from the daily overview and its totals. Earlier chat messages and their images remain. Internally, the server sets `deleted_at`; this is not a complete deletion of personal data. Original values remain in the change log. Entire chats, including this history and images, can be removed through separate daily chat deletion. A backup retention policy is still pending.

Success returns HTTP 200 with `{status:"updated"}` or `{status:"deleted"}`. Identical retries using the same UUID do not change the entry again, even if an earlier response was lost. A UUID with different content, a stale version, or pending AI analysis for that day returns HTTP 409; an unavailable entry or one belonging to another user returns HTTP 404. After an uncertain network response, the retry operation remains unchanged in the open dialog. Conflicts require explicitly accepting the current values. The dialog draft does not persist across an app restart.

The short database transaction coordinates with worker job claiming through the same advisory lock and locks the day against concurrent messages/changes. New direct changes are blocked during `queued` or `processing`. Ownership, the current entry version, and type are then checked; the change and audit record are committed together. The manual log contains the user, request ID, expected version, and before/after values. `updated_by_message_id = null` marks a direct change; the source message remains associated. The next AI job receives the current active entries and recalculated totals.

`npm run test:tracking` also checks direct corrections/deletions, versions after AI changes, user/day boundaries, unknown values versus a known zero, unchanged historical messages, audit records, parallel retries, and concurrent edits. Briefly stop the local API as described in the testing section. Tests use temporary accounts and controlled AI without OpenAI calls.

## Full daily chat deletion

Migration `20260917120000_day_deletion.sql` adds day versions, content-free deletion records/blocked UUIDs, and persistent photo deletion tasks. Restart the API after migration.

`GET /v1/days/{date}/deletion` returns `{day:{id,version,message_count,entry_count,photo_count}|null,pending_photo_deletions}`. The photo count includes unattached and interrupted uploads for that day. `GET /summary` includes the same additional fields; `GET /messages` returns `day_id` or `null`. Messages and image associations are loaded in a consistent read transaction.

`DELETE /v1/days/{date}` expects `{request_id,day_id,version}`. Success returns `{status:"deleted",pending_photo_deletions}`. HTTP 409 requires a fresh preview and another confirmation; HTTP 404 means the user's daily chat no longer exists. Identical retries of an already confirmed request ID continue returning HTTP 200, even if a new chat has already been created for the same date. Invalid input returns HTTP 400. All queries and deletions are bound to the authenticated user.

Messages, all entries including previously removed entries, both change histories, and analysis jobs disappear in one transaction. Photo tasks are stored persistently and attempted by the worker every five seconds. Storage errors preserve the association; new photo URLs and retries with old upload/message IDs are blocked. An already issued image link may work until physical removal or expiry. External AI calls already in progress cannot be recalled, but no later response is added to the deleted day.

The dialog shows the date and scope, warns that local drafts will be discarded, and locks other controls while open. After an uncertain network response, exactly the same request remains repeatable. Successful deletion discards only the current local daily draft and its photo selection; the day remains open for new messages. Other sessions detect the deleted day ID on their next poll and also discard older loaded messages, while retaining unsent text. The already confirmed deletion operation is stored locally before the request. After restart, opening the affected day shows the retry dialog with the same request ID; successful confirmation also removes the persistently stored local draft.

**Retention:** Deletion records and retired UUIDs remain until account deletion, without chat text, nutritional values, or image content. Deletion removes the active dataset, not previously downloaded files or existing backups. Automatic backups, retention periods, and the procedure for reapplying deletions after restoration must be configured before production use.

`npm run test:tracking` checks cascades, version conflicts caused by messages/AI/manual changes, user boundaries, parallel deletion retries, protection of newly created days, delayed AI/message/upload requests, and photo cleanup after Storage failure and worker restart. Tests use temporary accounts and controlled providers; stop the local API first.

## Photos and image analysis

Local setup adds `FITTY_SUPABASE_SERVICE_ROLE_KEY` exclusively to `server/.env`. For an existing checkout, run the migrations and `npm run setup:local`; existing OpenAI settings are preserved. Restart Go afterward. The client receives neither the Storage service key nor the OpenAI key.

- **Selection:** Up to four images per message from the photo library/file picker or camera; in the browser, also from the clipboard into the message field. Text and photo selections remain in the local draft when changing days, reloading, or restarting the app. Unsent images can be removed.
- **Preparation:** Source files up to 20 MiB, JPEG with a maximum of 2,048 pixels per side and quality factor 0.9. Native selection requests the compatible iOS representation. Browsers without HEIC support require JPEG/PNG/WebP; native HEIC conversion still needs testing on a physical iPhone.
- **Server validation:** JPEG, PNG, or WebP based on actual data and declared MIME type, at most 8 MiB, 4,096 pixels per side, and 12 megapixels. The server fully decodes and stores the image after JPEG re-encoding without EXIF/location metadata. The private `chat-attachments` bucket accepts only the prepared JPEG files.
- **Retries:** A persistent `pending` reservation becomes `ready` after successful Storage upload. UUID and SHA-256 protect against different content on retry. The message transaction associates all images together; an incomplete image selection does not create a partial message.
- **Retrieval:** Ownership check through Go, followed by a signed Storage path valid for ten minutes. The preview refreshes the link after nine minutes and offers a retry on retrieval errors. Such a link grants access until expiry and must not appear in logs or public messages.
- **Cleanup:** At API startup and hourly, at most 50 unattached uploads aged 24 hours or more and files from deleted accounts are cleaned up. Storage errors retain the database row for the next attempt. Images still referenced are preserved. Photos marked by daily chat deletion are additionally attempted every five seconds in batches of at most 50 files.

The analysis worker loads at most four images from the current message. For text without new images, it instead loads the latest image message from the at most 30 history messages considered for that day. This keeps follow-up questions and corrections based on earlier labels possible; currently these images are also resent for other text questions. Image data is sent to OpenAI as Base64 `input_image` with `detail: high`, without making the bucket public.

Plate/food photos produce clearly marked estimates. Readable labels are calculated against the stated consumed quantity. Workout displays can provide device readings; a menu remains advice. For unreadable images or unclear consumption, Fitty asks a follow-up question. A historical image alone must not justify a new entry. Several images of one meal do not represent several meals.

Prepared photos and the identity of a started send operation are persisted locally. There is no automatic upload queue, no resuming within a single file, no total storage limit, and no independent nutrition database. Interrupted uploads are repeated in full. Images are not stored as separate thumbnail files for previews; the same prepared file is used.

## AI text processing

Set your own `OPENAI_API_KEY` in `server/.env` and restart `npm run dev:api`. The key must never appear in an `EXPO_PUBLIC_` value. The default model is `gpt-5-mini`; `FITTY_OPENAI_MODEL` can select another Responses-compatible model with Structured Outputs. Reasoning is set to `low` and output is limited to at most 8,192 tokens. These parameters must be supported by the selected model.

The API saves a message immediately. A persistent job then processes it without requiring the app to remain open. The client periodically reloads messages and daily totals. Messages saved before AI activation are **not automatically** sent to OpenAI; they can be processed individually using Analyze (“Auswerten”).

The context contains:

- The profile with goals and dietary preferences.
- Numeric targets effective on the selected chat date; unset targets remain `null`.
- The explicitly selected calendar day and current message.
- Up to 30 preceding messages, totaling at most roughly 48 KB of history text.
- Up to 200 active entries and server-calculated daily totals.
- Where applicable, the image data and message associations described in the image analysis section.

The model returns response text and structured proposals to create, correct, or remove entries. The schema requires complete entries for creation/changes and `null` exclusively for removal. The server checks fields, units, value ranges, types, user-owned entries, and verbatim text evidence or exactly matched current image evidence. Advice and follow-up questions must not contain write proposals. This protects the data structure; domain interpretation and nutritional quality still need evaluation with real examples.

Food and activities reside in `tracking_entries`; PostgreSQL stores nutritional values and measurements as `numeric` with two decimal places. Daily totals are summed directly from these values. A quantity change replaces the entry; removal marks it as deleted. `entry_changes` records before/after values and the source message. Entries, change log, response, and completion status are written in **one** transaction. Errors or incomplete model responses create no entries.

One worker runs per Go process. Messages for the same day are processed sequentially. A failed job holds back later messages for that day until it is retried or explicitly skipped. Notes saved without AI do not block later messages. At most three attempts are allowed per message; ordinary provider errors do not trigger automatic paid retries. Interrupted processes are detected through a two-minute lease and resumed within the remaining attempt limit. A new lease ID prevents obsolete results from being applied later.

The processing attempt timeout is 95 seconds; the HTTP client limits the model call to 90 seconds. At most 20 changes per response and 200 active entries per day are supported. The status endpoint returns the last 1,000 analysis states for a day. Automatic offline sending in the client, streaming, unrestricted AI queries across days, and a dedicated monetary daily cap are not yet included in this version.

The Responses API is used with `store:false`. This disables retrievable storage of the response as Responses application state; it does not guarantee the complete absence of provider-side retention. The profile and chat context are sent to OpenAI for processing. [OpenAI: Data use and retention](https://platform.openai.com/docs/guides/your-data)

## Open on an iPhone

For the first device test, install Expo Go on the iPhone. The project uses SDK 57; the Expo Go version must match. The [Expo Go download page](https://expo.dev/go) shows the currently supported version. A dedicated development build is a separate later step.

1. Connect the iPhone and development machine to the same private Wi-Fi network. Both devices must remain connected during the test.
2. Start Supabase if it is not running. Find the computer's Wi-Fi address using `ip -br -4 addr`; use the Wi-Fi interface address, not a Docker, VPN, or `127.0.0.1` address.
3. Stop any running Go/Expo development servers on 8787/8788 first, then run the following commands with Node 24 from the repository root in two terminals. The temporary process environment variables leave the `.env` files unchanged.

Terminal 1:

```bash
FITTY_ADDR=0.0.0.0:8787 npm run dev:api
```

Terminal 2; replace the example address with your own Wi-Fi address:

```bash
FITTY_LAN_IP=192.168.1.100
REACT_NATIVE_PACKAGER_HOSTNAME="$FITTY_LAN_IP" \
EXPO_PUBLIC_API_URL="http://$FITTY_LAN_IP:8787" \
EXPO_PUBLIC_SUPABASE_URL="http://$FITTY_LAN_IP:54321" \
npm run start --workspace @fitty/client -- --lan --go
```

4. Scan the QR code in the Expo terminal with the iPhone camera and open it in Expo Go. Allow local network access. The link is `exp://<LAN-IP>:8788`. The [official Expo workflow](https://docs.expo.dev/get-started/start-developing/) describes launching through a QR code.
5. Sign in using the existing Fitty account from `.local/zugang.txt`. Then test sending a message, camera/photo selection, progress photo comparison, the keyboard, and recovery of an unsent daily chat draft after restarting the app. Test messages are stored in the signed-in account; chat analyses use the configured AI just as they do in the browser.

If the iPhone has connection problems, first open `http://<LAN-IP>:8787/healthz` in Safari. Expect JSON containing `status: "ok"`. If it is unreachable, check Wi-Fi/guest network isolation, network permissions, and the firewall. For the complete test, 8787 (Go), 8788 (Expo), and 54321 (Supabase Auth/photos) must be reachable on the local network. An Expo tunnel alone does not make Go and Supabase reachable. If Expo Go reports an incompatible SDK version, update the app or use a compatible development build.

For normal desktop development, stop both processes and restart them with `npm run dev:api` and `npm run dev:web`. If the Wi-Fi address changes, restart the test processes with the new address and use the new QR code.

On September 18, 2026, Go, Supabase Auth, the Expo Go manifest, and the iOS development bundle were checked over the local Wi-Fi address from the development machine. Both client server addresses are correctly set to the LAN in the bundle. This does not yet verify connectivity or usability from the actual iPhone; that test is performed by the user.

SDK 57 requires at least iOS 16.4. No local iOS simulator is available on Linux. A physical iPhone, native build, keyboard/safe-area behavior, and ongoing private distribution still need testing. HTTPS addresses will be used for later production web and app access; in particular, the browser requires a secure context (HTTPS or localhost) for secure message IDs.

## Docker network on this machine

The automatic Docker address pools were exhausted. After checking existing networks, a dedicated `fitty-local` network with `10.203.0.0/24` was created. Other networks are not modified.

```bash
npm run db:start -- --network-id fitty-local
```

If this network has been removed, recreate it after checking again for overlapping address ranges:

```bash
docker network create --driver bridge --subnet 10.203.0.0/24 fitty-local
```

The CLI configuration applies only locally and is not automatically transferred to Coolify.

## Verification

```bash
npm run check
npm run test:local
npm run test:photos
npm run test:tracking
npm run test:favorites:integration
npm run build:web
npm run bundle:ios
npm run db:lint
```

`check` runs TypeScript checks, native draft storage tests with controlled adapters, target validation/progress tests, photo capture date and pagination tests, keyboard and modal lifecycle tests, favorite completion tests, Go tests, and `go vet`. Go unit tests cover token validation, calendar dates, time zones and target values, CORS, Responses requests, strict JSON decoding, and domain-level AI validation. `test:local` checks Auth configuration, profiles, user boundaries, persistence, concurrent retries, and both history pagination flows. It uses two temporary accounts and removes them together with dependent data afterward. New messages in this test are explicitly sent with `analyze:false`.

`test:photos` checks private storage against real local Supabase Storage, format preparation, signed retrieval, user/day assignment, image-only messages, retries, and removal. Messages explicitly carry `analyze:false`. Temporary Storage objects and accounts are removed afterward; orphaned metadata is cleaned up by the regular worker no later than the next API startup.

`test:tracking` uses controlled model and Storage adapters with real PostgreSQL persistence, without OpenAI calls. It tests recording entries, corrections, advice, deletion, daily totals, rollback, ordering, leases, image context, follow-up questions, and upload cleanup. Target tests also verify historical validity, user isolation, optional values, stale and concurrent writes, identical retries after subsequent changes or date changes, and the worker's target context. Fully stop the Go server first so neither the AI worker nor cleanup claims test jobs. The script refuses to run if the API is running or its status is unclear; unrelated pending analysis jobs are left untouched. Temporary users are deleted afterward.

The web export is in `apps/client/dist`; the iOS JavaScript/Hermes bundle is in `apps/client/dist/ios`. The latter does not replace an Xcode build and is not an installable app. Also check Expo compatibility from the client directory using `npx expo install --check` and `npx expo-doctor`.

The progress photo cases in `test:tracking` verify private image paths, user isolation, image/date validation, unchanged upload retries, collisions with changed content, retries after time zone changes, viewing angle filters, multiple pages with the same date, separation from chat/AI, and persistent cleanup after Storage failure. Concurrent uploads and a retry deliberately started during cleanup must not duplicate or restore a photo. Existing chat and tracking tests continue in the same run.

### Daily view based on the UI template, September 17, 2026

The basis is `ui-preview/Fitty Tagesansicht.dc.html`; the template remains unchanged as a reference. The client adopts the warm paper colors, Instrument Sans/Serif, white daily card, green messages, and narrow sidebar. Styles are in `apps/client/src/styles/app.ts` and `day-summary.ts`. Fonts and their OFL licenses are in `apps/client/assets/fonts` and are loaded locally.

- At widths of 900 pixels and above, history remains in the sidebar. On mobile, the Fitty logo returns to the daily chat; history and profile remain accessible in a compact header. The calendar button opens date input.
- Daily totals and entry details use only existing API data. Activity calories remain separate; estimates are marked. Target bars use explicitly configured daily targets. No assumed calorie budgets, steps, or weekly statistics are taken from the template.
- Correct in chat (“Im Chat korrigieren”) adds the entry's label and quantity to a draft. The change is analyzed only after deliberate submission. Existing text and photo selections are preserved. Quick actions for meals, activity, and restaurants work the same way. Edit (“Bearbeiten”) and Delete (“Löschen”) also open the direct dialog described in the entry changes section.
- On the web, Enter inserts a new line; Ctrl+Enter or Cmd+Enter sends. Active text composition through an input method does not trigger sending. Photo selection, camera capture, and clipboard access remain available.
- Controls are at least 44 pixels high, with labeled icons and visible keyboard focus. No animations were added. At low window heights, quick actions are omitted to leave room for the chat.

Verification: TypeScript, web export, iOS JavaScript/Hermes export, and local Expo version compatibility checks passed. Chromium at widths of 320, 390, 768, 1024, and 1440 pixels: no horizontal overflow, input and send controls reachable; corrections without automatic sending or draft loss, quick actions, keyboard shortcuts, invalid dates, daily drafts, history, profile saving, reloading, and logout checked. Chat responses and daily totals were controlled UI test data; Auth and profile saving used the local server. A separate browser run tested real private photo uploads, multiple selection, clipboard pasting, removal, day changes, previews, enlargement, and image-only messages against Go/Supabase. No console errors or warnings in the tested flows. Temporary accounts and files were removed; no OpenAI calls were made for UI tests.

Native interaction, the iOS keyboard, and font weights on a physical iPhone remain to be checked. The Hermes export is not an installable iOS app and does not replace device verification.

Direct entries were then tested in the browser at widths of 1280 and 390 pixels using real local API/database/Storage access and temporary synthetic data: decimal commas and invalid input, quantity changes without implicit nutrition recalculation, preserved source information and chat drafts, version conflicts from a second session, explicit acceptance of current values, a response deliberately lost after commit with an identical retry UUID, empty activity calories versus zero, deletion cancellation and confirmation, reloading, and continued access to the source message's photo. No unexpected console errors or warnings; simulated conflict/network responses produce expected browser network errors. TypeScript, Go tests, `go vet`, and PostgreSQL integration including concurrent writers passed. The local migration was applied and the API restarted afterward. No OpenAI calls were made for this acceptance run.

### Completed on September 16, 2026

- TypeScript, Go tests, and `go vet`: passed.
- Local integration against Go and Supabase: passed, including Auth settings, user isolation, parallel retries, and pagination.
- Chromium at widths of 1280 and 390 pixels: login, keyboard operation, message persistence, reloading, day changes, profiles, history, and signing in again passed. A second isolated browser context loads the same saved chat.
- A response deliberately lost after persistence is handled through Send again (“Erneut senden”) without duplication. The regular browser flow produces no console errors or warnings; the simulated HTTP 503 response is reported as an expected network error.
- OpenAI client with a strict schema, rejection of incomplete responses, and nutrition validation: offline tests passed.
- Worker against PostgreSQL: meals, corrections, restaurant advice, unknown activity calories, change logs, repeated commits, rollback, user isolation, and restart leases passed.
- Browser with controlled responses: AI status, response display, daily totals, estimates, and retries checked.
- Live test with `gpt-5-mini` through the running Go API: breakfast with packaging values, quantity correction of the same entry, restaurant advice without an entry, and a walking pad session with unspecified calories passed. Four model calls; measured time from submission to retrieved completion was approximately 8 to 16 seconds each, including the worker and polling.
- Packaging values give exactly 157.5 kcal, 27.5 g protein, 10 g carbohydrates, and 0.5 g fat for 250 g of skyr. Correcting to 150 g gives 94.5 kcal, 16.5 g protein, 6 g carbohydrates, and 0.3 g fat. Resending the completed walking pad message creates neither another response nor an additional entry.
- Real AI responses and daily totals checked in Chromium at widths of 1280 and 390 pixels and after reloading; no console errors or warnings. The live test used only a temporary account with synthetic examples; the account and test data were removed afterward. Existing messages were not analyzed retroactively.
- Photo tests against the real Go API, PostgreSQL, and private Supabase Storage passed: actual format, ownership/day boundaries, signed retrieval, identical upload retries, image-only messages, reloading, and removal. Direct retrieval by other users and through the public bucket path is rejected.
- Controlled worker tests verify current image context, a follow-up question with a later text response and previous image, failed image retrieval without an entry, and cleanup of abandoned uploads. Unit tests also cover JPEG/PNG/WebP, size/dimension limits, EXIF/trailing data, transparency, and the Storage HTTP contract.
- Browser at widths of 1200 and 390 pixels: multiple selection, clipboard pasting, removal, preservation of photo selections when changing days, uploads, previews, enlargement, and image-only messages passed. No console errors or warnings in the tested flow.
- Six live image scenarios with `gpt-5-mini` passed in the final run: banana as an estimate; a label yielding 94.5 kcal/16.5 g protein/6 g carbohydrates/0.3 g fat for 150 g of skyr; correction to 200 g and 126 kcal in the same entry; walking pad with 30 minutes/2 km/120 device-reported kcal; a menu without an entry; an unreadable image with a follow-up question. Resending does not record entries again. Measured completion times including the worker and polling were approximately 4 to 12 seconds in this run. Earlier diagnostic attempts led to a stricter action schema, unambiguous image evidence, and more precise rounding instructions.
- Image acceptance used only temporary test accounts, self-created example images, and [Banana-Single.jpg by Evan-Amos](https://commons.wikimedia.org/wiki/File:Banana-Single.jpg) under [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/); the photo was re-encoded as JPEG for processing. Test accounts and Storage files were removed afterward. The photo is not part of the app or repository.
- Web export and iOS JavaScript/Hermes export: passed.
- New migration applied to the existing local database; Supabase schema linter reported no errors.

### Personal daily targets, September 17, 2026

TypeScript, target/draft unit tests, Go tests, `go vet`, PostgreSQL integration, schema lint, web export, and iOS JavaScript/Hermes export passed. The migration has been applied locally and the API restarted. Integration tests confirm historical targets, user isolation, version conflicts, concurrent writes, identical retries even after date changes, preserved targets after chat deletion, and the applicable target set in AI context. The Responses contract transmits configured and empty target values unchanged.

Chromium against the real Go API, Auth, PostgreSQL, and Storage at widths of 1280, 390, and 320 pixels: decimal commas, optional macros, progress, past days, reloading, preserved text/photo drafts when opening the profile, concurrent target changes, explicit confirmation, retrying the same UUID after a lost success response, and removing all targets with unchanged daily totals passed. An additional browser run with controlled API responses checks excess amounts, accessible progress values, and a day/time zone change with unsaved input. No unexpected console errors or warnings; simulated HTTP conflicts and network interruptions produce expected network messages. Temporary accounts and Storage files were removed. These target tests make no OpenAI calls; the quality of target-based advice and native interaction still need everyday and iPhone testing respectively.

### Progress photos, September 17, 2026

Migration applied locally; TypeScript, unit tests, Go tests, `go vet`, PostgreSQL integration, schema lint, web export, and iOS JavaScript/Hermes export passed. The local API has been started with the new version. Tests use temporary accounts and synthetic images.

Chromium against the real Go API, Supabase Auth, PostgreSQL, and private Storage: upload with date/viewing angle, date boundaries, exactly identical UUID/metadata/JPEG bytes after a lost success response, chronological history, viewing angle filters, A/B comparison, enlargement, deletion cancellation, deletion retries, and physical Storage cleanup passed. Reloading preserves saved photos; an existing daily chat draft survives switching sections and daily totals do not change. Comparison checked at widths of 1280, 390, and 320 pixels without horizontal overflow. An additional mock browser run covers comparison selection across pages and filters, discarding an unsaved selection, and locked navigation after uncertain network responses. No unexpected console errors or warnings; simulated network interruptions produce expected browser messages. Temporary accounts and Storage files were removed without OpenAI calls. Camera capture, native permission dialogs, and interaction on a physical iPhone remain pending.

### Mobile keyboard UX, September 26, 2026

The original chat placed keyboard avoidance below the navigation headers, while login and profile forms had none. Numeric inputs also lacked an explicit keyboard dismissal control. Keyboard avoidance now wraps the screen before safe-area padding; the entry editor has its own wrapper inside its native modal. Forms reveal the active input within the actual scrollable area, including space reserved for fixed save actions. A native “Fertig” button dismisses text and numeric keyboards. Keyboard listeners and native measurements are excluded on the web.

While typing on a phone, the chat hides its navigation and suggestion rows, reduces spacing, and keeps the selected date visible. Draft errors and uncertain send results remain accessible. The composer can scroll when attachments or notices exceed the available height. Navigation uses compact, labelled icon buttons on narrow screens; the entry editor uses shorter save text to keep its actions on one row. Email submission moves focus to the password field.

Verification:

- `npm run check` passed, including four keyboard tests with controlled native measurements/events. These cover dismissal on iOS/Android, listener cleanup, fields below or above the visible region, unrelated forms, and web behavior.
- A Chromium run with isolated Auth/API fixtures passed 26 checks: login focus, chat at widths of 320/390/430/1280 pixels, message sending, profile and target saves, entry editing, and progress photo navigation. Short views at 390 × 430 and 320 × 320 retain reachable fields and save controls. There was no horizontal overflow in the checked pages and no uncaught browser error. These checks did not write to real accounts or call OpenAI.
- TypeScript, web export, and the iOS JavaScript/Hermes export passed. Screenshots and the browser results are stored locally in `.local/ux-review/` and are excluded from Git.

Follow-up after iPhone feedback: the first revision collapsed the chat composer and entry editor's scrollable content on native devices. The shared form wrapper inherited `flex: 1`; combining it with `flexGrow: 0` and `flexBasis: 'auto'` still gives a zero basis in native Yoga. The wrapper now uses explicit grow/shrink properties without the shorthand. A reproduction compiled against the Yoga source shipped with this project's React Native version measured the old chat composer at 1 point (its border) and the editor content at 0. The corrected cases measured 241 and 250 points; oversized chat content also shrank correctly to a 200-point viewport. The reproduction is saved in `.local/ux-review/native-layout-repro.cpp`. This regression was not detected by the earlier browser or mocked keyboard tests.

Physical-device acceptance remains open: reload the project in Expo Go, type a multiline chat message, edit a numeric daily target, and edit an entry near the bottom of its form. Check that the input, “Fertig”, and send/save actions stay reachable with the iPhone keyboard open; repeat with photos attached, in landscape, and after closing/reopening the keyboard. Browser viewport resizing, the Yoga reproduction, and controlled event tests do not reproduce the native keyboard animation, safe-area geometry, or interactive dismissal.

### Compact chat composer, September 26, 2026

The chat now uses one rounded row with a plus button, multiline message input, and an accessible send icon. The plus button opens an overlay containing photo selection, camera capture, meal/activity/restaurant prompts, and draft discard when available. Selecting a prompt preserves existing text and returns focus to the input. On iOS, actions wait for the native modal's dismissal before opening another picker or focusing the input.

Normal autosave notices, persistent suggestion buttons, the date/character-count footer, and the chat's keyboard toolbar no longer take up space. Storage errors, conflicts, uncertain sends, upload progress, and discard confirmation remain visible. Character counts appear near the limit. Profile/login/editor forms retain their keyboard dismissal button. Existing photo previews and removal controls appear only when photos are attached.

`npm run check`, web export, and iOS JavaScript/Hermes export passed. Five keyboard tests and six menu lifecycle tests cover toolbar restoration and deferred actions without duplicate execution. A Chromium run with isolated Auth/API fixtures passed 39 checks, including all prompt actions, preserved drafts, Escape/backdrop dismissal, discard cancellation/confirmation, photo selection through the overlay, the four-photo limit, photo removal, sending, and existing form flows. The normal composer measured 68 pixels high at widths of 320, 390, and 430 pixels, and 78 pixels on desktop. There were no uncaught browser errors.

A separate native Yoga fixture passed 12 geometry cases with narrow screens, multiline-sized input, photos, and restricted available height. It retains the explicit grow/shrink fix described above; this fixture models layout geometry rather than native TextInput measurement or keyboard interaction. Local screenshots, the browser script/results, and the native fixture are in `.local/minimal-composer-review/`. Physical iPhone verification of the new menu, picker transitions, and keyboard remains pending.

### Favorite meals and compact navigation, September 27, 2026

The mobile header is now one row containing a hamburger button and the selected day or section title. Its menu provides the daily chat, history, profile, progress photos, previous/next/today navigation, date selection, chat deletion, and logout. Date input appears only when requested. The desktop sidebar remains available. Menu actions on iOS and web wait for modal dismissal before moving focus or opening another dialog; Android uses the visibility transition because it does not provide the native dismissal callback.

Expand the daily card's entries and use the star on a food entry to save its displayed portion. At least two matching characters in the composer reveal a compact, horizontally scrolling row of up to five favorites. Choosing a suggestion completes the phrase at the caret; text before and after it and attached photos remain intact. The caret moves directly after the inserted portion. The plus menu also opens the favorites list for selection or removal. Insertion never sends automatically, and estimates remain identified as estimates in the editable message. Normal AI processing starts only after sending; exact ledger reuse is not guaranteed. The saved snapshot does not change when the source entry is edited; remove and save again to update it.

Favorites belong to the account and survive deletion of their source day. The account-level client model survives navigation between dates, including an in-flight save. Migration `20260926190000_food_favorites.sql` adds the snapshot table. Apply it with `npm run db:migrate`, then restart the API. The migration has been applied locally and the updated API started. `npm run test:favorites:integration` creates two temporary accounts, checks PostgreSQL persistence, account boundaries, stale versions, snapshot provenance, source/day deletion, unchanged totals, repeated operations, and concurrent saves at the 200-favorite limit. It creates no AI jobs or Storage objects, can run with the API active, and removes the temporary accounts afterward.

TypeScript, unit tests, Go tests, `go vet`, PostgreSQL favorite integration, and schema lint passed. Web and iOS JavaScript/Hermes exports passed. Chromium with isolated Auth/API fixtures passed 51 checks at widths of 320, 390, 430, and 1280 pixels and heights down to 320 pixels. These cover menu/date navigation, retained daily drafts, favorite conflict recovery, reload and removal, a delayed removal across a day change, autocomplete inside existing text, caret placement, no automatic sending, photo selection, and existing profile/target/editor flows. The mobile header measured 57 pixels and the normal composer 68 pixels; no uncaught browser errors occurred. Programmatic scroll events now retain input focus on web, while native drag-to-dismiss behavior remains enabled.

A native Yoga model passed 12 geometry cases, including suggestions, photos, multiline input, and constrained height. Browser screenshots/results and layout artifacts are stored locally in `.local/favorites-header-review/`. These tests do not replace physical iPhone acceptance of keyboard animations, caret placement, or modal transitions.

### Verification limits

- Full physical iPhone acceptance, a standalone native build, and Coolify hosting remain pending.
- Real model access, four text workflows, and six image scenarios have been tested. This is not a comprehensive assessment of portion estimates, advice, or model reliability. Complex meals and poor-quality real labels require additional everyday samples. Label calculations also come from the model and must be checked when in doubt. Measured timings are individual local samples; actual API costs have not been evaluated.
- Camera capture, native permission dialogs, HEIC conversion, and uploads with the actual iPhone keyboard still need device testing despite the successful iOS bundle. Browser testing does not replace this acceptance step.
- `npm audit` reports 13 moderate warnings in the Expo dependency chain (`uuid` and `decode-uri-component`). No compatible automatic fix is offered; check upstream fixes again before production deployment.

## References

- [React Native 0.86: TextInput](https://reactnative.dev/docs/0.86/textinput)
- [React Native 0.86: Modal](https://reactnative.dev/docs/0.86/modal)
- [React Native 0.86: KeyboardAvoidingView](https://reactnative.dev/docs/0.86/keyboardavoidingview)
- [React Native 0.86: ScrollView](https://reactnative.dev/docs/0.86/scrollview)
- [React Native 0.86: Keyboard](https://reactnative.dev/docs/0.86/keyboard)
- [Supabase: React Native authentication](https://supabase.com/docs/guides/auth/quickstarts/react-native)
- [Supabase: Validate tokens with getUser](https://supabase.com/docs/reference/javascript/auth-getuser)
- [Supabase: Create accounts administratively](https://supabase.com/docs/reference/javascript/auth-admin-createuser)
- [Supabase: CLI configuration](https://supabase.com/docs/guides/local-development/cli/config)
- [Supabase: Current session coordination](https://github.com/supabase/supabase-js/blob/master/packages/core/auth-js/migrations/lockless-coordination.md)
- [pgx: Connection pool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool)
- [Expo: ImagePicker](https://docs.expo.dev/versions/latest/sdk/imagepicker/)
- [Expo: ImageManipulator](https://docs.expo.dev/versions/latest/sdk/imagemanipulator/)
- [Expo: FileSystem](https://docs.expo.dev/versions/latest/sdk/filesystem/)
- [OpenAI: Images and vision](https://developers.openai.com/api/docs/guides/images-vision)
- [Expo: SDK versions](https://docs.expo.dev/versions/latest/)

- [OpenAI: Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs)
- [OpenAI: GPT-5 mini](https://developers.openai.com/api/docs/models/gpt-5-mini)


Full daily chat deletion was additionally verified on September 17, 2026: real Go/PostgreSQL/Storage access with temporary test accounts, canceled confirmation, a version conflict after a new message, a deliberately lost success response and identical retry after a new chat had been created, rejection of an old message ID, physical removal of a private image, cleared daily totals, preservation of the profile/another day, two browser tabs, and preserved unsent text in the second tab. Locally confirmed drafts/photos are discarded; writing again and reloading work. The dialog was checked at widths of 1280, 390, and 320 pixels. A separate controlled UI test covers delayed older messages and creation of a new day during retry. The existing photo browser test continues to pass. No unexpected console errors; simulated network interruptions and HTTP conflicts produce expected browser network messages. TypeScript, Go tests, `go vet`, PostgreSQL integration, schema lint, web export, and iOS JavaScript/Hermes export passed. The migration has been applied locally and the API restarted. Temporary accounts and files were removed; no OpenAI calls were made in these tests. Physical iPhone testing and backup retention remain pending.


Persistent drafts were verified on September 17, 2026: six native storage tests with controlled adapters and a real Chromium/IndexedDB test for blob restoration and concurrent tabs. The browser run against Go, PostgreSQL, Auth, and Storage covers text/photo restoration after reloading, separate days, a lost successful send response followed by retrying the same ID after reload, a visible storage error and successful retry, two concurrent tabs with both explicit conflict choices, resumed daily chat deletion after a lost response, confirmed local discard, logout/login, and separate drafts for two accounts. Regular drafts and conflict notices were checked at 320 pixels wide and photo drafts at 390 pixels wide; input/send controls remain visible. Existing photo and entry editing browser tests continue to pass. `npm run check`, the final type check, web export, and iOS JavaScript/Hermes export passed. No unexpected console errors; only expected network messages from simulated failures. Temporary test accounts and Storage files were removed; no OpenAI calls were made in these checks. A physical iPhone test and fully offline startup remain pending.
