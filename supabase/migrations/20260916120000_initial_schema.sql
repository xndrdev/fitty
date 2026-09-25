-- Fachliche Tabellen werden später hier angelegt und ausschließlich von Go genutzt.
create schema if not exists fitty;

revoke all on schema fitty from public, anon, authenticated;
alter default privileges for role postgres in schema fitty
    revoke execute on functions from public;

comment on schema fitty is 'Private Anwendungsdaten für das Fitty-Go-Backend.';

-- Keine Client-Policies: Zugriff erfolgt später über das berechtigungsprüfende Backend.
insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values (
    'chat-attachments',
    'chat-attachments',
    false,
    20971520,
    array['image/jpeg', 'image/png', 'image/webp']
);
