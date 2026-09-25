# Fitty

Fitty wird ein privater Fitnessbegleiter für iOS und Web: ein Chat pro Tag für Essen, Sport, Walking Pad und Fragen zum Alltag. Die OpenAI API unterstützt Kalorien, Nährwerte, Beratung und die Auswertung von Fotos.

## Projektstand

Stand: 17. September 2026. Anmeldung, Profil, Tageschat sowie Text- und Foto-Tracking sind vorhanden:

- Privater E-Mail-/Passwort-Login mit Supabase Auth und gespeicherter Sitzung.
- Tageschat mit freier Texteingabe, Datumswechsel und paginiertem Verlauf.
- Tagesentwürfe mit Text und Fotos bleiben auf diesem Gerät nach Neuladen, App-Neustart und erneutem Login erhalten; offene Sendungen lassen sich ohne Duplikate fortsetzen.
- Nachrichten und Profil werden über Go in PostgreSQL gespeichert und nach Neuladen oder erneutem Login wieder geladen.
- Profil mit Name, Zeitzone, Zielen und Ernährungsvorlieben.
- Freiwillige Kalorien- und Makroziele mit Fortschrittsanzeige; frühere Tage behalten ihre damaligen Ziele, die auch der KI als Tageskontext dienen.
- Responsive Oberfläche für Desktop und Handy; gemeinsame Expo-Codebasis für iOS und Web.
- KI-Antworten, Essens- und Aktivitätseinträge, Korrekturen per Chat sowie Tageswerte aus der Datenbank.
- Einträge direkt bearbeiten und aus der Tagesübersicht entfernen, mit Schutz gegen veraltete Änderungen und wiederholte Anfragen.
- Ganze Tageschats nach Bestätigung löschen, einschließlich Einträgen, Änderungsverläufen und privater Fotos.
- Sichtbare Schätzungen, getrennte Aktivitätskalorien und Wiederholung nach Verarbeitungsfehlern.
- Bis zu vier Fotos pro Nachricht, Kamera, Bildauswahl und Einfügen aus der Zwischenablage im Browser; private Speicherung, Vorschauen und Vergrößerung.
- Eigener privater Verlauf für Fortschrittsfotos mit Aufnahmedatum, Perspektive und Vergleich zweier Aufnahmen.

**Prüfstand:** Text-Tracking und Fotos sind mit PostgreSQL, privatem Storage und echten `gpt-5-mini`-Antworten geprüft. Bildtests decken Lebensmittelschätzung, Etikettberechnung, Korrektur anhand eines vorherigen Fotos, Trainingsanzeige, Speisekartenberatung und ein unlesbares Bild ab. Desktop- und mobiler Browser sind geprüft; Kamera/HEIC auf einem echten iPhone und Coolify-Bereitstellung stehen noch aus. Ohne OpenAI-Schlüssel bleibt Fitty als Tagebuch nutzbar.

## Stack

| Bereich | Technologie |
| --- | --- |
| iOS und Web | React Native, Expo, TypeScript, Expo Router |
| Anwendungslogik | Go mit pgx |
| Datenbank | PostgreSQL innerhalb von Supabase |
| Anmeldung | Supabase Auth |
| Bilder | Supabase Storage mit privatem Bucket |
| KI | OpenAI Responses API über Go, konfigurierbares Modell |
| Betrieb, geplant | Selbst gehostet über Coolify |

## Lokal starten

Voraussetzungen: Node.js **24.21.0 LTS** (siehe `.node-version`), npm 11, Go **1.27** und Docker.

```bash
npm ci
npm run db:start
npm run db:migrate
npm run setup:local
```

Auf dem aktuellen Rechner statt `npm run db:start` das bereits angelegte Netzwerk verwenden:

```bash
npm run db:start -- --network-id fitty-local
```

Das Setup erzeugt einen privaten Entwicklungszugang sowie ignorierte `.env`-Dateien. Die Anmeldedaten stehen ausschließlich in **`.local/zugang.txt`**. Erneutes Setup verwendet dasselbe lokale Konto; es schreibt die lokale Konfiguration erneut. Es ist ausschließlich für die lokale Supabase-Instanz vorgesehen.

In zwei Terminals starten:

```bash
npm run dev:api
```

```bash
npm run dev:web
```

**Oberfläche: http://localhost:8788** · Healthcheck: http://127.0.0.1:8787/healthz · Supabase Studio: http://127.0.0.1:54323

## KI aktivieren

In der ignorierten Datei `server/.env` ergänzen:

```dotenv
OPENAI_API_KEY=dein_api_schluessel
FITTY_OPENAI_MODEL=gpt-5-mini
```

Anschließend die Go-API neu starten. Den Schlüssel niemals in die Client-Konfiguration schreiben oder committen. `gpt-5-mini` wurde lokal mit vier echten Beispielnachrichten geprüft; das Modell bleibt konfigurierbar. Den Modellzugang auf jeder neuen Installation prüfen.

