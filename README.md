# NordFolio

> **OBS:** Dette er et “vibecodet” projekt! Alt brug er på eget ansvar..!

Et lokalt afviklet og privatlivsfokuseret porteføljeværktøj til Nordnet-eksportfiler. NordFolio rekonstruerer din transaktionshistorik, beregner din reelle anskaffelsesværdi (AVCO) og visualiserer tidsvægtet afkast (TWR) og drawdowns – 100% lokalt på din egen maskine.

Al din transaktionshistorik og dine personlige indstillinger gemmes **lokalt i en enkelt JSON-databasefil (`db.json`)**. Ingen finansielle data eller personlige oplysninger forlader nogensinde din computer.

**OBS:** Værktøjet understøtter KUN danskesprogede eksportfiler i sin nuværende stand..! Af samme årsag findes NordFolio kun på dansk og viser porteføljeværdi i DKK.

---

<p align="center">
  <img src="assets/nordfolio-demo.gif" alt="NordFolio Demonstration" width="100%">
</p>

---

## Nøglefunktioner

1. **Robust parser til Nordnets CSV-eksport**:
   - Detekterer automatisk om filen er encodet i **UTF-8** eller **WINDOWS-1252**.
   - Håndterer tab-separerede filer med europæiske/danske talformater (komma som decimalseparator).
2. **Beskyttelse mod dubletter**:
   - Springer automatisk allerede importerede linjer over ved hjælp af Nordnets unikke transaktions-ID'er. Du kan derfor trygt uploade overlappende eksportfiler.
3. **Avanceret visualisering af din portefølje**:
   - **Porteføljeværdi (DKK)**: Rekonstruerer en daglig tidslinje over din porteføljeværdi opdelt på aktier/fonde, ETF'er og kontanter, og viser den sammenholdt med din investerede kapital.
   - **Simpelt afkast (%)**: Viser dit kumulative afkast over tid, overlejret med et simuleret MSCI World-indeks (8% p.a.) som benchmark.
   - **Tidsvægtet afkast (TWR %)**: Viser dit tidsvægtede afkast (den institutionelle standard), som fjerner effekten af dine ind- og udbetalinger, overlejret med MSCI World som benchmark.
   - **Portfolio Drawdown (%)**: Beregner dit maksimale værditab (peak-to-trough) over tid, så du kan visualisere din reelle risikoprofil og volatilitet.
   - **Månedligt afkast (%)**: Konverterer øjeblikkeligt tidslinjen til et farvekodet søjlediagram (grøn for overskud, rød for fald) grupperet pr. kalendermåned.
   - Hurtige tidsfiltre: **1 måned, 6 måneder, 1 år, YTD, MTD eller Maks (altid)**.
4. **Automatisk valutadetektering & Live FX-kurser**:
   - Finder og registrerer automatisk nye valutaer (USD, EUR, SEK, NOK, GBP osv.) ved CSV-import.
   - Henter automatisk live-valutakurser via en sikker backend-proxy (Frankfurter API med automatisk fallback to ExchangeRate-API). Løsningen kører fuldstændig uden om browserens adblockere (adblocker-proof same-origin proxy).
   - Viser visuelle indikatorer for valutaudsving i realtid i forhold til dine gemte kurser.
5. **Interaktiv styring af aktiver**:
   - Rediger tickersymboler og giv dine aktiver beskrivende navne.
   - Klassificer dine aktiver som aktier/fonde eller ETF'er (kontantbeholdningen beregnes automatisk).
   - Indtast manuelle priser (realtidskurser) og vælg handelsvaluta.
   - Giv dine forskellige Nordnet-depoter/konti beskrivende navne (f.eks. "Aktiesparekonto").
6. **Detaljeret transaktionslog**:
   - Gennemse hele din samlede transaktionshistorik med en lynhurtig, lokal søgefunktion.

---

## Teknologier & Arkitektur

- **Backend**: Skrevet i Go  og bruger udelukkende standardbiblioteket `net/http` til routing, en simpel og trådsikker JSON-database samt backend-proxy til valutakurser).
- **Frontend**: En ren Single Page Application (SPA) bygget med **semantisk HTML5**, **Vanilla CSS3** og **Chart.js hentet via CDN** til interaktive grafer. Ingen tunge frameworks eller lignende.

