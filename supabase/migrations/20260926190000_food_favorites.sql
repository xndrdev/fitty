-- Favorites keep the selected portion and provenance after its original day is deleted.
create table fitty.food_favorites (
  user_id uuid not null references auth.users(id) on delete cascade,
  entry_id bigint not null check (entry_id > 0),
  entry jsonb not null check (jsonb_typeof(entry) = 'object' and entry->>'kind' = 'food'),
  created_at timestamptz not null default now(),
  primary key (user_id, entry_id)
);
create index food_favorites_recent on fitty.food_favorites(user_id, created_at desc, entry_id desc);
revoke all on fitty.food_favorites from public, anon, authenticated;
grant select, insert, delete on fitty.food_favorites to fitty_api;
