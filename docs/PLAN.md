# Umsetzungsplan

Stand: 17. September 2026. Meilensteine 1 bis 4 sind implementiert: Anmeldung, Profil, Tageschat, Text-Tracking und private Fotoanhänge mit Bildanalyse. Die lokale Prüfung umfasst kontrollierte Antworten, echte Modellaufrufe und den Browser bei Desktop- und Handybreite. Kamera/HEIC auf einem echten iPhone, weitere Alltagsbilder sowie Meilensteine 5 und 6 bleiben offen. Der konkrete Prüfstand steht in der [Entwicklungsanleitung](ENTWICKLUNG.md).

## Ziel

Fitty übernimmt den bisherigen persönlichen Ablauf aus dem ChatGPT-Projekt „Fitness“ in eine eigene Anwendung: ein Chat pro Tag für Essen, Sport und Walking Pad, ergänzt um Fotos, Screenshots und Beratung zum jeweiligen Tag. Die Bedienung bleibt frei und gesprächsbasiert. Strukturierte Daten ermöglichen nachvollziehbare Tageswerte und den späteren Zugriff auf vergangene Trackings.

Die technische Aufteilung ist im [Architekturkonzept](ARCHITEKTUR.md) beschrieben.

## Vereinbarte Anforderungen

- Privates Projekt mit dem Namen Fitty.
- Native iOS-App sowie Weboberfläche mit Login und gemeinsamem Datenbestand.
- Ein Tageschat als zentrale Oberfläche; vergangene Tage bleiben zugänglich.
- Freie Texteingabe und Einfügen kopierter Inhalte.
- Fotos und Screenshots hochladen, erkennen und für das Tracking auswerten.
- Essen mit Kalorien und Nährwerten erfassen; Sport und Walking-Pad-Einheiten dokumentieren.
- Im selben Chat Fragen zu Essensplanung, Restaurantbesuchen und dem aktuellen Tag stellen.
- Datenhaltung auf dem eigenen Server.
- Go als Backend und selbst gehostetes Supabase über das vorhandene Coolify.
- OpenAI API für Gesprächsantworten und Bildverständnis.

## Arbeitsannahmen für die erste Version

Diese Entscheidungen halten den Einstieg klein und können vor der jeweiligen Umsetzung geändert werden:

- Ein persönliches Benutzerkonto, keine öffentliche Registrierung. Daten werden trotzdem einem Benutzer zugeordnet.
- React Native mit Expo, TypeScript und Expo Router bildet die gemeinsame Basis für iOS und Web.
- Tageszuordnung anhand der Profilzeitzone; zunächst `Europe/Berlin`.
- Tracking-Einträge können direkt aus eindeutigen Aussagen entstehen und anschließend korrigiert werden. Es gibt keine Pflichtbestätigung für jede Mahlzeit.
- Wesentliche Unklarheiten werden vor dem Buchen erfragt. Reine Mengenabschätzungen können als solche gekennzeichnet gespeichert werden.
- Gegessene Kalorien und gemeldeter Aktivitätsverbrauch werden getrennt dargestellt. Eine automatische Verrechnung mit einem Tagesziel ist zunächst nicht vorgesehen.
- Für KI-Antworten ist eine Internetverbindung erforderlich. Vollständige Offline-Synchronisation gehört nicht zur ersten Version.

## Nutzerabläufe

### Tageschat

Nach der Anmeldung öffnet sich der heutige Tag. Oben steht eine kompakte Zusammenfassung mit Energie, Protein, Kohlenhydraten, Fett und Aktivitäten. Darunter befinden sich Nachrichten, Bilder und Tracking-Einträge. Die Eingabe unterstützt Text, Kamera und Bildauswahl; im Browser auch Datei-Uploads und eingefügte Bilder, soweit der Browser dies unterstützt.

Ein Datumswechsel öffnet einen eigenen Tageschat. Beim Lesen eines alten Tages darf eine neue Nachricht nicht unbemerkt dem heutigen Tag zugeordnet werden. Nachträgliche Angaben wie „Das war gestern“ müssen einen sichtbaren Zieltag haben.

### Essen per Text oder Foto erfassen

Beispiele:

- „Frühstück: 250 g Skyr und eine Banane.“
- Foto eines Tellers mit „Mein Mittagessen“.
- Foto von Verpackung und Nährwerttabelle mit „Davon habe ich 150 g gegessen“.

