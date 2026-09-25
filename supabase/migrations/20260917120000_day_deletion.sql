alter table fitty.days add column version bigint not null default 1 check (version > 0);
grant delete on fitty.days to fitty_api;

-- Receipts deliberately survive the day, but contain no chat or tracking data.
create table fitty.day_deletions (
  user_id uuid not null references auth.users(id) on delete cascade,
  request_id uuid not null,
  day_id bigint not null,
  local_date date not null,
  expected_version bigint not null check (expected_version > 0),
  created_at timestamptz not null default now(),
  primary key (user_id, request_id)
);
-- Prevent delayed message/upload retries from restoring deleted content.
create table fitty.deleted_identifiers (
  user_id uuid not null references auth.users(id) on delete cascade,
  kind text not null check (kind in ('message', 'attachment')),
  id uuid not null,
  primary key (user_id, kind, id)
);
alter table fitty.attachments add column delete_requested_at timestamptz;
alter table fitty.attachments add column deletion_request_id uuid;
create index attachments_pending_deletion on fitty.attachments(delete_requested_at)
  where delete_requested_at is not null;
create index attachments_day on fitty.attachments(user_id, local_date);
revoke all on fitty.day_deletions, fitty.deleted_identifiers from public, anon, authenticated;
grant select, insert on fitty.day_deletions, fitty.deleted_identifiers to fitty_api;
