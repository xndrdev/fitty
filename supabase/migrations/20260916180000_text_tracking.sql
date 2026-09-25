alter table fitty.messages add constraint messages_owner_key unique(id, day_id, user_id);
alter table fitty.messages add column reply_to bigint;
alter table fitty.messages add constraint messages_reply_owner foreign key(reply_to, day_id, user_id)
  references fitty.messages(id, day_id, user_id) on delete cascade;
create unique index one_reply_per_message on fitty.messages(reply_to) where reply_to is not null;

create table fitty.analysis_jobs (
  message_id bigint primary key,
  day_id bigint not null,
  user_id uuid not null,
  status text not null check (status in ('queued','processing','completed','failed','disabled','skipped')),
  attempts integer not null default 0 check (attempts between 0 and 3),
  lease uuid,
  started_at timestamptz,
  finished_at timestamptz,
  error_code text not null default '',
  model text not null default '',
  created_at timestamptz not null default now(),
  foreign key(message_id, day_id, user_id) references fitty.messages(id, day_id, user_id) on delete cascade
);
create index analysis_queue on fitty.analysis_jobs(status, message_id);
create index analysis_day_order on fitty.analysis_jobs(day_id, message_id, status);

create table fitty.tracking_entries (
  id bigint generated always as identity primary key,
  user_id uuid not null,
  day_id bigint not null,
  source_message_id bigint not null,
  updated_by_message_id bigint not null,
  kind text not null check (kind in ('food','activity')),
  label text not null check (char_length(label) between 1 and 160),
  amount text not null check (char_length(amount) <= 160),
  calories numeric(12,2) check (calories between 0 and 20000),
  protein_g numeric(12,2) check (protein_g between 0 and 2000),
  carbs_g numeric(12,2) check (carbs_g between 0 and 2000),
  fat_g numeric(12,2) check (fat_g between 0 and 2000),
  duration_minutes numeric(12,2) check (duration_minutes between 0 and 1440),
  distance_km numeric(12,2) check (distance_km between 0 and 500),
  source text not null check (source in ('estimate','user','device')),
  notes text not null check (char_length(notes) <= 1000),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  deleted_at timestamptz,
  foreign key(day_id, user_id) references fitty.days(id, user_id) on delete cascade,
  foreign key(source_message_id, day_id, user_id) references fitty.messages(id, day_id, user_id) on delete cascade,
  foreign key(updated_by_message_id, day_id, user_id) references fitty.messages(id, day_id, user_id) on delete cascade,
  unique(id, day_id, user_id),
  check ((kind='food' and calories is not null and protein_g is not null and carbs_g is not null and fat_g is not null and duration_minutes is null and distance_km is null)
    or (kind='activity' and protein_g is null and carbs_g is null and fat_g is null and (duration_minutes is not null or distance_km is not null)))
);
create index tracking_day on fitty.tracking_entries(user_id, day_id) where deleted_at is null;

create table fitty.entry_changes (
  message_id bigint not null references fitty.analysis_jobs(message_id) on delete cascade,
  action_index integer not null,
  entry_id bigint not null references fitty.tracking_entries(id) on delete cascade,
  operation text not null check (operation in ('create','update','delete')),
  before_value jsonb,
  after_value jsonb,
  evidence text not null,
  created_at timestamptz not null default now(),
  primary key(message_id, action_index)
);
revoke all on fitty.analysis_jobs, fitty.tracking_entries, fitty.entry_changes from public, anon, authenticated;
grant select, insert, update on fitty.analysis_jobs, fitty.tracking_entries to fitty_api;
grant select, insert on fitty.entry_changes to fitty_api;
grant usage, select on all sequences in schema fitty to fitty_api;