Fitty erkennt Lebensmittel und verwendet vorhandene Mengen- und Verpackungsangaben. Geschätzte Bestandteile und Portionen bleiben sichtbar als Schätzung markiert. Bei einem unklaren Bild fragt Fitty nach, statt einen vermeintlich exakten Eintrag zu erzeugen.

Mehrere Fotos können dieselbe Mahlzeit beschreiben, etwa Teller und Verpackung. Sie dürfen nicht automatisch als mehrere Mahlzeiten gezählt werden.

### Aktivitäten erfassen

Freitext und Screenshots können Dauer, Strecke, Geschwindigkeit und angezeigten Energieverbrauch liefern. Fitty unterscheidet abgelesene Gerätewerte von eigenen Schätzungen. Fehlende Werte werden nicht als gemessen dargestellt.

### Beratung im Tageskontext

„Heute Abend gehe ich italienisch essen“ oder ein Foto einer Speisekarte führt zu einer Antwort unter Berücksichtigung von Profil, den für diesen Tag gültigen persönlichen Zielen und Tagesstand. Geplante oder lediglich vorgeschlagene Mahlzeiten zählen nicht zur aufgenommenen Energie.

### Persönliche Tagesziele

Kalorien, Protein, Kohlenhydrate und Fett lassen sich im Profil einzeln als freiwillige Ziele hinterlegen. Leere Felder bedeuten kein Ziel; es gibt keine automatisch berechneten Vorgaben. Änderungen gelten ab heute in der gespeicherten Profilzeitzone. Frühere Tage zeigen weiterhin ihren damaligen Zielstand. Die Tageskarte zeigt Fortschritt, verbleibende Menge oder Überschreitung sachlich an; Bewegung erhöht das Essensziel nicht automatisch.

### Fortschrittsfotos

Regelmäßig persönliche Fortschrittsfotos aufnehmen oder vorhandene Bilder auswählen und privat mit Aufnahmedatum sowie Perspektive speichern. Der eigene Fotoverlauf erlaubt die Auswahl zweier Aufnahmen zum Vorher-/Nachher-Vergleich, Filter nach Perspektive und das Vergrößern der vollständigen Bilder. Die Fotos sind unabhängig vom Tageschat und werden nicht an die KI übertragen. Einzelne Aufnahmen lassen sich nach Bestätigung entfernen. Die erste Umsetzung enthält keine automatischen Erinnerungen oder körperbezogene Auswertung.

### Korrekturen und Verlauf

„Es waren nur 150 g“, „Das war Tofu statt Hähnchen“ oder das Entfernen eines falschen Eintrags aktualisiert den bestehenden Datensatz und die Tageswerte. Frühere Nachrichten bleiben zum Verständnis erhalten. Eine Tagesliste oder ein Kalender öffnet ältere Chats einschließlich ihrer Bilder und Einträge.

## Umfang der ersten nutzbaren Version

- Login und dauerhaft nutzbare Sitzungen auf iOS und Web.
- Profil mit Zeitzone, persönlichen Zielen und Ernährungsvorlieben.
- Tageschat mit Text und mehreren Bildanhängen pro Nachricht.
- Mahlzeiten-, Verpackungs-, Trainings- und Walking-Pad-Bilder auswerten.
- Strukturierte, bearbeitbare Essens- und Aktivitätseinträge.
- Tagesübersicht aus gespeicherten Einträgen berechnen.
- Beratung mit Tageskontext und Trennung zwischen geplant und tatsächlich erfasst.
- Verlauf und Zugriff auf Bilder früherer Tage.
- Verständliche Zustände für Upload, Analyse, Rückfrage und Fehler; erneutes Senden ohne doppelte Buchung.
- Privater Zugriff, Backups und überprüfte Wiederherstellung.

## Spätere Erweiterungen

Diese Punkte sind mögliche Ausbaustufen und keine Voraussetzung für die erste Version:

- Apple Health, automatische Geräteanbindung und Barcode-Scan.
- Spracheingabe, Erinnerungen und Push-Mitteilungen.
- Umfangreiche Wochenstatistiken, Rezepte und wiederkehrende Mahlzeiten.
- Benutzerbezogener Datenexport aus der Oberfläche.
- Automatisierter Import alter ChatGPT-Chats.
- Mehrbenutzerbetrieb, öffentliche Registrierung und vollständige Offline-Synchronisation.

## Meilensteine

