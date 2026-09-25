# Lokale Entwicklung

Stand: 17. September 2026. Verfügbar sind Anmeldung, Profil, persönliche Tagesziele, Tageschats, KI-Textverarbeitung, private Fotoanhänge mit Bildanalyse sowie ein eigener Fortschrittsfoto-Vergleich. Coolify-Betrieb und Tests auf einem echten iPhone folgen. Der konkrete Prüfstand steht unten.

## Versionen

| Werkzeug | Stand |
| --- | --- |
| Node.js / npm | 24.21.0 LTS / 11.12.1 |
| Expo | SDK 57, Paket 57.0.23 |
| React / React Native | 19.2.3 / 0.86.3 |
| TypeScript | 6.0.x |
| Go / pgx | 1.27 / 5.11.0 |
| Supabase CLI / JavaScript-Client | 2.117.0 / 2.116.0 |
| PostgreSQL lokal | Major 17 |

`react-native-reanimated` und `react-native-worklets` sind im Root-Paket per Override auf die von Expo erwarteten Versionen festgelegt. Bei Expo-Upgrades gemeinsam aktualisieren. Die Sitzungsverwaltung verwendet die Koordination der aktuellen Supabase-Bibliothek ohne die inzwischen veraltete eigene `lock`-Option.

## Installation und Start

Node-Version aus `.node-version` aktivieren, dann:

```bash
npm ci
npm run db:start
npm run db:migrate
npm run setup:local
```

Auf diesem Rechner beim Start `npm run db:start -- --network-id fitty-local` verwenden; siehe Abschnitt zu Docker weiter unten.

`setup:local` erstellt ein privates Konto `fitty@example.test` mit zufälligem Passwort, setzt ein eigenes Passwort für die eingeschränkte Datenbankrolle und schreibt:

- `.local/zugang.txt`: Anmeldedaten für die Vorschau.
- `.local/account.json`: Zugangsdaten zur Wiederverwendung beim nächsten lokalen Setup.
- `server/.env`: Go-Konfiguration einschließlich Datenbankpasswort.
- `apps/client/.env`: öffentliche URLs und öffentlicher Supabase-Schlüssel.

Diese Dateien sind von Git ausgeschlossen und werden mit Dateimodus 0600 angelegt. Das Setup ist auf die lokale Instanz begrenzt, verwendet für die Kontoanlage kurz den lokalen Admin-Schlüssel und speichert diesen nicht in der App. Erneutes Ausführen schreibt die lokale Konfiguration erneut; eigene LAN-Anpassungen anschließend wieder vornehmen. Vorhandene Zeilen für `OPENAI_API_KEY` und `FITTY_OPENAI_MODEL` bleiben erhalten. Keine Produktionskonfiguration damit verwalten.

Danach in getrennten Terminals:

```bash
npm run dev:api
```

```bash
npm run dev:web
```

Oberfläche: **http://localhost:8788**. Mit den Daten aus `.local/zugang.txt` anmelden. Nachrichten erscheinen nach erfolgreicher Speicherung mit Zeitstempel; ein neuer Tag entsteht beim ersten Senden. „Heute“ richtet sich nach der Profilzeitzone. Ein bereits geöffneter Tag wechselt bei Mitternacht nicht während der Eingabe automatisch.

| Befehl | Zweck |
| --- | --- |
| `npm run dev` | Expo-Entwicklungsserver für iPhone und Web |
| `npm run db:stop` | Supabase stoppen, Daten behalten |
| `npm run db:status` | Lokale URLs und Schlüssel anzeigen; Ausgabe vertraulich behandeln |
| `npm run db:migrate` | Neue SQL-Migrationen anwenden |
| `npm run db:reset` | Lokale Daten löschen und Schema neu aufbauen |
| `npm run setup:local` | Lokalen Zugang und Konfiguration einrichten |
| `npm run test:local` | Auth/Chat gegen Go und Supabase prüfen, ausdrücklich ohne KI-Aufträge |
| `npm run test:tracking` | Tracking-Worker mit kontrollierten Antworten gegen PostgreSQL prüfen |
| `npm run test:targets` | Optionale Zielwerte, Dezimaleingaben und Fortschrittsberechnung prüfen |
| `npm run test:progress-photos` | Kalenderdaten und Zusammenführen paginierter Fotoverläufe prüfen |

Nach einem gewollten `db:reset` das Setup erneut ausführen, um das lokale Konto wieder anzulegen. Den Reset nicht für normale Updates verwenden.

## Konfiguration

`npm run dev:api` lädt `server/.env` über einen Node-Starthelfer. Bereits gesetzte Prozessvariablen haben Vorrang. Ein gebautes Go-Binary liest ausschließlich seine Prozessumgebung. Expo lädt `apps/client/.env`; nach Änderungen den Entwicklungsserver neu starten.

| Variable | Verwendung |
| --- | --- |
| `FITTY_ADDR` | Bind-Adresse, Standard `127.0.0.1:8787` |
| `FITTY_ALLOWED_ORIGINS` | Durch Kommas getrennte Browser-Origins; lokal `http://localhost:8788,http://127.0.0.1:8788` |
| `FITTY_DATABASE_URL` | PostgreSQL-Verbindung für `fitty_api`; Pflichtwert und geheim |
| `FITTY_SUPABASE_URL` | Supabase Auth und Storage aus Sicht des Go-Servers |
| `FITTY_SUPABASE_KEY` | Öffentlicher/Anon-Schlüssel für die Auth-Prüfung |
| `FITTY_SUPABASE_SERVICE_ROLE_KEY` | Geheim, nur im Go-Server für den privaten Bilder-Bucket; ohne Wert sind Foto-Uploads deaktiviert |
| `OPENAI_API_KEY` | Geheim, ausschließlich serverseitig; ohne Wert kein KI-Worker |
| `FITTY_OPENAI_MODEL` | Modell für Responses API, Standard `gpt-5-mini` |
| `EXPO_PUBLIC_API_URL` | Go-API aus Sicht des Clients |
| `EXPO_PUBLIC_SUPABASE_URL` | Supabase Auth und signierte Bildabrufe aus Sicht des Clients |
| `EXPO_PUBLIC_SUPABASE_KEY` | Öffentlicher/Anon-Schlüssel, niemals Service-Role- oder OpenAI-Key |

`/healthz` meldet, ob der Go-Prozess läuft und die KI konfiguriert ist. Beim Start prüft Go die Datenbankverbindung; ein späterer erfolgreicher Healthcheck ist kein Datenbank-, Auth- oder Storage-Readiness-Test.

## Anmeldung und Datenhaltung

Die globale Einstellung `auth.enable_signup = false` deaktiviert die öffentliche Registrierung. `auth.email.enable_signup = true` ist in dieser CLI-Version erforderlich, damit der E-Mail-/Passwort-Provider zum Anmelden verfügbar bleibt. Die Integrationstests prüfen sowohl deaktivierte Registrierung als auch aktivierten E-Mail-Login.

Go validiert jedes Access Token über `GET /auth/v1/user` beim konfigurierten Supabase-Auth-Dienst. Es übernimmt keine ungeprüften JWT-Claims. Ungültige Sitzungen ergeben HTTP 401, eine nicht erreichbare Auth-Instanz HTTP 503. Alle fachlichen Abfragen sind an die bestätigte Benutzer-ID gebunden. Sitzungen werden im Browser durch die Supabase-Bibliothek und auf iOS über AsyncStorage erhalten; Abmelden beendet die lokale Sitzung.

Migrationen legen das nicht über PostgREST exponierte Schema `fitty` mit `profiles`, `days` und `messages` an. Die Rolle `fitty_api` besitzt nur die benötigten Lese-/Schreibrechte; Clients und die Rollen `anon`/`authenticated` haben keinen direkten Zugriff. Ein zusammengesetzter Fremdschlüssel bindet jede Nachricht an Tag und Benutzer. Pro Benutzer/Datum ist nur ein Tag zulässig.

