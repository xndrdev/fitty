alter table fitty.tracking_entries add column version bigint not null default 1 check (version > 0);
-- NULL distinguishes a direct edit from an AI change attributed to a message.
alter table fitty.tracking_entries alter column updated_by_message_id drop not null;

create table fitty.manual_entry_changes (
  user_id uuid not null,
  request_id uuid not null,
  day_id bigint not null,
  entry_id bigint not null,
  expected_version bigint not null check (expected_version > 0),
  operation text not null check (operation in ('update','delete')),
  before_value jsonb not null,
  after_value jsonb,
  created_at timestamptz not null default now(),
  primary key (user_id, request_id),
  foreign key (entry_id, day_id, user_id) references fitty.tracking_entries(id, day_id, user_id) on delete cascade,
  check ((operation = 'update' and after_value is not null) or (operation = 'delete' and after_value is null))
);
create index manual_entry_history on fitty.manual_entry_changes(entry_id, created_at);
revoke all on fitty.manual_entry_changes from public, anon, authenticated;
grant select, insert on fitty.manual_entry_changes to fitty_api;