### 1. Projektgrundlage und lokale Entwicklung

- [x] Geplante Verzeichnisstruktur für App, Go-Server und Dokumentation anlegen.
- [x] Aktuelle, zueinander passende Versionen prüfen und im Projekt festhalten.
- [x] Lokalen Supabase-Betrieb, Konfiguration und SQL-Migrationen einrichten.
- [x] Konfigurationsvorlagen ohne Zugangsdaten und eine Startanleitung erstellen.
- [x] Go-Server mit Healthcheck sowie Expo-Startansicht für Web und iOS erstellen.

Abnahme: Beide Oberflächen und das Backend lassen sich anhand der README starten. Eine leere Datenbank lässt sich vollständig aus versionierten Migrationen aufbauen.

Prüfgrenze: Das iOS-JavaScript-Bundle wurde erzeugt. Ein nativer Xcode-Build und der Start auf einem echten iPhone sind in dieser Linux-Umgebung noch nicht geprüft.

### 2. Anmeldung, Profil und persistenter Tageschat

- [x] Supabase Auth für das persönliche Konto einrichten und öffentliche Registrierung deaktivieren.
- [x] Sitzungsverwaltung in iOS und Web sowie Tokenprüfung und Benutzerzuordnung in Go umsetzen.
- [x] Profil, Tageschats und Nachrichten speichern und laden.
- [x] Datumsnavigation, Zeitzone und Wiederaufnahme alter Chats umsetzen.
- [x] Grundlayout für Handy und Desktop einschließlich Tastatur- und Fokusverhalten erstellen.

Abnahme: Eine Nachricht bleibt nach Neuladen und erneutem Login erhalten und erscheint auf dem anderen Gerät. Tageswechsel und Nachträge verändern keine fremden Tageschats. Unangemeldete Zugriffe werden abgewiesen.

Prüfgrenze: Die native Sitzungsverwaltung und das iOS-Bundle sind implementiert; ein Lauf auf einem echten iPhone bleibt offen. Browser und API werden lokal gegen Supabase geprüft.

### 3. Text-Tracking und Beratung

- [x] OpenAI über den Go-Server anbinden; API-Schlüssel ausschließlich serverseitig verwalten.
- [x] Strukturierte Analyse für Erfassung, Korrektur, Beratung und Rückfragen festlegen.
- [x] Essens- und Aktivitätseinträge einschließlich Herkunft und Schätzungsstatus speichern.
- [x] Schreibvorschläge der KI validieren und Tageswerte im Backend berechnen.
- [x] Wiederholte Anfragen und Korrekturen ohne doppelte Einträge verarbeiten.
- [x] Profil, Tageswerte und passenden Chatkontext für Antworten zusammenstellen.

Abnahme: Ein Frühstück wird erfasst, eine spätere Mengenänderung korrigiert denselben Eintrag, eine Restaurantfrage erzeugt keine Mahlzeit. Ein erneut übermittelter Auftrag erhöht die Tageswerte nicht nochmals.

Lokale Abnahme am 16. September 2026 bestanden: `gpt-5-mini` erfasst 250 g Skyr anhand angegebener Packungswerte und korrigiert denselben Eintrag auf 150 g. Eine Restaurantfrage verändert weder Einträge noch Tageswerte. Eine Walking-Pad-Einheit bleibt mit Dauer und Strecke separat erfasst; unbekannte Kalorien bleiben offen. Wiederholtes Senden erzeugt kein Duplikat. Echte Antworten und Tageswerte bleiben nach Neuladen im Desktop- und mobilen Browser erhalten. Dies prüft den Ablauf anhand synthetischer Beispiele; die Qualität von Portionsschätzungen und weiteren Alltagssituationen bleibt separat zu bewerten.

### 4. Fotos und Screenshots

- [x] Kamera, Bildauswahl und Web-Upload einschließlich mehrerer Anhänge umsetzen.
- [x] Dateityp, Größe und Benutzerzuordnung prüfen; iPhone-Bilder in unterstützte Analyseformate umwandeln (native Geräteprüfung offen).
- [x] Bilder mit passenden Größen und Vorschauen in privaten Storage-Buckets speichern.
- [x] Bildanalyse mit Text und Tageskontext verbinden; Portionen, Etiketten und Gerätewerte unterscheiden.
- [x] Persistente Verarbeitungszustände, Wiederholung nach Fehlern und bereinigte abgebrochene Uploads umsetzen.
- [x] Bilder, Analyseergebnis und Tracking-Eintrag im Chat gemeinsam darstellen.