| Endpunkt | Funktion |
| --- | --- |
| `GET /v1/profile` | Eigenes Profil oder Standardwerte laden |
| `PUT /v1/profile` | Name, Zeitzone, Freitextziele und Vorlieben speichern |
| `GET /v1/targets` | Numerische Ziele für heute, heutiges Datum, Gültigkeitsbeginn und Version laden |
| `PUT /v1/targets` | Numerische Ziele ab heute mit Versionsprüfung speichern |
| `GET /v1/progress-photos` | Eigenen Fortschrittsfoto-Verlauf laden, optional nach Perspektive filtern |
| `PUT /v1/progress-photos/{id}?date=YYYY-MM-DD&view=front` | Fortschrittsfoto mit Datum/Perspektive hochladen; mit derselben UUID und denselben Daten wiederholbar |
| `GET /v1/progress-photos/{id}/url` | Privaten Bildpfad für ein eigenes Fortschrittsfoto ausstellen |
| `DELETE /v1/progress-photos/{id}` | Eigenes Fortschrittsfoto entfernen; wiederholbar |
| `GET /v1/days?before=YYYY-MM-DD` | Bis zu 30 eigene Tage, absteigend |
| `GET /v1/days/{date}/messages?before={id}` | Bis zu 100 Nachrichten, chronologisch dargestellt |
| `POST /v1/days/{date}/messages` | Nachricht speichern und bei verfügbarer KI automatisch auswerten; optional `analyze:false` nur speichern |
| `GET /v1/days/{date}/summary` | Einträge, Tageswerte, gültige Ziele, Verarbeitungsstatus und KI-Verfügbarkeit |
| `PUT /v1/days/{date}/entries/{id}` | Eigenen Eintrag mit Versionsprüfung direkt bearbeiten |
| `DELETE /v1/days/{date}/entries/{id}` | Eigenen Eintrag mit Versionsprüfung aus der Tagesübersicht entfernen |
| `POST /v1/messages/{id}/analysis/retry` | Eigene Nachricht nachholen oder nach Fehler erneut auswerten |
| `POST /v1/messages/{id}/analysis/skip` | Fehlgeschlagene Auswertung bewusst überspringen |
| `PUT /v1/days/{date}/attachments/{id}` | Binäres Bild hochladen; wiederholbar mit gleicher UUID und identischen Bilddaten |
| `GET /v1/attachments/{id}/url` | Nach Besitzerprüfung zehn Minuten gültigen Bildpfad ausstellen |
| `DELETE /v1/attachments/{id}` | Eigenen, noch nicht an eine Nachricht gebundenen Upload entfernen |
| `GET /v1/days/{date}/deletion` | Aktuellen Tagesstand und Löschumfang laden |
| `DELETE /v1/days/{date}` | Bestätigten Tageschat samt Einträgen und Fotos löschen |

Beide Listen liefern bei weiteren Ergebnissen `next_before`. Nachrichten sind auf 8.000 Zeichen begrenzt. Derselbe Auftrag wird über `(user_id, client_id)` auch bei parallelen Wiederholungen nur einmal gespeichert. Andere Inhalte oder ein anderer Tag mit derselben ID ergeben HTTP 409. Die Oberfläche behält eine unbestätigte ID beim erneuten Senden; Eingaben sind bis zur Bestätigung gesperrt. Text, Fotoauswahl und unbestätigte Sendeaufträge werden pro Konto und Datum auf diesem Gerät gespeichert. Ein offener Auftrag bleibt über einen Neustart mit derselben ID wiederholbar. Es gibt keinen automatischen Hintergrundversand und keine geräteübergreifende Entwurfssynchronisation.

`POST /v1/days/{date}/messages` akzeptiert zusätzlich `attachment_ids` mit höchstens vier eigenen fertigen Uploads desselben Tages. Ohne Text ist mindestens ein Bild erforderlich. Beim Wiederholen einer Nachrichten-ID müssen Text, Tag und geordnete Bildauswahl identisch bleiben.

## Persönliche Tagesziele

Migration `20260917150000_daily_targets.sql` ergänzt `profiles.targets_version`, `daily_targets` und `daily_target_changes`. Nach `npm run db:migrate` die Go-API neu starten. Der bestehende Profil-Endpunkt behält seinen Vertrag; dessen Speichervorgänge verändern keine numerischen Ziele.

Im Profil speichert **„Tagesziele speichern“** die vier freiwilligen Ziele unabhängig von **„Profil speichern“**. Dezimalkomma und Dezimalpunkt sind erlaubt, Tausendertrennzeichen nicht. Leere Felder werden als `null` gespeichert; `0` ist kein Zielwert. Werte müssen positiv sein und dürfen höchstens zwei Nachkommastellen haben. Die technischen Obergrenzen sind 20.000 kcal und 2.000 g je Makronährstoff; das sind Validierungsgrenzen und keine Empfehlungen. Kalorien und Makros werden nicht gegenseitig abgeleitet. Die Balken zeigen nur gesetzte Ziele; Überschreitungen bleiben als Text sichtbar, der Balken endet bei 100 Prozent. Aktivitätskalorien verändern das Essensziel nicht.

`GET /v1/targets` liefert `{version,today,effective_from,targets}`. `today` stammt aus der gespeicherten Profilzeitzone; `effective_from` ist ohne bisherigen Zielstand `null`. `targets` enthält immer `calories`, `protein_g`, `carbs_g` und `fat_g`, jeweils als Zahl oder `null`.

`PUT /v1/targets` erwartet `{request_id,version,effective_from,targets}` mit UUID, geladener Version, dem geladenen `today` als Gültigkeitsdatum und allen vier Feldern. Erfolg liefert HTTP 200 mit dem aktuellen GET-Format. Ein ungültiges Werteobjekt ergibt HTTP 400. Eine veraltete Version, ein inzwischen gewechselter Tag oder eine anders belegte Request-ID ergeben HTTP 409. Die Oberfläche behält eigene Eingaben und verlangt das Laden und bewusste Übernehmen des neuen Stands, bevor erneut gespeichert wird.

Änderungen gelten ab heute. Historische Tage laden den jüngsten Zielstand bis zu ihrem Datum; mehrere Änderungen an einem Tag ersetzen diesen Tagesstand. Alle vier Ziele leeren hebt sie ab heute auf, ohne frühere Ziele zu entfernen. `GET /v1/days/{date}/summary` liefert diesen Stand als `targets` und dessen Datum als `targets_effective_from`, auch ohne Nachrichten an diesem Tag. Eine Tageschat-Löschung lässt die Ziele und ihren Verlauf erhalten.

Bei einer unklaren Netzwerkantwort bleibt derselbe Speichervorgang mit derselben UUID und denselben Werten im geöffneten Formular wiederholbar. Bereits bestätigte Wiederholungen ändern nichts mehr und liefern den jetzt gültigen Stand; das gilt auch nach weiteren Änderungen und einem Tageswechsel. Bis zur Klärung sind Felder und Navigation gesperrt. Ungespeicherte Ziele und unbestätigte Ziel-Speichervorgänge liegen nur im Arbeitsspeicher; nach einem App-Neustart wird der Serverstand neu geladen. Die dauerhaften Tageschat-Entwürfe bleiben beim Öffnen des Profils erhalten.

Go lädt den für das Chatdatum gültigen Zielstand als `daily_targets` für die KI. Die Anweisung nutzt ihn zusammen mit `daily_totals` für Beratung, behandelt Ziele aber nicht als gegessene Mengen und erlaubt keine automatischen Zieländerungen. Das Modell bekommt keine abgeleiteten Zielwerte für leere Felder.

## Fortschrittsfotos

Migration `20260917170000_progress_photos.sql` ergänzt `progress_photos` und `progress_photo_tombstones`. Nach `npm run db:migrate` die Go-API neu starten. Es werden der bestehende private Bucket und dessen Serverkonfiguration verwendet. Das Objektverzeichnis `<user>/progress/` trennt Fortschrittsfotos von Chatbildern; direkte Datenbankzugriffe durch den Client bleiben gesperrt.

Im Bereich **„Fortschrittsfotos“** ein Foto aufnehmen oder auswählen. Aufnahmedatum ist zunächst heute in der Profilzeitzone; vergangene Daten ab 1900 sind zulässig. Perspektiven sind `front` (vorne), `side` (seitlich), `back` (hinten) und `other` (sonstige). Erst **„Fortschrittsfoto speichern“** lädt das Bild hoch. Es gelten dieselben Aufbereitungs-/Größengrenzen wie bei Chatbildern: ausgewählte Datei bis 20 MiB, clientseitig JPEG mit maximal 2.048 Pixeln je Seite; Server bis 8 MiB, höchstens 12 Megapixel/4.096 Pixel je Seite, erneute JPEG-Kodierung ohne EXIF-Metadaten.

`PUT` überträgt das binäre Bild und liefert `{photo:{id,date,view,mime_type,byte_size,width,height}}`. UUID, Datum, Perspektive und normalisierte Bilddaten bleiben bei einer Wiederholung identisch; geänderte Angaben oder eine stillgelegte ID ergeben HTTP 409. Ein neuer Upload mit ungültigem Datum, zukünftiger Aufnahme, unbekannter Perspektive oder unlesbarem Bild ergibt HTTP 400, zu große Dateien HTTP 413. Ein bereits reserviertes Foto kann nach einem Zeitzonenwechsel weiterhin bestätigt werden, auch wenn sein damaliges Aufnahmedatum nun als morgen erscheint. Bei Storage-Problemen bleibt die Reservierung erhalten und HTTP 503 erlaubt eine bewusste Wiederholung.

