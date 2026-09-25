-- Uploads survive interrupted requests and account/message deletion until the
-- Go cleanup worker has removed their private Storage object.
alter table fitty.messages drop constraint messages_content_check;
alter table fitty.messages add constraint messages_content_check
  check (char_length(btrim(content)) between 0 and 8000);
alter table fitty.messages add unique (id, user_id);

create table fitty.attachments (
  id uuid primary key,
  user_id uuid references auth.users(id) on delete set null,
  local_date date not null,
  message_id bigint,
  position smallint check (position between 0 and 3),
  object_path text not null unique,
  sha256 text not null check (length(sha256) = 64),
  mime_type text not null default 'image/jpeg' check (mime_type = 'image/jpeg'),
  byte_size integer not null check (byte_size between 1 and 8388608),
  width integer not null check (width between 1 and 4096),
  height integer not null check (height between 1 and 4096),
  state text not null default 'pending' check (state in ('pending', 'ready')),
  created_at timestamptz not null default now(),
  foreign key (message_id, user_id) references fitty.messages(id, user_id) on delete set null (message_id),
  unique (message_id, position)
);
create index attachments_message on fitty.attachments(user_id, message_id, position);
create index attachments_cleanup on fitty.attachments(created_at) where message_id is null;
revoke all on fitty.attachments from public, anon, authenticated;
grant select, insert, update, delete on fitty.attachments to fitty_api;

update storage.buckets set file_size_limit=8388608, allowed_mime_types=array['image/jpeg']
where id='chat-attachments';
