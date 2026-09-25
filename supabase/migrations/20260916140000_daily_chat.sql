create table fitty.profiles (
  user_id uuid primary key references auth.users(id) on delete cascade,
  display_name text not null default '' check (char_length(display_name) <= 80),
  time_zone text not null default 'Europe/Berlin',
  goals text not null default '' check (char_length(goals) <= 2000),
  preferences text not null default '' check (char_length(preferences) <= 2000),
  updated_at timestamptz not null default now()
);

create table fitty.days (
  id bigint generated always as identity primary key,
  user_id uuid not null references auth.users(id) on delete cascade,
  local_date date not null,
  time_zone text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, local_date),
  unique (id, user_id)
);

create table fitty.messages (
  id bigint generated always as identity primary key,
  day_id bigint not null,
  user_id uuid not null,
  client_id uuid not null,
  role text not null default 'user' check (role in ('user', 'assistant')),
  content text not null check (char_length(btrim(content)) between 1 and 8000),
  created_at timestamptz not null default now(),
  foreign key (day_id, user_id) references fitty.days(id, user_id) on delete cascade,
  unique (user_id, client_id)
);
create index messages_day_order on fitty.messages(user_id, day_id, id desc);

-- Only the Go server accesses these tables. The schema is not exposed in
-- PostgREST; browser and native clients use the authenticated Go API.
revoke all on all tables in schema fitty from public, anon, authenticated;
do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'fitty_api') then
    create role fitty_api login nosuperuser nocreatedb nocreaterole noinherit nobypassrls;
  end if;
end $$;
grant usage on schema fitty to fitty_api;
grant select, insert, update on fitty.profiles, fitty.days to fitty_api;
grant select, insert on fitty.messages to fitty_api;
grant usage, select on all sequences in schema fitty to fitty_api;