`GET /v1/progress-photos` liefert `{photos:[],next_before,photos_enabled,pending_deletions}` mit höchstens 24 fertigen Aufnahmen, absteigend nach Datum und UUID. `view` filtert optional nach einer der vier Perspektiven. `before` übernimmt den URL-kodierten Cursor aus `next_before`; ohne weitere Aufnahmen ist dieser `null`. Laufende oder entfernte Bilder werden nicht angezeigt. Im Client können zwei verschiedene Aufnahmen als A und B ausgewählt werden, auch aus unterschiedlichen Seiten oder Filtern. Beide zeigen Datum/Perspektive und das vollständige Bild; Antippen öffnet die Vergrößerung. Die Auswahl des Vergleichs selbst wird nicht gespeichert.

`GET /{id}/url` liefert `{path,expires_in:600}`. Neue Links werden nur nach Besitzerprüfung für fertige, nicht entfernte Fotos ausgestellt. Alte bereits ausgegebene Links können bis zum Ablauf oder der physischen Dateilöschung noch funktionieren. Bereits heruntergeladene Kopien und Backups sind von der Löschung nicht betroffen.

Die Bestätigung **„Foto endgültig entfernen“** sendet `DELETE /{id}` ohne Body. HTTP 200 mit `{status:"removed"}` bestätigt das Ausblenden und den dauerhaften Bereinigungsauftrag. Wiederholung derselben eigenen ID ist auch nach physischer Löschung erfolgreich; fremde oder unbekannte IDs liefern HTTP 404. Datum oder Perspektive korrigiert man durch Entfernen und neues Hinzufügen. Die Bereinigung prüft Löschaufträge alle fünf Sekunden; unterbrochene Uploads älter als 24 Stunden und Aufnahmen gelöschter Konten werden beim Start und danach stündlich gesucht. Ein Storage-Ausfall lässt die Aufträge für spätere Versuche bestehen. Fertige Aufnahmen werden nicht aufgrund ihres Alters gelöscht. Stillgelegte UUIDs bleiben bis zur Kontolöschung erhalten.

Ungespeicherte Auswahl und offene Upload-/Löschanfragen werden nur in der geöffneten Ansicht gehalten. Nach einem Neustart den gespeicherten Fotoverlauf prüfen; es erfolgt kein automatischer Upload. Im geöffneten Formular bleibt eine unklare Upload-Antwort mit genau denselben Bildbytes, ID, Datum und Perspektive wiederholbar; die Navigation ist bis zur Bestätigung gesperrt. Ein noch ungesendetes Foto lässt sich nach Bestätigung verwerfen. Bestehende Tageschat-Entwürfe bleiben beim Bereichswechsel erhalten. Fortschrittsfotos gelangen nicht an OpenAI und zählen nicht als Tagestracking.

## Dauerhafte Tagesentwürfe

Text und bis zu vier aufbereitete JPEG-Fotos werden beim Bearbeiten automatisch pro Konto und Datum auf dem jeweiligen Gerät gespeichert. Der Status unterscheidet laufendes Speichern, abgeschlossene Speicherung und Fehler. Nach Neuladen oder erneutem Login stehen die Entwürfe unter „Entwürfe auf diesem Gerät“ im Verlauf bereit. Die App öffnet weiterhin standardmäßig den heutigen Tag; Entwürfe anderer Tage lassen sich aus der Liste wählen.

Im Browser speichert IndexedDB (`fitty-drafts`, Store `drafts`) Metadaten und Bild-Blobs atomar in einem Datensatz. Ein erneutes Öffnen erzeugt neue Vorschau-URLs aus den gespeicherten Bilddaten. Unveränderte Fotos werden während des Tippens aus dem lokalen Blob-Cache wiederverwendet. Auf iOS enthält AsyncStorage die Metadaten; Fotodateien liegen unter `Paths.document/fitty-drafts/<user>/<date>/`. Relative Dateipfade werden beim Laden anhand des aktuellen Dokumentverzeichnisses aufgelöst. Entfernte native Dateien werden erst nach bestätigtem Metadaten-Commit bereinigt. Das Freigeben einer Vorschau löscht ausschließlich temporäre Cachedateien beziehungsweise Blob-URLs.

Jeder Stand besitzt eine zufällige Revision. Speichern ersetzt nur die geladene Revision; auch ein leerer Entwurf bleibt als inhaltsfreie Versionszeile erhalten. Zwei Browser-Tabs können sich dadurch nicht unbemerkt überschreiben. Bei Konflikten bleibt der lokale Text sichtbar. „Gespeicherten Entwurf übernehmen“ lädt den aktuellen Stand; „Meinen Entwurf speichern“ ersetzt ihn nach der ausdrücklich beschrifteten Auswahl. Ein gespeicherter offener Sende-/Löschvorgang muss zuerst übernommen und abgeschlossen werden. Der Konflikthinweis ist auf kleinen Displays innerhalb seiner eigenen Fläche scrollbar.

Vor dem ersten Upload wird der vollständige Sendeauftrag mit Nachrichten-ID, Inhalt und geordneter Bildauswahl gespeichert. Uploads oder eine verlorene Antwort können mit denselben Kennungen wiederholt werden. Es erfolgt kein automatischer Versand beim Öffnen oder Wiederverbinden. Erst nach Serverbestätigung und erfolgreichem lokalen Abschluss wird der Entwurf geleert. Hat ein anderer Tab den Auftrag schon abgeschlossen und einen neuen Entwurf begonnen, bleibt dieser erhalten. Auch eine bestätigte Tageschat-Löschung wird vor der Serveranfrage lokal festgehalten und lässt sich nach Neustart identisch bestätigen.

„Entwurf verwerfen“ verlangt eine separate Bestätigung und entfernt den lokalen Text sowie die Fotoauswahl dieses Tages. Ein bereits offener Sendevorgang bleibt bis zur Klärung gesperrt. Eine bestätigte Tageschat-Löschung entfernt den zugehörigen lokalen Entwurf ebenfalls. Abmelden erhält gespeicherte Entwürfe; ein anderes Konto lädt sie nicht. Noch nicht gespeicherte Änderungen werden vor dem Abmelden gesichert. Ungebundene serverseitige Uploadreste werden nach Möglichkeit entfernt, andernfalls greift die bestehende 24-Stunden-Bereinigung.

**Grenzen:** Entwürfe synchronisieren sich nicht zwischen Geräten. Sie sind lokale App-/Browserdaten und keine separat verschlüsselte Ablage. Browserdaten löschen, Deinstallation oder Verlust des Geräts kann sie entfernen. Bei vollem oder nicht verfügbarem Speicher bleibt der aktuelle Text sichtbar und Fitty bietet erneutes Speichern an; bis zur Bestätigung die App geöffnet lassen. Der Browser warnt beim Verlassen mit noch ungesicherten Änderungen. Ein sofortiger Prozessabbruch kann Änderungen seit dem letzten abgeschlossenen Speichern verlieren. Die Oberfläche kann in einer geöffneten Sitzung ohne API-Verbindung weiter Entwürfe bearbeiten; der vollständige Offline-Start inklusive Sitzung, Profil, App-Dateien und Verlauf ist noch nicht umgesetzt. Direkte Eintragsbearbeitungsdialoge werden weiterhin nur im Arbeitsspeicher gehalten.

