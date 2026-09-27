# Implementation plan

As of September 17, 2026, milestones 1 through 4 are implemented: login, profiles, daily chats, text tracking, and private photo attachments with image analysis. Local verification includes controlled responses, real model calls, and browser testing at desktop and mobile widths. Camera/HEIC testing on a physical iPhone, further everyday images, and milestones 5 and 6 remain pending. Detailed verification status is documented in the [development guide](DEVELOPMENT.md).

## Goal

Fitty brings the existing personal workflow from the ChatGPT project “Fitness” into a dedicated application: one chat per day for food, exercise, and walking pad sessions, supplemented by photos, screenshots, and advice for that day. Interaction remains free-form and conversational. Structured data provides traceable daily totals and access to past tracking records.

The technical division of responsibilities is described in the [architecture document](ARCHITECTURE.md).

## Agreed requirements

- A private project named Fitty.
- A native iOS app and web interface with login and shared data.
- A daily chat as the central interface; past days remain accessible.
- Free text input and pasting copied content.
- Upload, recognize, and analyze photos and screenshots for tracking.
- Record food with calories and nutritional values; document exercise and walking pad sessions.
- Ask questions about meal planning, restaurant visits, and the current day in the same chat.
- Store data on a privately operated server.
- A Go backend and self-hosted Supabase through the existing Coolify installation.
- The OpenAI API for conversational responses and image understanding.

## Working assumptions for the first version

These decisions keep the initial scope small and can be changed before their respective implementation:

- One personal user account, with no public registration. Data is still associated with a user.
- React Native with Expo, TypeScript, and Expo Router provides the shared foundation for iOS and the web.
- Days are assigned according to the profile time zone, initially `Europe/Berlin`.
- Tracking entries can be created directly from unambiguous statements and corrected afterward. Confirmation is not required for every meal.
- Significant ambiguities are clarified before recording entries. Quantity estimates alone may be saved if marked as estimates.
- Consumed calories and reported activity expenditure are displayed separately. Automatically offsetting them against a daily target is not initially planned.
- AI responses require an internet connection. Full offline synchronization is outside the first version's scope.

## User workflows

### Daily chat

After login, today's chat opens. A compact summary at the top shows energy, protein, carbohydrates, fat, and activities. Messages, images, and tracking entries appear below. Input supports text, camera capture, and image selection; browsers also support file uploads and pasted images where available.

Changing the date opens a separate daily chat. When reading a past day, a new message must not be silently assigned to today. Retrospective statements such as “That was yesterday” must have a visible target date.

### Record food through text or photos

Examples:

- “Breakfast: 250 g of skyr and a banana.”
- A photo of a plate with “My lunch.”
- A photo of packaging and its nutrition label with “I ate 150 g of this.”

Fitty recognizes food and uses any available quantity and packaging information. Estimated ingredients and portions remain visibly marked as estimates. If an image is unclear, Fitty asks a follow-up question instead of creating a supposedly exact entry.

Several photos may describe the same meal, such as a plate and its packaging. They must not automatically count as multiple meals.

### Record activities

Free text and screenshots can provide duration, distance, speed, and displayed energy expenditure. Fitty distinguishes readings from devices from its own estimates. Missing values are not presented as measurements.

### Advice in the day's context

“I'm going to an Italian restaurant tonight” or a photo of a menu produces a response that considers the profile, personal targets effective on that day, and current daily totals. Planned or merely suggested meals do not count toward consumed energy.

### Personal daily targets

Calories, protein, carbohydrates, and fat can be saved individually as optional profile targets. Empty fields mean no target; no recommendations are calculated automatically. Changes take effect from today in the saved profile time zone. Past days continue to show their historical targets. The daily card presents progress, the remaining amount, or an excess neutrally; exercise does not automatically increase the food target.

### Progress photos

Regularly take personal progress photos or select existing images and save them privately with a capture date and viewing angle. The separate photo history supports selecting two photos for a before/after comparison, filtering by viewing angle, and enlarging complete images. Photos are independent of daily chats and are not sent to the AI. Individual photos can be removed after confirmation. The first implementation includes neither automatic reminders nor body analysis.

### Corrections and history

“It was only 150 g,” “That was tofu rather than chicken,” or removing an incorrect entry updates the existing record and daily totals. Earlier messages remain available for context. A day list or calendar opens older chats, including their images and entries.

## Scope of the first usable version