Abnahme: Ein Tellerfoto ergibt eine sichtbare Schätzung; eine lesbare Nährwerttabelle mit Mengenangabe ergibt eine nachvollziehbare Berechnung. Ein Trainings-Screenshot übernimmt lesbare Werte. Eine Speisekarte bleibt eine Beratungsgrundlage. Unlesbare Bilder führen zu einer Rückfrage. Erneute Verarbeitung erzeugt keine Duplikate.

Lokale Prüfung am 16. September 2026 bestanden: öffentliches Bananenfoto als sichtbare Schätzung, synthetisches Skyr-Etikett mit 94,5 kcal für 150 g, Korrektur desselben Eintrags auf 200 g/126 kcal, Walking-Pad-Anzeige mit 30 Minuten/2 km/120 Geräte-kcal, Speisekarte ohne Buchung und unlesbares Bild mit Rückfrage. Wiederholte Nachrichten erzeugen keine Duplikate. Browserprüfung deckt Mehrfachauswahl, Zwischenablage, Entfernen, Tagesentwürfe, Vorschauen, Vergrößern und Neuladen ab. Komplexe Tellerfotos, schlechte reale Etiketten und der native Kamera-/HEIC-Ablauf müssen zusätzlich im Alltag geprüft werden.

### 5. Alltagstauglichkeit auf iOS und im Web

Stand vom 17. September 2026: Die Tagesansicht übernimmt Farben, Schriften, Seitenleiste und Chatgestaltung aus `ui-preview/Fitty Tagesansicht.dc.html`. Navigation und Tageskarte sind für kleine Bildschirme verdichtet. Einträge lassen sich direkt bearbeiten, mit Bestätigung aus den Tageswerten entfernen oder über einen ergänzten Chatentwurf korrigieren. Versionsprüfung und Request-IDs verhindern unbemerkte Überschreibungen und doppelte Änderungen. Angezeigte Werte stammen aus dem Backend. Zielbalken erscheinen ausschließlich für ausdrücklich hinterlegte persönliche Tagesziele.

- [x] Tagesübersicht, Verlauf und direkte Bearbeitung beziehungsweise Entfernung von Einträgen fertigstellen.
- [x] Tagesentwürfe mit Text/Fotos und offenen Sende-/Löschanfragen lokal dauerhaft speichern; Speicherfehler und konkurrierende Tabs behandeln.
- [x] Freiwillige Kalorien-/Makroziele mit zeitlicher Gültigkeit speichern, Tagesfortschritt anzeigen und den gültigen Zielstand in den KI-Kontext aufnehmen.
- [x] Privaten Fortschrittsfoto-Verlauf mit Aufnahmedatum, Perspektive, Vergleich zweier Aufnahmen und bestätigtem Entfernen ergänzen.
- [ ] Verbleibende Lade-/Fehlerzustände und Wiederaufnahme direkter Eintragsdialoge nach Neustart vervollständigen.
- [ ] Auf echtem iPhone und im Desktop-Browser mit denselben Beispielen testen.
- [ ] Bedienbarkeit mit Tastatur, Screenreader-Beschriftungen, Kontraste und reduzierte Bewegung prüfen.
- [x] Das Löschen von Chats samt zugehörigen Einträgen und Bildern anbieten.

Abnahme: Ein kompletter Beispieltag lässt sich auf beiden Plattformen ohne Datenverlust erfassen, nachlesen und korrigieren. Gelöschte Inhalte verschwinden aus dem aktiven Datenbestand und den Tageswerten. Der Backup-Aufbewahrungszeitraum ist dokumentiert.

Teilabnahme im Browser am 17. September 2026: direkte Essens-/Aktivitätskorrekturen, leere Werte gegenüber 0, veralteter Dialog nach Änderung aus einer anderen Sitzung, Wiederholung nach verlorener Antwort und bestätigte Eintragslöschung gegen Go/PostgreSQL/Storage erfolgreich. Frühere Nachrichten und Fotos bleiben erhalten. Die separate vollständige Chatlöschung ist ebenfalls implementiert: Bestätigung mit aktueller Vorschau, Schutz gegen veraltete Löschanfragen und dauerhafte Fotobereinigung nach Storage-Ausfällen. Zusätzliche Browserabnahme der vollständigen Löschung mit echten privaten Fotos, zwei Tabs, verlorener Antwort und sicherer Neuanlage bei Desktop-/Handybreite erfolgreich. Dauerhafte Entwürfe sind anschließend mit Fotos, Neuladen, identischem Sende-/Lösch-Retry, simuliertem Speicherausfall und Kontowechsel im Browser geprüft. Echte iPhone-Prüfung und Backup-Aufbewahrungsregel sind weiterhin offen.

