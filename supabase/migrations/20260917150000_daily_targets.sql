alter table fitty.profiles add column targets_version bigint not null default 0 check (targets_version >= 0);

-- Effective dates preserve the goal used for earlier calendar days.
create table fitty.daily_targets (
  user_id uuid not null references auth.users(id) on delete cascade,
  effective_from date not null,
  calories numeric(7,2) check (calories > 0 and calories <= 20000),
  protein_g numeric(6,2) check (protein_g > 0 and protein_g <= 2000),
  carbs_g numeric(6,2) check (carbs_g > 0 and carbs_g <= 2000),
  fat_g numeric(6,2) check (fat_g > 0 and fat_g <= 2000),
  primary key (user_id, effective_from)
);
create table fitty.daily_target_changes (
  user_id uuid not null references auth.users(id) on delete cascade,
  request_id uuid not null,
  expected_version bigint not null check (expected_version >= 0),
  effective_from date not null,
  targets jsonb not null,
  created_at timestamptz not null default now(),
  primary key (user_id, request_id)
);
revoke all on fitty.daily_targets, fitty.daily_target_changes from public, anon, authenticated;
grant select, insert, update on fitty.daily_targets to fitty_api;
grant select, insert on fitty.daily_target_changes to fitty_api;