- Login and persistent sessions on iOS and the web.
- Profiles with a time zone, personal targets, and dietary preferences.
- Daily chat with text and multiple image attachments per message.
- Analysis of meal, packaging, workout, and walking pad images.
- Structured, editable food and activity entries.
- A daily overview calculated from stored entries.
- Advice with daily context and a distinction between plans and actual records.
- History and access to images from past days.
- Clear upload, analysis, follow-up question, and error states; resending without duplicate entries.
- Private access, backups, and verified restoration.

## Future extensions

These are possible extensions, not prerequisites for the first version:

- Apple Health, automatic device integration, and barcode scanning.
- Voice input, reminders, and push notifications.
- Detailed weekly statistics, recipes, and recurring meals.
- Per-user data export from the interface.
- Automated import of old ChatGPT chats.
- Multiple users, public registration, and full offline synchronization.

## Milestones

### 1. Project foundation and local development

- [x] Create the planned directory structure for the app, Go server, and documentation.
- [x] Verify current, mutually compatible versions and record them in the project.
- [x] Set up local Supabase, configuration, and SQL migrations.
- [x] Create configuration templates without credentials and a startup guide.
- [x] Create a Go server with a health check and an initial Expo view for the web and iOS.

Acceptance: Both interfaces and the backend can be started using the README. An empty database can be built entirely from versioned migrations.

Verification limit: The iOS JavaScript bundle has been generated. A native Xcode build and launch on a physical iPhone have not yet been tested in this Linux environment.

### 2. Login, profiles, and persistent daily chat

- [x] Set up Supabase Auth for the personal account and disable public registration.
- [x] Implement session management on iOS and the web, plus token validation and user identification in Go.
- [x] Save and load profiles, daily chats, and messages.
- [x] Implement date navigation, time zones, and reopening old chats.
- [x] Create the basic mobile and desktop layout, including keyboard and focus behavior.

Acceptance: A message survives reloading and signing in again and appears on the other device. Date changes and retrospective entries do not modify unrelated daily chats. Unauthenticated access is rejected.

Verification limit: Native session management and the iOS bundle are implemented; testing on a physical iPhone remains pending. The browser and API are tested locally against Supabase.

### 3. Text tracking and advice

- [x] Integrate OpenAI through the Go server; manage API keys exclusively on the server.
- [x] Define structured analysis for recording entries, corrections, advice, and follow-up questions.
- [x] Store food and activity entries, including their source and estimation status.
- [x] Validate AI write proposals and calculate daily totals in the backend.
- [x] Handle repeated requests and corrections without duplicate entries.
- [x] Assemble profiles, daily totals, and relevant chat context for responses.

Acceptance: Breakfast is recorded, a later quantity change corrects the same entry, and a restaurant question creates no meal. Resubmitting a job does not increase daily totals again.

Local acceptance passed on September 16, 2026: `gpt-5-mini` records 250 g of skyr using supplied packaging values and corrects the same entry to 150 g. A restaurant question changes neither entries nor daily totals. A walking pad session remains separately recorded with duration and distance; unknown calories remain unspecified. Resending creates no duplicate. Real responses and daily totals survive reloading in desktop and mobile browsers. This verifies the workflow with synthetic examples; portion estimate quality and further everyday scenarios require separate assessment.

### 4. Photos and screenshots

- [x] Implement camera capture, image selection, and web uploads, including multiple attachments.
- [x] Validate file type, size, and user ownership; convert iPhone images into supported analysis formats (native device verification pending).
- [x] Store appropriately sized images and previews in private Storage buckets.
- [x] Combine image analysis with text and daily context; distinguish portions, labels, and device readings.
- [x] Implement persistent processing states, retries after errors, and cleanup of abandoned uploads.
- [x] Display images, analysis results, and tracking entries together in the chat.

Acceptance: A plate photo produces a visibly marked estimate; a readable nutrition label and quantity produce a traceable calculation. A workout screenshot supplies readable values. A menu remains a basis for advice. Unreadable images lead to a follow-up question. Reprocessing creates no duplicates.

Local verification passed on September 16, 2026: a public banana photo as a visibly marked estimate, a synthetic skyr label with 94.5 kcal for 150 g, correction of the same entry to 200 g/126 kcal, a walking pad display with 30 minutes/2 km/120 device-reported kcal, a menu without an entry, and an unreadable image with a follow-up question. Repeated messages create no duplicates. Browser verification covers multiple selection, clipboard pasting, removal, daily drafts, previews, enlargement, and reloading. Complex meal photos, poor-quality real labels, and the native camera/HEIC workflow still need everyday testing.

### 5. Everyday usability on iOS and the web