---

## Kom i gang

### 1. Forudsætninger
Du skal have **Go 1.16+** installeret på din maskine.
Tjek din version med:
```bash
go version
```

### 2. Kør applikationen lokalt
Kør Go-applikationen direkte fra rodmappen:
```bash
go run .
```

Appen vil som standard starte op på `http://localhost:8080` og oprette/bruge `db.json` as din lokale database.

####  Konfiguration (Valgfrit)
Du kan konfigurere port, databasesti og offline-tilstand via flags eller environment-variabler:

- **Port**: `-port 3000` (eller via nvironment-variabel `PORT=3000`)
- **Databasesti**: `-db <min-egen-portefoelje.json>`
- **Offline-tilstand (Deaktiver FX API)**: `-disable-fx-api` (eller environment-variabel `DISABLE_FX_API=true`). Dette lukker fuldstændigt af for alle eksterne netværksforbindelser, hvor du selv sætter valutakurserne manuelt.

```bash
# Eksempel: Kør på port 3000 i fuld offline-tilstand
go run . -port 3000 -disable-fx-api
```

### 3. Kør med Docker
Du kan køre applikationen i en Docker-container. Da Dockerfilen ligger i `docker-directoriet`, skal du angive stien med `-f` og sætte build-konteksten til rod-directory (`.`):

```bash
# 1. Byg Docker-image
docker build -f docker/Dockerfile -t nordfolio .

# 2. Kør containeren (åbner port 8080)
docker run -d -p 8080:8080 --name portfolio-tracker nordfolio

# 3. Kør containeren i offline-tilstand (uden eksterne API-kald)
docker run -d -p 8080:8080 -e DISABLE_FX_API=true --name offline-portfolio-tracker nordfolio
```

#### Kør i Docker med persistent storage
Har du ikke mappet din db-fil ud til din maskines filsystem mister du dine konfigurationer og importerede data.

For at gemme persiste dine data skal du **mounte et directory fra din computer til containerens `/data`-directory** og starte applikationen med `-db`-flag'et:

```bash
# 1. Opret et directory på din maskine
mkdir -p data

# 2. Kør containeren med persistent storage
docker run -d \
  -p 8080:8080 \
  -v /absolut/sti/til/data:/data \
  --name portfolio-tracker \
  nordfolio \
  -db /data/db.json

# 3. Kør i offline-tilstand med persistent storage
docker run -d \
  -p 8080:8080 \
  -v /absolut/sti/til/data:/data \
  -e DISABLE_FX_API=true \
  --name offline-portfolio-tracker \
  nordfolio \
  -db /data/db.json
```

### 4. Åbn dashboardet
Åbn din browser og gå til:
**[http://localhost:8080](http://localhost:8080)**

---

## Sådan importerer du dine data

1. Log ind på din **Nordnet**-konto.
2. Gå til din transaktionshistorik (Eksport) og download filen som CSV (hedder typisk `transactions-and-notes-export.csv`).
3. Træk og slip (eller vælg) filen i panelet **Importer Nordnet transaktioner** på dashboardet.
4. Klik på **Indlæs og importer**. Dit dashboard er nu live!
5. Gå til fanen **Aktiver & Priser** for at tilføje realtidskurser, klassificere dine ETF'er og give dine depoter beskrivende navne.

> **VIGTIGT:** Din CSV-eksport **SKAL indeholde hele din transaktionshistorik** (helt fra din første handel). Da NordFolio rekonstruerer din historiske balance, reelle anskaffelsesværdi (AVCO) og løbende kontosaldi kronologisk fra bunden, vil en delvis historik føre til forkerte startværdier, skæve kurver og misvisende performancetal. Sørg for at eksportere det fulde tidsrum.

---

## Kørsel af test
For at afvikle testsuiten, som verificerer CSV-parsing, decimalberegninger, interne/eksterne overførsler og AVCO-omkostningsberegninger:
```bash
go test -v ./...
```