`npm run test:drafts` prüft die native Ablage mit kontrollierten AsyncStorage-/Dateisystem-Adaptern: konkurrierende Schreibvorgänge, leere Versionszeilen, relative Pfade nach Neustart, Kontotrennung, fehlende Bilder, Kopierfehler und unklare Metadaten-Commits. Der Test ist Teil von `npm run check`. Die Implementierung verwendet die vorhandenen SDK-57-Abhängigkeiten; Grundlage sind die [offizielle Expo-FileSystem-Dokumentation](https://docs.expo.dev/versions/latest/sdk/filesystem/) und die installierten API-Definitionen.

## Direkte Änderungen an Einträgen

Migration `20260917090000_manual_entries.sql` ergänzt `tracking_entries.version` und `manual_entry_changes`. Nach der Migration Go neu starten. Jeder Eintrag in der Tagesübersicht enthält seine aktuelle `version`. KI-Korrekturen, direkte Änderungen und Löschungen erhöhen diese Version.

`PUT /v1/days/{date}/entries/{id}` erwartet `{request_id, version, entry}`. `request_id` ist eine UUID, `version` die beim Öffnen geladene Eintragsversion und `entry` das vollständige Werteobjekt ohne `id` oder `version`. Die Art (`food`/`activity`) bleibt unveränderlich. Zahlen sind nicht negativ und haben höchstens zwei Nachkommastellen. Für Essen sind Kalorien und alle drei Makros erforderlich, für Bewegung mindestens Dauer oder Distanz. Ein leeres optionales Zahlenfeld entspricht `null`; `0` ist ein bekannter Wert. Die Mengenbeschreibung rechnet Nährwerte nicht automatisch um: Alle Werte gelten für die gesamte angegebene Menge. Die Herkunft bleibt ausdrücklich auswählbar; das Bearbeiten einer Schätzung macht sie nicht automatisch exakt.

`DELETE` auf demselben Pfad erwartet `{request_id, version}`. Eine Bestätigung entfernt den Eintrag aus der Tagesübersicht und ihren Summen. Frühere Chatnachrichten und deren Bilder bleiben erhalten. Intern setzt der Server `deleted_at`; das ist keine vollständige Löschung persönlicher Daten. Die ursprünglichen Werte bleiben im Änderungsprotokoll. Ganze Chats einschließlich dieser Historie und Bilder können über die separate Tageschat-Löschung entfernt werden. Eine Backup-Aufbewahrungsregel steht noch aus.

Erfolg liefert HTTP 200 mit `{status:"updated"}` beziehungsweise `{status:"deleted"}`. Identische Wiederholungen derselben UUID verändern den Eintrag nicht nochmals, auch wenn eine frühere Antwort verloren ging. Eine UUID mit anderem Inhalt, eine veraltete Version oder eine ausstehende KI-Auswertung dieses Tages ergeben HTTP 409; ein nicht mehr verfügbarer oder fremder Eintrag HTTP 404. Nach einer unklaren Netzwerkantwort bleibt der Wiederholungsauftrag im geöffneten Dialog unverändert. Konflikte verlangen eine bewusste Übernahme der aktuellen Werte. Der Dialogentwurf ist nicht über einen App-Neustart hinweg gespeichert.

Die kurze Datenbanktransaktion koordiniert sich mit der Worker-Übernahme über denselben Advisory Lock und sperrt den Tag gegen gleichzeitige Nachrichten/Änderungen. Während `queued` oder `processing` sind neue direkte Änderungen gesperrt. Anschließend werden Eigentümer, aktuelle Eintragsversion und Art geprüft; Änderung und Protokolleintrag werden gemeinsam committed. Im manuellen Protokoll stehen Benutzer, Request-ID, erwartete Version und Vorher-/Nachher-Werte. `updated_by_message_id = null` kennzeichnet eine direkte Änderung; die Ursprungsnachricht bleibt zugeordnet. Der nächste KI-Auftrag erhält die aktuellen aktiven Einträge und neu berechneten Summen.

`npm run test:tracking` prüft zusätzlich direkte Korrekturen/Löschungen, Versionen nach KI-Änderungen, Nutzer-/Tagesgrenzen, unbekannte Werte gegenüber einer bekannten Null, unveränderte historische Nachrichten, Audit-Einträge, parallele Wiederholungen und konkurrierende Bearbeitungen. Dafür die lokale API wie im Testabschnitt beschrieben kurz stoppen. Die Tests nutzen temporäre Konten und eine kontrollierte KI ohne OpenAI-Aufrufe.

## Vollständige Tageschat-Löschung

Migration `20260917120000_day_deletion.sql` ergänzt Tagesversionen, inhaltsfreie Löschbelege/gesperrte UUIDs und die dauerhaften Foto-Löschaufträge. Nach der Migration die API neu starten.

`GET /v1/days/{date}/deletion` liefert `{day:{id,version,message_count,entry_count,photo_count}|null,pending_photo_deletions}`. Die Fotomenge umfasst auch ungebundene und unterbrochene Uploads dieses Tages. `GET /summary` enthält dieselben zusätzlichen Felder; `GET /messages` liefert `day_id` oder `null`. Nachrichten und Bildzuordnung werden dabei in einer konsistenten Lesetransaktion geladen.

`DELETE /v1/days/{date}` erwartet `{request_id,day_id,version}`. Erfolg liefert `{status:"deleted",pending_photo_deletions}`. HTTP 409 verlangt eine frische Vorschau und erneute Bestätigung; HTTP 404 bedeutet, dass der eigene Tageschat nicht mehr existiert. Identische Wiederholungen einer bereits bestätigten Request-ID liefern weiterhin HTTP 200, auch wenn für dasselbe Datum bereits ein neuer Chat angelegt wurde. Ungültige Eingaben liefern HTTP 400. Alle Abfragen und Löschungen sind an den angemeldeten Benutzer gebunden.

Nachrichten, sämtliche Einträge einschließlich früher entfernter Einträge, beide Änderungsverläufe und Analyseaufträge verschwinden in einer Transaktion. Fotoaufträge werden dabei dauerhaft gespeichert und vom Worker alle fünf Sekunden versucht. Storage-Fehler behalten die Zuordnung; neue Foto-URLs und Wiederholungen alter Upload-/Nachrichten-IDs sind gesperrt. Ein bereits ausgegebener Bildlink kann bis zur physischen Entfernung oder seinem Ablauf funktionieren. Bereits laufende externe KI-Aufrufe lassen sich nicht zurückrufen, aber keine spätere Antwort wird dem gelöschten Tag hinzugefügt.

Der Dialog zeigt Datum und Umfang, warnt vor dem Verwerfen lokaler Entwürfe und sperrt währenddessen die übrige Bedienung. Nach einer unklaren Netzwerkantwort bleibt exakt dieselbe Anfrage wiederholbar. Erfolgreiches Löschen verwirft nur den aktuellen lokalen Tagesentwurf und dessen Fotoauswahl; der Tag bleibt für neue Nachrichten geöffnet. Andere Sitzungen erkennen die gelöschte Tages-ID beim nächsten Polling und verwerfen auch ältere geladene Nachrichten, behalten aber ihre ungesendeten Texte. Der bereits bestätigte Löschauftrag wird vor dem Request lokal gespeichert. Nach einem Neustart öffnet sich beim betroffenen Tag der Wiederholungsdialog mit derselben Request-ID; eine erfolgreiche Bestätigung entfernt auch den dauerhaft gespeicherten lokalen Entwurf.

**Aufbewahrung:** Löschbelege und stillgelegte UUIDs bleiben ohne Chattext/Nährwerte/Bildinhalt bis zur Kontolöschung bestehen. Die Löschung entfernt den aktiven Bestand, keine bereits heruntergeladenen Dateien oder vorhandenen Sicherungen. Automatische Backups, Fristen und der Ablauf zur erneuten Anwendung von Löschungen nach einem Restore sind vor dem Produktivbetrieb einzurichten.

`npm run test:tracking` prüft die Kaskaden, Versionskonflikte durch Nachrichten/KI/manuelle Änderungen, Nutzergrenzen, parallele Löschwiederholungen, Schutz neu angelegter Tage, verspätete KI-/Nachrichten-/Upload-Anfragen und die Fotobereinigung nach Storage-Ausfall und Worker-Neustart. Die Tests verwenden temporäre Konten und kontrollierte Provider; die lokale API vorher stoppen.

## Fotos und Bildanalyse

Das lokale Setup ergänzt `FITTY_SUPABASE_SERVICE_ROLE_KEY` ausschließlich in `server/.env`. Bei einem bestehenden Checkout Migrationen und `npm run setup:local` ausführen; vorhandene OpenAI-Einstellungen bleiben erhalten. Go danach neu starten. Der Client bekommt weder den Storage-Service-Schlüssel noch den OpenAI-Schlüssel.

- **Auswahl:** Bis zu vier Bilder pro Nachricht aus Fotobibliothek/Dateiauswahl oder Kamera; im Browser auch aus der Zwischenablage im Nachrichtenfeld. Text und Fotoauswahl bleiben beim Tageswechsel, Neuladen und App-Neustart im lokalen Entwurf erhalten. Noch nicht versendete Bilder lassen sich entfernen.
- **Aufbereitung:** Ausgangsdateien bis 20 MiB, JPEG mit höchstens 2.048 Pixeln je Seite und Qualitätsfaktor 0,9. Die native Auswahl fordert die kompatible iOS-Repräsentation an. Browser ohne HEIC-Unterstützung benötigen JPEG/PNG/WebP; native HEIC-Konvertierung ist auf einem echten iPhone noch zu prüfen.
- **Serverprüfung:** JPEG, PNG oder WebP anhand tatsächlicher Daten und deklariertem MIME-Typ, höchstens 8 MiB, 4.096 Pixel je Seite und 12 Megapixel. Der Server decodiert vollständig und speichert nach erneutem JPEG-Encoding ohne EXIF-/Standortmetadaten. Der private Bucket `chat-attachments` akzeptiert ausschließlich die aufbereiteten JPEG-Dateien.
- **Wiederholung:** Eine dauerhafte `pending`-Reservierung wird nach erfolgreichem Storage-Upload `ready`. UUID und SHA-256 schützen vor abweichenden Inhalten bei erneutem Senden. Die Nachrichten-Transaktion ordnet alle Bilder gemeinsam zu; eine unvollständige Bildauswahl erzeugt keine Teilnachricht.
- **Abruf:** Besitzerprüfung über Go, anschließend signierter Storage-Pfad mit zehn Minuten Gültigkeit. Die Vorschau erneuert den Link nach neun Minuten und bietet bei Abruffehlern eine Wiederholung. Ein solcher Link gewährt bis zum Ablauf Zugriff und gehört nicht in Logs oder öffentliche Nachrichten.
- **Bereinigung:** Beim API-Start und stündlich werden höchstens 50 ungebundene Uploads ab einem Alter von 24 Stunden sowie Dateien gelöschter Konten bereinigt. Storage-Fehler behalten die Datenbankzeile für den nächsten Versuch. Noch referenzierte Bilder bleiben erhalten. Durch Tageschat-Löschung vorgemerkte Fotos werden zusätzlich alle fünf Sekunden in Batches von höchstens 50 Dateien versucht.

Der Analyse-Worker lädt höchstens vier Bilder der aktuellen Nachricht. Bei Text ohne neue Bilder lädt er alternativ die jüngste Bildnachricht aus den höchstens 30 berücksichtigten Verlaufsnachrichten desselben Tages. Dadurch bleiben Rückfragen und Korrekturen anhand vorheriger Etiketten möglich; derzeit werden diese Bilder auch bei sonstigen Textfragen erneut übertragen. Bilddaten gehen als Base64-`input_image` mit `detail: high` an OpenAI, ohne den Bucket öffentlich freizugeben.

Teller-/Lebensmittelfotos erzeugen erkennbare Schätzungen. Lesbare Etiketten werden mit der angegebenen verzehrten Menge verrechnet. Trainingsanzeigen können Gerätewerte liefern; eine Speisekarte bleibt Beratung. Bei unlesbaren Bildern oder unklarem Verzehr fragt Fitty nach. Ein historisches Bild allein darf keine neue Buchung begründen. Mehrere Bilder einer Mahlzeit sind keine mehreren Mahlzeiten.

Vorbereitete Fotos und die Identität eines begonnenen Sendevorgangs werden lokal dauerhaft gespeichert. Es gibt keine automatische Upload-Warteschlange, keine Wiederaufnahme innerhalb einer einzelnen Datei, kein Gesamtspeicherlimit und keine unabhängige Nährwertdatenbank. Abgebrochene Uploads werden vollständig wiederholt. Bilder werden für die Vorschau nicht zusätzlich als eigene Thumbnail-Datei gespeichert; verwendet wird dieselbe aufbereitete Datei.

## KI-Textverarbeitung

In `server/.env` einen eigenen `OPENAI_API_KEY` hinterlegen und `npm run dev:api` neu starten. Der Schlüssel gehört niemals in einen `EXPO_PUBLIC_`-Wert. Standard ist `gpt-5-mini`; `FITTY_OPENAI_MODEL` kann ein anderes Responses-kompatibles Modell mit Structured Outputs auswählen. Reasoning wird auf `low` und die Ausgabe auf höchstens 8.192 Tokens begrenzt. Diese Parameter müssen zum gewählten Modell passen.

Die API speichert eine Nachricht sofort. Ein persistenter Auftrag verarbeitet sie anschließend, ohne dass die App offen bleiben muss. Der Client lädt Nachrichten und Tageswerte regelmäßig nach. Vor Aktivierung gespeicherte Nachrichten werden **nicht automatisch** an OpenAI geschickt; sie lassen sich über „Auswerten“ einzeln nachholen.

Der Kontext enthält:

- Profil mit Zielen und Ernährungsvorlieben.
- Numerische Ziele, die für das ausgewählte Chatdatum gelten; nicht gesetzte Ziele bleiben `null`.
- Den ausdrücklich ausgewählten Kalendertag und die aktuelle Nachricht.
- Bis zu 30 vorherige Nachrichten, insgesamt höchstens etwa 48 KB Verlaufstext.
- Bis zu 200 aktive Einträge und die serverseitig berechneten Tageswerte.
- Gegebenenfalls die im Abschnitt zur Bildanalyse beschriebenen Bilddaten und deren Nachrichtenbezug.

Das Modell liefert Antworttext und strukturierte Vorschläge für Anlegen, Korrigieren oder Entfernen. Das Schema verlangt vollständige Einträge für Anlegen/Ändern und `null` ausschließlich beim Entfernen. Der Server prüft Felder, Einheiten, Wertebereiche, Typen, eigene Einträge sowie wörtliche Textbelege oder exakt zugeordnete aktuelle Bildbelege. Beratung und Rückfragen dürfen keine Schreibvorschläge enthalten. Das schützt die Datenstruktur; die fachliche Einordnung und Nährwertqualität müssen weiterhin mit echten Beispielen bewertet werden.

Essen und Aktivitäten liegen in `tracking_entries`; PostgreSQL speichert Nährwerte und Maße als `numeric` mit zwei Nachkommastellen. Tageswerte werden direkt daraus summiert. Eine Mengenänderung ersetzt den Eintrag, eine Entfernung markiert ihn als gelöscht. `entry_changes` hält vorherige/nachherige Werte und die Quellnachricht fest. Einträge, Änderungsprotokoll, Antwort und Abschlussstatus werden in **einer** Transaktion geschrieben. Fehler oder unvollständige Modellantworten buchen nichts.

Pro Go-Prozess läuft ein Worker. Nachrichten desselben Tages werden nacheinander bearbeitet. Ein fehlgeschlagener Auftrag hält spätere Nachrichten dieses Tages zurück, bis er erneut verarbeitet oder bewusst übersprungen wird. Ohne KI gespeicherte Notizen blockieren spätere Nachrichten nicht. Pro Nachricht sind maximal drei Versuche erlaubt; nach gewöhnlichen Providerfehlern erfolgt kein automatisches kostenpflichtiges Retry. Abgebrochene Prozesse werden über eine zweiminütige Lease erkannt und im verbleibenden Versuchslimit wieder aufgenommen. Eine neue Lease-ID verhindert das nachträgliche Verbuchen überholter Ergebnisse.

Das Zeitlimit für einen Verarbeitungsversuch beträgt 95 Sekunden; der HTTP-Client begrenzt den Modellaufruf auf 90 Sekunden. Es sind höchstens 20 Änderungen je Antwort und 200 aktive Einträge je Tag vorgesehen. Der Statusabruf liefert die letzten 1.000 Analysezustände eines Tages. Automatischer Offline-Versand im Client, Streaming, freie tagübergreifende KI-Abfragen und ein eigener monetärer Tagesdeckel gehören noch nicht zu diesem Stand.

Die Responses API wird mit `store:false` verwendet. Das deaktiviert die abrufbare Speicherung der Antwort als Responses-Anwendungszustand; es ist keine Zusage einer vollständig ausgeschlossenen dienstseitigen Aufbewahrung. Profil und Chatkontext werden für die Verarbeitung an OpenAI übertragen. [OpenAI: Datenverwendung und Aufbewahrung](https://platform.openai.com/docs/guides/your-data)

## Auf dem iPhone öffnen

Für den ersten Gerätetest Expo Go auf dem iPhone installieren. Das Projekt verwendet SDK 57; die Expo-Go-Version muss dazu passen. Die [Expo-Go-Downloadseite](https://expo.dev/go) zeigt die aktuell unterstützte Version. Ein späterer eigener Development Build ist ein separater Schritt.

1. iPhone und Entwicklungsrechner im selben eigenen WLAN verwenden. Beide Geräte müssen während des Tests verbunden bleiben.
2. Supabase starten, falls es nicht läuft. Die WLAN-Adresse des Rechners mit `ip -br -4 addr` ablesen; die Adresse der WLAN-Schnittstelle verwenden, nicht Docker-, VPN- oder `127.0.0.1`-Adressen.
3. Bereits laufende Go-/Expo-Entwicklungsserver auf 8787/8788 zuerst beenden und die folgenden Befehle mit Node 24 aus dem Repository-Stamm in zwei Terminals ausführen. Die temporären Prozessvariablen lassen die `.env`-Dateien unverändert.

Terminal 1:

```bash
FITTY_ADDR=0.0.0.0:8787 npm run dev:api
```

Terminal 2, Beispieladresse durch die eigene WLAN-Adresse ersetzen:

```bash
FITTY_LAN_IP=192.168.1.100
REACT_NATIVE_PACKAGER_HOSTNAME="$FITTY_LAN_IP" \
EXPO_PUBLIC_API_URL="http://$FITTY_LAN_IP:8787" \
EXPO_PUBLIC_SUPABASE_URL="http://$FITTY_LAN_IP:54321" \
npm run start --workspace @fitty/client -- --lan --go
```

4. Den QR-Code im Expo-Terminal mit der iPhone-Kamera scannen und in Expo Go öffnen. Den Zugriff auf das lokale Netzwerk erlauben. Der Link lautet `exp://<LAN-IP>:8788`. Der [offizielle Expo-Ablauf](https://docs.expo.dev/get-started/start-developing/) beschreibt den Start per QR-Code.
5. Mit dem vorhandenen Fitty-Konto aus `.local/zugang.txt` anmelden. Anschließend Nachricht senden, Kamera/Fotoauswahl, Fortschrittsfoto-Vergleich, Tastatur und Wiederherstellung eines ungesendeten Tageschat-Entwurfs nach App-Neustart prüfen. Testnachrichten landen im angemeldeten Konto; Chat-Auswertungen verwenden die konfigurierte KI wie im Browser.

Bei Verbindungsproblemen auf dem iPhone in Safari zuerst `http://<LAN-IP>:8787/healthz` aufrufen. Erwartet wird JSON mit `status: "ok"`. Ist das nicht erreichbar, WLAN-/Gastnetztrennung, Netzwerkerlaubnis und Firewall prüfen. Für den vollständigen Test müssen 8787 (Go), 8788 (Expo) und 54321 (Supabase Auth/Fotos) im lokalen Netz erreichbar sein. Ein Expo-Tunnel alleine macht Go und Supabase nicht erreichbar. Meldet Expo Go eine inkompatible SDK-Version, die App aktualisieren beziehungsweise einen passenden Development Build verwenden.

Für normale Desktop-Entwicklung beide Prozesse beenden und wieder mit `npm run dev:api` sowie `npm run dev:web` starten. Bei geänderter WLAN-Adresse die Testprozesse mit der neuen Adresse erneut starten und den neuen QR-Code verwenden.

Am 18. September 2026 wurden Go, Supabase Auth, das Expo-Go-Manifest und das iOS-Entwicklungsbundle über die lokale WLAN-Adresse vom Entwicklungsrechner aus geprüft. Die beiden Client-Serveradressen sind im Bundle korrekt auf das LAN gesetzt. Das ist noch kein Nachweis der Verbindung oder Bedienbarkeit vom tatsächlichen iPhone; dessen Test erfolgt durch den Benutzer.

SDK 57 benötigt mindestens iOS 16.4. Unter Linux ist kein lokaler iOS-Simulator verfügbar. Ein echtes iPhone, nativer Build, Tastatur-/Safe-Area-Verhalten und dauerhafte private Verteilung sind noch zu prüfen. Für den späteren produktiven Web- und App-Zugang werden HTTPS-Adressen verwendet; insbesondere benötigt der Browser für sichere Nachrichten-IDs einen sicheren Kontext (HTTPS oder localhost).

## Docker-Netzwerk auf diesem Rechner

Die automatischen Docker-Adresspools waren ausgeschöpft. Deshalb wurde nach Prüfung vorhandener Netze ein eigenes Netzwerk `fitty-local` mit `10.203.0.0/24` angelegt. Fremde Netzwerke werden nicht verändert.

```bash
npm run db:start -- --network-id fitty-local
```

Falls dieses Netzwerk entfernt wurde, nach erneuter Prüfung auf Adressüberschneidungen wieder anlegen:

```bash
docker network create --driver bridge --subnet 10.203.0.0/24 fitty-local
```

Die CLI-Konfiguration gilt ausschließlich lokal und wird nicht automatisch auf Coolify übertragen.

## Prüfungen

```bash
npm run check
npm run test:local
npm run test:photos
npm run test:tracking
npm run build:web
npm run bundle:ios
npm run db:lint
```

`check` prüft TypeScript, native Entwurfsspeichertests mit kontrollierten Adaptern, Zielvalidierung/Fortschritt, Foto-Aufnahmedaten und Paginierung, Go-Tests und `go vet`. Die Go-Unit-Tests decken Tokenprüfung, Kalenderdaten, Zeitzonen und Zielwerte, CORS sowie Responses-Anfragen, strenge JSON-Auswertung und fachliche KI-Validierung ab. `test:local` prüft Auth-Konfiguration, Profil, Benutzergrenzen, Datenhaltung, gleichzeitige Wiederholungen und beide Verlaufspaginierungen. Es verwendet zwei temporäre Konten und entfernt sie samt abhängigen Daten anschließend. Neue Nachrichten werden in diesem Test explizit mit `analyze:false` gesendet.

`test:photos` prüft private Speicherung gegen echtes lokales Supabase Storage, Formataufbereitung, signierte Abrufe, Nutzer-/Tageszuordnung, reine Bildnachrichten, Wiederholungen und Entfernen. Die Nachrichten tragen ausdrücklich `analyze:false`. Temporäre Storage-Objekte und Konten werden anschließend entfernt; verwaiste Metadaten bereinigt der reguläre Worker spätestens beim nächsten API-Start.

`test:tracking` verwendet einen kontrollierten Modell- und Storage-Adapter mit echter PostgreSQL-Datenhaltung, ohne OpenAI-Aufrufe. Es prüft Erfassung, Korrektur, Beratung, Löschung, Tageswerte, Rollback, Reihenfolge, Leasing, Bildkontext, Rückfragen und Upload-Bereinigung. Zieltests prüfen außerdem historische Gültigkeit, Nutzertrennung, optionale Werte, veraltete und parallele Schreibvorgänge, identische Wiederholungen nach weiteren Änderungen oder Tageswechsel sowie den Zielkontext des Workers. Den Go-Server vorher vollständig stoppen, damit weder KI-Worker noch Bereinigung Testaufträge übernehmen. Das Skript verweigert einen laufenden oder unklaren API-Status; fremde offene Analyseaufträge werden nicht angerührt. Temporäre Nutzer werden anschließend gelöscht.

Der Webexport liegt in `apps/client/dist`, das iOS-JavaScript-/Hermes-Bundle in `apps/client/dist/ios`. Letzteres ersetzt keinen Xcode-Build und ist keine installierbare App. Expo-Kompatibilität zusätzlich im Client-Verzeichnis mit `npx expo install --check` und `npx expo-doctor` prüfen.

Die Fortschrittsfoto-Fälle in `test:tracking` prüfen private Bildpfade, Nutzertrennung, Bild-/Datumsprüfung, unveränderte Upload-Wiederholungen, Kollisionen bei geändertem Inhalt, Retry nach Zeitzonenwechsel, Perspektivenfilter, mehrere Seiten mit gleichem Datum, Trennung von Chat/KI sowie dauerhafte Bereinigung bei Storage-Ausfall. Parallele Uploads und eine absichtlich während der Bereinigung gestartete Wiederholung dürfen keine Aufnahme duplizieren oder wiederherstellen. Vorhandene Chat- und Trackingtests laufen im selben Durchgang weiter.

### Tagesansicht nach UI-Vorlage, 17. September 2026

Grundlage ist `ui-preview/Fitty Tagesansicht.dc.html`; die Vorlage bleibt unverändert als Referenz erhalten. Der Client übernimmt die warmen Papierfarben, Instrument Sans/Serif, weiße Tageskarte, grüne Nachrichten und schmale Seitenleiste. Styles liegen in `apps/client/src/styles/app.ts` und `day-summary.ts`. Schriften samt OFL-Lizenzen liegen unter `apps/client/assets/fonts` und werden lokal geladen.

- Ab 900 Pixel Breite bleibt der Verlauf in der Seitenleiste. Mobil führt das Fitty-Logo zurück zum Tageschat; Verlauf und Profil bleiben in einer kompakten Kopfzeile erreichbar. Die Datumseingabe öffnet sich über den Kalender-Button.
- Tageswerte und Eintragsdetails verwenden ausschließlich die vorhandenen API-Daten. Aktivitätskalorien bleiben separat, Schätzungen sind markiert. Zielbalken verwenden die ausdrücklich hinterlegten Tagesziele. Keine angenommenen Kalorienbudgets, Schritte oder Wochenstatistiken aus der Vorlage.
- „Im Chat korrigieren“ ergänzt einen Entwurf mit Bezeichnung und Menge des Eintrags. Die Änderung wird erst beim bewussten Absenden ausgewertet. Vorhandener Text und Fotoauswahl bleiben erhalten. Schnellaktionen für Mahlzeit, Bewegung und Restaurant funktionieren genauso. „Bearbeiten“ und „Löschen“ öffnen zusätzlich den direkten Dialog aus dem Abschnitt zu Eintragsänderungen.
- Im Web fügt Enter eine neue Zeile ein; Strg+Enter beziehungsweise Cmd+Enter sendet. Laufende Texteingabe über eine Eingabemethode löst keinen Versand aus. Fotoauswahl, Kamera und Zwischenablage bleiben verfügbar.
- Die Bedienelemente haben mindestens 44 Pixel Höhe, beschriftete Icons und sichtbaren Tastaturfokus. Es wurden keine Animationen ergänzt. Bei geringer Fensterhöhe entfallen die Schnellaktionen zugunsten des Chats.

Prüfung: TypeScript, Webexport, iOS-JavaScript-/Hermes-Export und lokaler Expo-Versionsabgleich erfolgreich. Chromium bei 320, 390, 768, 1024 und 1440 Pixel Breite: keine horizontale Überbreite, Eingabe und Senden erreichbar; Korrektur ohne automatischen Versand oder Entwurfsverlust, Schnellaktionen, Tastaturkürzel, ungültiges Datum, Tagesentwürfe, Verlauf, Profilspeicherung, Neuladen und Logout geprüft. Chatantworten und Tageswerte waren kontrollierte UI-Testdaten; Auth und Profilspeicherung nutzten den lokalen Server. Ein separater Browserlauf prüfte echte private Foto-Uploads, Mehrfachauswahl, Zwischenablage, Entfernen, Tageswechsel, Vorschau, Vergrößerung und reine Bildnachrichten gegen Go/Supabase. Keine Konsolenfehler oder Warnungen in den geprüften Abläufen. Temporäre Konten und Dateien wurden entfernt; keine OpenAI-Aufrufe für die UI-Tests.

Die native Bedienung, iOS-Tastatur und Schriftgewichte auf einem echten iPhone bleiben zu prüfen. Der Hermes-Export ist keine installierbare iOS-App und ersetzt diese Geräteprüfung nicht.

Direkte Einträge wurden anschließend mit echten lokalen API-/Datenbank-/Storage-Zugriffen und temporären synthetischen Daten im Browser bei 1280 und 390 Pixel Breite geprüft: Dezimalkomma und fehlerhafte Eingaben, Mengenänderung ohne implizite Nährwertumrechnung, erhaltene Herkunft und Chatentwürfe, Versionskonflikt durch eine zweite Sitzung, bewusste Übernahme aktueller Werte, absichtlich nach Commit verlorene Antwort mit identischer Wiederholungs-UUID, leere Aktivitätskalorien gegenüber 0, Löschabbruch und Löschbestätigung, Neuladen sowie weiter zugängliches Foto der Ursprungsnachricht. Keine unerwarteten Konsolenfehler oder Warnungen; die simulierten Konflikt-/Netzwerkantworten erzeugen erwartete Browser-Netzwerkfehler. TypeScript, Go-Tests, `go vet` und die PostgreSQL-Integration einschließlich konkurrierender Schreiber sind erfolgreich. Die lokale Migration wurde angewendet und die API anschließend neu gestartet. Es gab keine OpenAI-Aufrufe für diese Abnahme.

### Durchgeführt am 16. September 2026

- TypeScript, Go-Tests und `go vet`: erfolgreich.
- Lokale Integration gegen Go und Supabase: erfolgreich, einschließlich Auth-Einstellungen, Benutzertrennung, paralleler Wiederholungen und Paginierung.
- Chromium bei 1280 und 390 Pixel Breite: Login, Tastaturbedienung, Nachrichtenspeicherung, Neuladen, Tageswechsel, Profil, Verlauf und erneuter Login erfolgreich. Ein zweiter isolierter Browserkontext lädt denselben gespeicherten Chat.
- Eine absichtlich nach der Speicherung verlorene Antwort wird über „Erneut senden“ ohne Duplikat verarbeitet. Der reguläre Browserablauf erzeugt keine Konsolenfehler oder Warnungen; die simulierte HTTP-503-Antwort wird erwartungsgemäß als Netzwerkfehler gemeldet.
- OpenAI-Client mit striktem Schema, Ablehnung unvollständiger Antworten und Nährwertvalidierung: Offline-Tests erfolgreich.
- Worker gegen PostgreSQL: Mahlzeit, Korrektur, Restaurantberatung, unbekannte Aktivitätskalorien, Änderungsprotokoll, wiederholter Commit, Rollback, Benutzertrennung und Neustart-Leases erfolgreich.
- Browser mit kontrollierten Antworten: KI-Status, Antwortanzeige, Tageswerte, Schätzungen und Wiederholung geprüft.
- Live-Test mit `gpt-5-mini` über die laufende Go-API: Frühstück mit Packungswerten, Mengenkorrektur desselben Eintrags, Restaurantberatung ohne Buchung und Walking Pad mit offenen Kalorien erfolgreich. Vier Modellaufrufe; gemessene Zeit vom Absenden bis zum abgerufenen Abschluss jeweils etwa 8 bis 16 Sekunden einschließlich Worker und Polling.
- Die Packungswerte ergeben für 250 g Skyr exakt 157,5 kcal, 27,5 g Protein, 10 g Kohlenhydrate und 0,5 g Fett. Die Korrektur auf 150 g ergibt 94,5 kcal, 16,5 g Protein, 6 g Kohlenhydrate und 0,3 g Fett. Erneutes Senden der abgeschlossenen Walking-Pad-Nachricht erzeugt weder eine weitere Antwort noch eine zusätzliche Buchung.
- Echte KI-Antworten und Tageswerte im Chromium bei 1280 und 390 Pixel Breite sowie nach Neuladen geprüft; keine Konsolenfehler oder Warnungen. Der Live-Test nutzte ausschließlich ein temporäres Konto mit synthetischen Beispielen; Konto und Testdaten wurden anschließend entfernt. Bestehende Nachrichten wurden nicht nachträglich ausgewertet.
- Foto-Tests gegen echte Go-API, PostgreSQL und privaten Supabase Storage erfolgreich: tatsächliches Format, Eigentümer-/Tagesgrenzen, signierte Abrufe, identische Upload-Wiederholung, reine Bildnachricht, Neuladen und Entfernen. Direkter Abruf durch andere Nutzer und über den öffentlichen Bucket-Pfad wird abgewiesen.
- Kontrollierte Worker-Tests prüfen aktuellen Bildkontext, Rückfrage mit späterer Textantwort und vorherigem Bild, fehlgeschlagenen Bildabruf ohne Buchung sowie Bereinigung abgebrochener Uploads. Unit-Tests decken zusätzlich JPEG/PNG/WebP, Größen-/Dimensionsgrenzen, EXIF-/Trailingdaten, Transparenz und den Storage-HTTP-Vertrag ab.
- Browser bei 1200 und 390 Pixel Breite: Mehrfachauswahl, Einfügen aus der Zwischenablage, Entfernen, Erhalt der Fotoauswahl beim Tageswechsel, Upload, Vorschau, Vergrößerung und reine Bildnachricht erfolgreich. Keine Konsolenfehler oder Warnungen im geprüften Ablauf.
- Sechs fachliche Live-Bildfälle mit `gpt-5-mini` im abschließenden Lauf erfolgreich: Banane als Schätzung; Etikett mit 94,5 kcal/16,5 g Protein/6 g Kohlenhydraten/0,3 g Fett für 150 g Skyr; Korrektur auf 200 g und 126 kcal im selben Eintrag; Walking Pad mit 30 Minuten/2 km/120 Geräte-kcal; Speisekarte ohne Buchung; unlesbares Bild mit Rückfrage. Wiederholtes Senden bucht nicht erneut. Gemessene Abschlusszeiten einschließlich Worker und Polling lagen in diesem Lauf etwa zwischen 4 und 12 Sekunden. Frühere Diagnoseversuche führten zu strengerem Aktionsschema, eindeutigen Bildbelegen und einer präziseren Rundungsanweisung.
- Für die Bildabnahme wurden ausschließlich temporäre Testkonten, selbst erstellte Beispielbilder und [Banana-Single.jpg von Evan-Amos](https://commons.wikimedia.org/wiki/File:Banana-Single.jpg) unter [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) verwendet; das Foto wurde für die Verarbeitung als JPEG neu codiert. Testkonten und Storage-Dateien wurden danach entfernt. Das Foto ist kein Bestandteil der App oder des Repositorys.
- Webexport und iOS-JavaScript-/Hermes-Export: erfolgreich.
- Neue Migration auf die vorhandene lokale Datenbank angewendet; Supabase-Schema-Linter ohne Fehler.

### Persönliche Tagesziele, 17. September 2026

TypeScript, Ziel-/Entwurfs-Unit-Tests, Go-Tests, `go vet`, PostgreSQL-Integration, Schema-Lint, Webexport und iOS-JavaScript-/Hermes-Export erfolgreich. Die Migration ist lokal angewendet und die API neu gestartet. Integrationstests bestätigen historische Ziele, Nutzertrennung, Versionskonflikte, gleichzeitige Schreibvorgänge, identische Wiederholungen auch nach Tageswechsel, erhaltene Ziele nach Chatlöschung sowie den passenden Zielstand im KI-Kontext. Der Responses-Vertrag überträgt gesetzte und leere Zielwerte unverändert.

Chromium gegen echte Go-API, Auth, PostgreSQL und Storage bei 1280, 390 und 320 Pixel Breite: Dezimalkomma, optionale Makros, Fortschritt, frühere Tage, Neuladen, erhaltene Text-/Fotoentwürfe beim Profilwechsel, konkurrierende Zieländerung, bewusste Bestätigung, Wiederholung derselben UUID nach verlorener Erfolgsantwort und Entfernen aller Ziele bei unveränderten Tageswerten erfolgreich. Ein ergänzender Browserlauf mit kontrollierten API-Antworten prüft Überschreitungen, zugängliche Fortschrittswerte und einen Tages-/Zeitzonenwechsel bei ungespeicherten Eingaben. Keine unerwarteten Konsolenfehler oder Warnungen; die simulierten HTTP-Konflikte und Netzwerkabbrüche erzeugen erwartete Netzwerkmeldungen. Temporäre Konten und Storage-Dateien sind entfernt. Diese Zieltests verwenden keine OpenAI-Aufrufe; die Qualität zielbezogener Beratung und die native Bedienung bleiben im Alltag beziehungsweise auf dem iPhone zu prüfen.

### Fortschrittsfotos, 17. September 2026

Migration lokal angewendet; TypeScript, Unit-Tests, Go-Tests, `go vet`, PostgreSQL-Integration, Schema-Lint, Webexport und iOS-JavaScript-/Hermes-Export erfolgreich. Die lokale API ist mit dem neuen Stand gestartet. Die Tests nutzen temporäre Konten und synthetische Bilder.

Chromium gegen echte Go-API, Supabase Auth, PostgreSQL und privaten Storage: Upload mit Datum/Perspektive, Datumsgrenzen, exakt identische UUID/Angaben/JPEG-Bytes bei verlorener Erfolgsantwort, chronologischer Verlauf, Perspektivenfilter, A/B-Vergleich, Vergrößern, Löschabbruch, Löschretry und physische Storage-Bereinigung erfolgreich. Neuladen erhält gespeicherte Aufnahmen; ein bestehender Tageschat-Entwurf bleibt nach dem Bereichswechsel erhalten und Tageswerte ändern sich nicht. Vergleich bei 1280, 390 und 320 Pixel Breite ohne horizontale Überbreite geprüft. Ein ergänzender Mock-Browserlauf deckt seiten- und filterübergreifende Vergleichsauswahl, den Abbruch einer ungespeicherten Auswahl und gesperrte Navigation bei unklaren Netzwerkantworten ab. Keine unerwarteten Konsolenfehler oder Warnungen; simulierte Netzwerkabbrüche erzeugen erwartete Browsermeldungen. Temporäre Konten und Storage-Dateien sind entfernt, ohne OpenAI-Aufrufe. Kamera, native Berechtigungsdialoge und Bedienung auf einem echten iPhone bleiben offen.

### Prüfgrenzen

- Echtes iPhone, nativer Build und Coolify-Betrieb stehen aus.
- Der reale Modellzugang, vier Textabläufe und sechs Bildfälle sind geprüft. Dies ist keine umfassende Bewertung von Portionsschätzungen, Beratung oder Modellzuverlässigkeit. Komplexe Teller und schlechte reale Etiketten bleiben zusätzliche Alltagsproben. Auch Etikettberechnungen stammen vom Modell und müssen bei Zweifeln kontrolliert werden. Die gemessenen Laufzeiten sind einzelne lokale Stichproben; tatsächliche API-Kosten wurden nicht ausgewertet.
- Kamera, native Berechtigungsdialoge, HEIC-Konvertierung und Upload mit echter iPhone-Tastatur sind trotz erfolgreichem iOS-Bundle noch am Gerät zu prüfen. Die Browserprüfung ersetzt diese Abnahme nicht.
- `npm audit` meldet 13 moderate Warnungen aus der Expo-Abhängigkeitskette (`uuid` und `decode-uri-component`). Eine kompatible automatische Behebung wird nicht angeboten; Upstream-Korrekturen vor produktiver Bereitstellung erneut prüfen.

## Quellen

- [Supabase: React-Native-Anmeldung](https://supabase.com/docs/guides/auth/quickstarts/react-native)
- [Supabase: Token mit getUser prüfen](https://supabase.com/docs/reference/javascript/auth-getuser)
- [Supabase: Konten administrativ anlegen](https://supabase.com/docs/reference/javascript/auth-admin-createuser)
- [Supabase: CLI-Konfiguration](https://supabase.com/docs/guides/local-development/cli/config)
- [Supabase: Aktuelle Sitzungskoordination](https://github.com/supabase/supabase-js/blob/master/packages/core/auth-js/migrations/lockless-coordination.md)
- [pgx: Verbindungspool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool)
- [Expo: ImagePicker](https://docs.expo.dev/versions/latest/sdk/imagepicker/)
- [Expo: ImageManipulator](https://docs.expo.dev/versions/latest/sdk/imagemanipulator/)
- [Expo: FileSystem](https://docs.expo.dev/versions/latest/sdk/filesystem/)
- [OpenAI: Images and vision](https://developers.openai.com/api/docs/guides/images-vision)
- [Expo: SDK-Versionen](https://docs.expo.dev/versions/latest/)

- [OpenAI: Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs)
- [OpenAI: GPT-5 mini](https://developers.openai.com/api/docs/models/gpt-5-mini)


Vollständige Tageschat-Löschung am 17. September 2026 zusätzlich geprüft: echte Go-/PostgreSQL-/Storage-Zugriffe mit temporären Testkonten, Bestätigungsabbruch, Versionskonflikt nach neuer Nachricht, absichtlich verlorene Erfolgsantwort und identischer Retry bei inzwischen neu angelegtem Chat, alte Nachrichten-ID abgewiesen, privates Bild physisch entfernt, Tageswerte geleert, Profil/anderer Tag erhalten, zwei Browser-Tabs und erhaltene ungesendete Texte im zweiten Tab. Lokal bestätigte Entwürfe/Fotos werden verworfen; erneutes Schreiben und Neuladen funktionieren. Dialog bei 1280, 390 und 320 Pixel Breite geprüft. Ein separater kontrollierter UI-Test deckt verspätete ältere Nachrichten und die Neuanlage eines Tages während der Wiederholung ab. Der vorhandene Foto-Browsertest bleibt grün. Keine unerwarteten Konsolenfehler; simulierte Netzwerkabbrüche und HTTP-Konflikte erzeugen erwartete Browser-Netzwerkmeldungen. TypeScript, Go-Tests, `go vet`, PostgreSQL-Integration, Schema-Lint, Webexport und iOS-JavaScript-/Hermes-Export erfolgreich. Die Migration ist lokal angewendet und die API neu gestartet. Temporäre Konten und Dateien entfernt; keine OpenAI-Aufrufe in diesen Tests. Prüfung auf einem echten iPhone und Backup-Aufbewahrung bleiben offen.


Dauerhafte Entwürfe am 17. September 2026 geprüft: sechs native Speichertests mit kontrollierten Adaptern sowie echter Chromium-/IndexedDB-Test für Blob-Wiederherstellung und konkurrierende Tabs. Der Browserlauf gegen Go, PostgreSQL, Auth und Storage umfasst Text-/Fotowiederherstellung nach Neuladen, getrennte Tage, verlorene erfolgreiche Sendeantwort mit anschließender Wiederholung derselben ID nach Neuladen, sichtbaren Speicherfehler und erfolgreiches erneutes Speichern, zwei konkurrierende Tabs mit beiden bewussten Konfliktentscheidungen, wiederaufgenommene Tageschat-Löschung nach verlorener Antwort, lokales Verwerfen mit Bestätigung, Logout/Login und getrennte Entwürfe zweier Konten. Normale Entwürfe und Konflikthinweise bei 320 Pixel Breite sowie Fotoentwürfe bei 390 Pixel Breite geprüft; Eingabe/Senden bleiben im sichtbaren Bereich. Die bestehenden Foto- und Eintragsbearbeitungs-Browsertests sind weiterhin grün. `npm run check`, abschließender Typecheck, Webexport und iOS-JavaScript-/Hermes-Export erfolgreich. Keine unerwarteten Konsolenfehler; nur erwartete Netzwerkmeldungen der simulierten Fehler. Temporäre Testkonten und Storage-Dateien entfernt; keine OpenAI-Aufrufe in diesen Prüfungen. Ein echter iPhone-Test und ein vollständiger Offline-Start bleiben offen.