As of September 17, 2026: the daily view adopts the colors, fonts, sidebar, and chat design from `ui-preview/Fitty Tagesansicht.dc.html`. Navigation and the daily card are condensed for small screens. Entries can be edited directly, removed from daily totals after confirmation, or corrected through a prefilled chat draft. Version checks and request IDs prevent unnoticed overwrites and duplicate changes. Displayed values come from the backend. Target bars appear only for explicitly configured personal daily targets.

- [x] Complete the daily overview, history, and direct editing or removal of entries.
- [x] Persist daily drafts with text/photos and pending send/delete requests locally; handle storage errors and concurrent tabs.
- [x] Store optional calorie/macronutrient targets with effective dates, show daily progress, and include the applicable targets in the AI context.
- [x] Add a private progress photo history with capture dates, viewing angles, comparison of two photos, and confirmed removal.
- [x] Store personal favorite meal portions and offer matching suggestions while typing, plus a favorites picker.
- [x] Condense the mobile header into a single row with navigation and daily actions in a hamburger menu.
- [ ] Complete the remaining loading/error states and restore direct entry dialogs after a restart.
- [ ] Test the same examples on a physical iPhone and in a desktop browser.
- [ ] Check keyboard usability, screen reader labels, contrast, and reduced motion.
- [x] Support deleting chats together with their entries and images.

Acceptance: A complete example day can be recorded, reviewed, and corrected on both platforms without data loss. Deleted content disappears from the active dataset and daily totals. The backup retention period is documented.

Partial browser acceptance on September 17, 2026: direct food/activity corrections, empty values versus zero, a stale dialog after a change in another session, retries after a lost response, and confirmed entry deletion passed against Go/PostgreSQL/Storage. Earlier messages and photos remain available. Separate full chat deletion is also implemented: confirmation with a current preview, protection against stale deletion requests, and persistent photo cleanup after Storage failures. Additional browser acceptance of full deletion with real private photos, two tabs, a lost response, and safe creation of a new chat passed at desktop and mobile widths. Persistent drafts were then verified in the browser with photos, reloading, identical send/delete retries, simulated storage failure, and account switching. Physical iPhone testing and a backup retention policy remain pending.

### 6. Coolify hosting and private deployment

- [ ] Deploy Supabase, the Go service, and the web interface with HTTPS, persistent storage, and separate secrets.
- [ ] Configure internal connectivity between Go and PostgreSQL, along with private Storage access.
- [ ] If email password resets are required: configure SMTP and allowed redirect URLs.
- [ ] Set up backups for the database, image files, and required operational configuration.
- [ ] Restore an example day and image attachment in a separate environment.
- [ ] Set up the chosen iOS distribution method and test installation on the user's own device.
- [ ] Document operations, including updates, migrations, restoration, and API cost limits.

Acceptance: The iOS app and web interface use the same server. Data survives a restart and restoration. Access on the user's own iPhone works through the agreed distribution method.

## Verification strategy

Targeted automated tests cover domain rules: day assignment, calculations from stored entries, corrections, user boundaries, and protection against duplicate entries. Controlled example outputs are used for AI responses to keep these tests reproducible.

A small set of representative texts and images is also tested with the real OpenAI integration: a meal, packaging, a poorly readable label, a walking pad screenshot, and a menu. Evaluation covers correct assignment, traceable sources, and handling of uncertainty. A food photo is not evidence of an exactly measurable calorie count.

Device tests cover image selection, uploads, keyboard behavior, session expiry, and connection interruptions. A restore test explicitly includes image files.

## Open decisions

| Decision | Timing | Suggested starting point |
| --- | --- | --- |
| OpenAI model and output limit | Live acceptance for milestone 3 | Provisionally `gpt-5-mini`, 8,192 output tokens; assess quality, latency, and costs with personal examples, configurable through an environment variable |
| Importing existing project instructions | When content becomes available | Personal goals and preferences can be saved; selectively incorporate instructions from “Fitness” once provided |
| Native iOS distribution | Before milestone 5 | Clarify a suitable private distribution method and Apple prerequisites; treat Expo Go as an early development tool only |
| Domains, SMTP, and storage destination | Before milestone 6 | Use the existing Coolify infrastructure; manage credentials outside the repository |
| Backup destination and retention | Before milestone 6 | A restorable copy outside the application server; choose the retention period explicitly |
| Old fitness chats | After the first version | Plan import separately using a provided export or selected chats |

The ChatGPT project “Fitness” is not directly accessible from this working environment. Importing its content and instructions requires those materials to be provided. API usage is billed separately from the ChatGPT subscription.
