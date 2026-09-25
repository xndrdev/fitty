-- Progress photos have their own lifecycle and never enter chat analysis.
create table fitty.progress_photos (
  id uuid primary key,
  user_id uuid references auth.users(id) on delete set null,
  taken_on date not null,
  view text not null check (view in ('front', 'side', 'back', 'other')),
  object_path text not null unique,
  sha256 text not null check (length(sha256) = 64),
  mime_type text not null default 'image/jpeg' check (mime_type = 'image/jpeg'),
  byte_size integer not null check (byte_size between 1 and 8388608),
  width integer not null check (width between 1 and 4096),
  height integer not null check (height between 1 and 4096),
  state text not null default 'pending' check (state in ('pending', 'ready')),
  created_at timestamptz not null default now(),
  deleted_at timestamptz
);
create index progress_photos_timeline on fitty.progress_photos(user_id, taken_on desc, id desc)
  where state = 'ready' and deleted_at is null;
create index progress_photos_cleanup on fitty.progress_photos(created_at)
  where state = 'pending' or deleted_at is not null or user_id is null;

-- Keep only identities after deletion so a delayed upload cannot restore a photo.
create table fitty.progress_photo_tombstones (
  user_id uuid not null references auth.users(id) on delete cascade,
  id uuid not null,
  primary key (user_id, id)
);
revoke all on fitty.progress_photos, fitty.progress_photo_tombstones from public, anon, authenticated;
grant select, insert, update, delete on fitty.progress_photos to fitty_api;
grant select, insert on fitty.progress_photo_tombstones to fitty_api;
