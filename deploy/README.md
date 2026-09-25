# Bereitstellung

Aktuell sind Anmeldung, Profil und persistente Tageschats lokal eingerichtet. Die Bereitstellung über Coolify folgt in Meilenstein 6 des [Plans](../docs/PLAN.md).

Vorgesehene Dienste sind das Go-Backend, der Expo-Webexport und selbst gehostetes Supabase mit Auth, PostgreSQL und Storage. Produktionswerte und Zugangsdaten werden in Coolify gepflegt.

`supabase/config.toml` konfiguriert ausschließlich die lokale Supabase CLI. Diese Datei konfiguriert keinen Coolify-Supabase-Stack. Die versionierten SQL-Migrationen gelten für beide Umgebungen; vor dem ersten Deployment werden PostgreSQL-Version und Storage-Schema auf Kompatibilität geprüft.

Die lokale Umgebung enthält Entwicklungszugänge und ersetzt keine Produktionskonfiguration. Login, Benutzerzuordnung und das eingeschränkte Datenbankkonto sind umgesetzt. Vor einer Veröffentlichung folgen Produktionskonten, HTTPS, die Konfiguration der Produktionszugriffe sowie Backups einschließlich der Bilddateien.