Neue Nachrichten werden nach dem Speichern automatisch verarbeitet. Bereits ohne KI gespeicherte Nachrichten können einzeln über **„Auswerten“** nachgeholt werden. Reine Beratung und geplantes Essen bleiben ohne Buchung; überprüfbare Mengenänderungen korrigieren vorhandene Einträge. Tageswerte stammen aus gespeicherten Einträgen, nicht aus dem Antworttext.

Für die Analyse werden Profil, die für den ausgewählten Tag gültigen Ziele, Tageschat, passende Tracking-Daten und gegebenenfalls Bilddaten an OpenAI übertragen. Die API wird separat vom ChatGPT-Abonnement abgerechnet.

## Tagesziele festlegen

In der Tagesübersicht **„Tagesziele festlegen“** oder im Profil **„Deine Tagesziele“** öffnen. Kalorien, Protein, Kohlenhydrate und Fett sind einzeln optional; ein leeres Feld bedeutet kein Ziel. Mit **„Tagesziele speichern“** gelten Änderungen ab heute in deiner gespeicherten Profilzeitzone. Frühere Tage behalten ihre damaligen Ziele.

Die Tageskarte zeigt Aufnahme, Ziel und Fortschritt. Aktivitätskalorien bleiben separat. Fitty verwendet die gesetzten Ziele für den Beratungskontext; es berechnet keine Zielvorgaben und ändert sie nicht per Chat. Technische Details stehen in der [Entwicklungsanleitung](docs/ENTWICKLUNG.md#persönliche-tagesziele).

## Fotos verwenden

Im Chat **„Fotos“** oder **„Kamera“** wählen; im Browser lassen sich Bilder auch in das Nachrichtenfeld einfügen. Ergänze zum Beispiel „Mein Mittagessen“, „Davon habe ich 150 g gegessen“ oder eine Frage zur Speisekarte. Eine reine Bildnachricht ist ebenfalls möglich; bei unklarem Verzehr fragt Fitty nach.

`npm run setup:local` hinterlegt auch den privaten Storage-Zugang als `FITTY_SUPABASE_SERVICE_ROLE_KEY` in `server/.env`. Für eine bestehende Installation zuerst die Migrationen anwenden, das lokale Setup erneut ausführen und Go neu starten; der vorhandene OpenAI-Schlüssel bleibt erhalten. Der Service-Schlüssel darf niemals in die Client-Konfiguration gelangen.

Die App bereitet Bilder bis 20 MiB als JPEG mit höchstens 2.048 Pixeln je Seite auf. Go prüft jeden Upload, entfernt Metadaten und speichert höchstens 8 MiB pro Bild. Unbenutzte Uploads werden nach 24 Stunden bereinigt. Details und Grenzen stehen in der [Entwicklungsanleitung](docs/ENTWICKLUNG.md#fotos-und-bildanalyse).

## Fortschrittsfotos vergleichen

**„Fortschrittsfotos“** in der Navigation öffnen, ein Foto aufnehmen oder auswählen und Aufnahmedatum sowie Perspektive festlegen. Speichere regelmäßig neue Aufnahmen und wähle im Verlauf zwei davon als **Foto A** und **Foto B**. Der Vergleich zeigt beide Bilder vollständig; durch Antippen kannst du sie vergrößern. Ein Filter hilft, etwa zwei Aufnahmen von vorne zu finden.

Die Bilder bleiben privat in deinem Konto und werden nicht an die KI geschickt. Sie haben einen eigenen Verlauf unabhängig von Tageschats. Einzelne Fortschrittsfotos lassen sich nach Bestätigung entfernen. Noch nicht gespeicherte Fotoauswahlen bleiben nur in der geöffneten Ansicht erhalten. Details stehen in der [Entwicklungsanleitung](docs/ENTWICKLUNG.md#fortschrittsfotos).

## Prüfen und bauen

```bash
npm run check
npm run test:local
npm run test:photos
npm run test:tracking
npm run build:web
npm run bundle:ios
npm run db:lint
```

`test:tracking` prüft die Worker ohne Modellaufrufe; dafür den laufenden Go-Server zuvor stoppen. `test:local` und `test:photos` benötigen laufendes Supabase und die Go-API und speichern ausdrücklich ohne KI-Auswertung. Sie verwenden temporäre Konten und entfernen ihre Testdaten anschließend. `bundle:ios` erzeugt ein JavaScript-/Hermes-Bundle, keine signierte installierbare App.

## Dokumentation

- [Umsetzungsplan](docs/PLAN.md): Anforderungen und Meilensteine.
- [Architektur](docs/ARCHITEKTUR.md): Datenmodell, Zugriff, KI und Bilder.
- [Entwicklung](docs/ENTWICKLUNG.md): Konfiguration, iPhone-Verbindung und Prüfstand.
- [Bereitstellung](deploy/README.md): Geplanter Coolify-Betrieb.