### 6. Betrieb auf Coolify und private Bereitstellung

- [ ] Supabase, Go-Service und Weboberfläche mit HTTPS, persistentem Speicher und getrennten Secrets bereitstellen.
- [ ] Interne Verbindung zwischen Go und PostgreSQL sowie private Storage-Zugriffe konfigurieren.
- [ ] Falls Passwortreset per E-Mail vorgesehen ist: SMTP und erlaubte Rücksprungadressen einrichten.
- [ ] Backups für Datenbank, Bilddateien und notwendige Betriebskonfiguration einrichten.
- [ ] Wiederherstellung mit Beispieltag und Bildanhang in einer getrennten Umgebung durchführen.
- [ ] Gewählten iOS-Verteilungsweg einrichten und Installation auf dem eigenen Gerät testen.
- [ ] Betriebsanleitung mit Updates, Migrationen, Wiederherstellung und API-Kostenbegrenzung dokumentieren.

Abnahme: iOS-App und Weboberfläche verwenden denselben Server. Daten überstehen einen Neustart und eine Wiederherstellung. Der Zugang auf dem eigenen iPhone funktioniert über den vereinbarten Verteilungsweg.

## Prüfstrategie

Die fachlichen Regeln erhalten gezielte automatisierte Tests: Tageszuordnung, Berechnung aus gespeicherten Einträgen, Korrekturen, Benutzergrenzen und Schutz vor Doppelbuchungen. Für KI-Antworten werden kontrollierte Beispielausgaben verwendet, damit diese Tests reproduzierbar bleiben.

Zusätzlich wird ein kleiner Satz repräsentativer Texte und Bilder mit der realen OpenAI-Anbindung geprüft: Mahlzeit, Verpackung, schlecht lesbares Etikett, Walking-Pad-Screenshot und Speisekarte. Bewertet werden korrekte Zuordnung, nachvollziehbare Herkunft und der Umgang mit Unsicherheit. Ein Essensfoto gilt nicht als Beleg für eine exakt messbare Kalorienzahl.

Gerätetests decken Fotoauswahl, Upload, Tastaturverhalten, Sitzungsablauf und Verbindungsabbrüche ab. Ein Restore-Test prüft ausdrücklich auch die Bilddateien.

## Noch offene Entscheidungen

| Entscheidung | Zeitpunkt | Vorgeschlagener Ausgangspunkt |
| --- | --- | --- |
| OpenAI-Modell und Ausgabenlimit | Live-Abnahme Meilenstein 3 | Vorläufig `gpt-5-mini`, 8.192 Output-Tokens; anhand eigener Beispiele auf Qualität, Latenz und Kosten prüfen, per Umgebungsvariable anpassbar |
| Übernahme bisheriger Projektanweisungen | Bei verfügbaren Inhalten | Persönliche Ziele und Vorlieben sind hinterlegbar; Anweisungen aus „Fitness“ nach Bereitstellung gezielt übernehmen |
| Native iOS-Verteilung | Vor Meilenstein 5 | Passenden privaten Verteilungsweg und Apple-Voraussetzungen klären; Expo Go nur für frühe Entwicklung betrachten |
| Domains, SMTP und Speicherziel | Vor Meilenstein 6 | Vorhandene Coolify-Infrastruktur verwenden; Zugangsdaten außerhalb des Repositorys verwalten |
| Backup-Ziel und Aufbewahrung | Vor Meilenstein 6 | Wiederherstellbare Kopie außerhalb des Anwendungsservers; Zeitraum bewusst festlegen |
| Alte Fitness-Chats | Nach der ersten Version | Import separat anhand eines bereitgestellten Exports oder ausgewählter Chats planen |

Das ChatGPT-Projekt „Fitness“ ist aus dieser Arbeitsumgebung nicht direkt zugänglich. Ein Import und die Übernahme seiner Anweisungen benötigen bereitgestellte Inhalte. API-Nutzung wird separat vom ChatGPT-Abonnement abgerechnet.
