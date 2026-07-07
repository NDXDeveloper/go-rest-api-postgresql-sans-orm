# DOCKER.md — Cycle de vie Docker, expliqué de A à Z

Ce document décrit **toutes** les commandes Docker utiles au projet, de la construction à la  
suppression complète, avec l'explication de **ce que chaque commande fait** et **pourquoi**. Il  
s'adresse autant au débutant Docker qu'à celui qui veut comprendre précisément ce qu'il exécute.

> **Docker Compose v2 vs v1.** La commande moderne est **`docker compose`** (sous-commande de  
> `docker`, avec une espace). L'ancien binaire autonome est **`docker-compose`** (avec un tiret).  
> Les deux sont fonctionnellement équivalents ici. Dans ce document, on utilise `docker compose` ;  
> l'équivalent v1 est rappelé quand c'est utile. Les scripts du dossier `scripts/` détectent  
> automatiquement la version installée (voir `scripts/_commun.sh`).

## Table des matières

- [Vue d'ensemble de la pile](#vue-densemble-de-la-pile)
- [1. Construire les images](#1-construire-les-images)
- [2. Démarrer et arrêter la pile](#2-démarrer-et-arrêter-la-pile)
- [3. Consulter les journaux](#3-consulter-les-journaux)
- [4. Exécuter des commandes dans un conteneur](#4-exécuter-des-commandes-dans-un-conteneur)
- [5. Inspecter l'état](#5-inspecter-létat)
- [6. Supprimer les données (volume)](#6-supprimer-les-données-volume)
- [7. Supprimer images et réseaux](#7-supprimer-images-et-réseaux)
- [8. Nettoyage global du système Docker](#8-nettoyage-global-du-système-docker)
- [9. Reconstruction complète](#9-reconstruction-complète)
- [Tableau de correspondance v2 ↔ v1](#tableau-de-correspondance-v2--v1)
- [Aide-mémoire par intention](#aide-mémoire-par-intention)

---

## Vue d'ensemble de la pile

`docker-compose.yml` définit **deux services**, **un réseau** et **un volume** :

| Objet Docker            | Nom (par défaut)                  | Rôle                                                          |
|-------------------------|-----------------------------------|--------------------------------------------------------------|
| Service `postgres`      | conteneur `bibliotheque_postgres` | Base PostgreSQL 18 (+ pg_cron), initialisée par les scripts `sql/` |
| Service `api`           | conteneur `bibliotheque_api`      | L'API Go, démarrée **après** que PostgreSQL soit « healthy »  |
| Réseau (bridge)         | `<projet>_reseau_bibliotheque`    | Isole la communication API ↔ base                            |
| Volume nommé            | `<projet>_donnees_postgres`       | **Persiste** les données PostgreSQL (`/var/lib/postgresql`)   |

Le préfixe `<projet>` vient de `COMPOSE_PROJECT_NAME` (dans `.env`) ou, à défaut, du nom du dossier.

Points de conception importants :

- **`depends_on … condition: service_healthy`** : l'API ne démarre qu'une fois la base saine
  (sonde `pg_isready`), ce qui évite les erreurs de connexion au tout premier lancement.
- **Image PostgreSQL construite localement.** Le service `postgres` possède une section `build:`
  (`context: .`, `dockerfile: docker/postgres/Dockerfile`) : l'image officielle `postgres`
  n'embarque **pas** l'extension pg_cron. On étend donc l'image (`FROM postgres:18` + installation
  du paquet `postgresql-18-cron`) pour disposer des tâches planifiées côté serveur.
- **pg_cron préchargée au démarrage.** Le `command:` du service passe deux paramètres au serveur :
  `-c shared_preload_libraries=pg_cron` (chargement de la bibliothèque, indispensable) et
  `-c cron.database_name=bibliotheque` (base ciblée par l'ordonnanceur).
- **Scripts d'init montés dans `/docker-entrypoint-initdb.d/`** : au **premier** démarrage (volume
  vide), l'image PostgreSQL exécute les fichiers `00` → `10`, puis le script shell `99_*.sh`,
  **dans l'ordre** (garanti par les préfixes numériques). Aux démarrages suivants (volume déjà
  peuplé), ils **ne** sont **pas** rejoués.

---

## 1. Construire les images

Ce projet construit **deux** images : celle de l'**API** (racine `Dockerfile`) et celle de
**PostgreSQL + pg_cron** (`docker/postgres/Dockerfile`).

### Construire les images (via Compose)

```bash
docker compose build              # construit les DEUX services (postgres + api), avec cache
docker compose build --no-cache   # reconstruction totale, sans cache de couches
docker compose build api          # ne (re)construit que l'image de l'API
docker compose build postgres     # ne (re)construit que l'image PostgreSQL + pg_cron
```

- `build` construit les images définies par les `Dockerfile` des services :
  - **API** — `Dockerfile` (racine), build **multi-stage** : une étape compile le binaire Go, une
    étape produit une image finale minimale (~20 Mo, non-`root`).
  - **PostgreSQL** — `docker/postgres/Dockerfile` : part de `postgres:18` et installe le paquet
    `postgresql-18-cron`. En effet, l'image officielle `postgres` n'embarque **pas** pg_cron ;
    l'extension est ensuite **préchargée** au démarrage via `command:` (voir la
    [vue d'ensemble](#vue-densemble-de-la-pile)).
- `--no-cache` ignore le cache de couches Docker : utile quand une dépendance système a changé ou
  pour repartir d'une base parfaitement propre. Plus lent.

### Construire directement avec `docker build` (hors Compose)

```bash
docker build -t bibliotheque-api:local .                                       # image de l'API
docker build -t bibliotheque-postgres:local -f docker/postgres/Dockerfile .    # image PostgreSQL+pg_cron
```

- `-t nom:tag` nomme l'image.
- `-f` désigne un `Dockerfile` situé ailleurs qu'à la racine (ici celui de PostgreSQL).
- Le `.` final est le **contexte de build** (le dossier envoyé au démon Docker). Le
  `.dockerignore` en exclut le superflu (`.git`, `.env`…), ce qui accélère le build et évite de
  fuiter des secrets dans une couche.

> **Cache et rapidité.** Le `Dockerfile` de l'API copie d'abord `go.mod`/`go.sum` et télécharge les  
> dépendances **avant** de copier le code. Tant que ces deux fichiers ne changent pas, l'étape de  
> téléchargement est servie depuis le cache : les recompilations sont bien plus rapides.

---

## 2. Démarrer et arrêter la pile

### Démarrer

```bash
docker compose up -d --build
```

- `up` crée (au besoin) le réseau, le volume, puis démarre les conteneurs.
- `-d` (*detached*) : en arrière-plan, la main vous est rendue.
- `--build` : (re)construit les images (API et PostgreSQL) avant de démarrer.

Variante utile pour observer directement les logs (premier plan) :

```bash
docker compose up --build          # Ctrl+C pour arrêter
```

### Arrêter **sans** perdre les données

```bash
docker compose stop     # arrête les conteneurs, SANS les supprimer (redémarrage rapide via « start »)
docker compose start    # redémarre des conteneurs stoppés
```

```bash
docker compose down     # arrête ET supprime conteneurs + réseau ; CONSERVE le volume de données
```

- **`stop`/`start`** : met en pause/reprend les conteneurs **existants** (ni recréation, ni perte
  de données). Idéal pour une interruption courte.
- **`down`** : supprime conteneurs et réseau mais **garde le volume nommé** : vos données PostgreSQL
  survivent. C'est l'arrêt « propre » du quotidien.

### Redémarrer

```bash
docker compose restart          # redémarre tous les services
docker compose restart api      # redémarre uniquement l'API
```

Un simple `restart` **ne reconstruit pas** l'image : il relance le conteneur tel quel. Après une  
modification du **code Go**, il faut **reconstruire** (voir [§9](#9-reconstruction-complète)).

---

## 3. Consulter les journaux

```bash
docker compose logs               # tous les services, depuis le début
docker compose logs -f api        # suit (« follow ») les logs de l'API en direct
docker compose logs --tail=100 postgres   # les 100 dernières lignes de PostgreSQL
docker compose logs -f --since=10m        # les 10 dernières minutes, en direct
```

- `-f` : flux continu (comme `tail -f`). `Ctrl+C` pour sortir (n'arrête **pas** le conteneur).
- `--tail=N` : ne montre que les N dernières lignes.
- `--since` / `--until` : borne temporelle.

Les logs de l'API sont **structurés** (`slog`). En `LOG_FORMAT=json`, vous pouvez les filtrer,  
par exemple avec `jq` :

```bash
docker compose logs --no-log-prefix api | jq 'select(.statut >= 500)'
```

> **Message bénin au tout premier démarrage.** Dans les logs de `postgres`, une ligne  
> `FATAL:  database "bibliotheque" does not exist` peut apparaître **une seule fois** au tout  
> premier lancement. C'est **normal** : pg_cron (préchargée) tente de se connecter à la base  
> pendant le *bootstrap*, **avant** que l'entrypoint n'ait fini de créer la base et d'exécuter les  
> scripts d'init. Le serveur redémarre ensuite proprement et la base devient « healthy ». Aucune  
> action n'est requise.

---

## 4. Exécuter des commandes dans un conteneur

`exec` lance une commande **dans un conteneur déjà démarré**.

### Ouvrir un client SQL

```bash
# Client interactif psql via Compose (superutilisateur « postgres »).
# Depuis l'intérieur du conteneur, la connexion passe par le socket local :
# le superutilisateur n'a pas à ressaisir son mot de passe.
docker compose exec postgres psql -U postgres -d bibliotheque

# Forme équivalente avec « docker exec » et le nom du conteneur :
docker exec -it bibliotheque_postgres psql -U postgres -d bibliotheque

# Se connecter en tant que RÔLE APPLICATIF (droits restreints) :
docker compose exec postgres psql -U app_bibliotheque -d bibliotheque

# Depuis la MACHINE HÔTE (client psql installé localement), via le port publié :
psql -h localhost -p "${BDD_PORT_HOTE:-5432}" -U app_bibliotheque -d bibliotheque
```

### Vérifier extensions, tâches pg_cron, triggers

À l'invite `psql`, les **méta-commandes** listent rapidement les objets :
`\dx` (extensions), `\dt` (tables), `\df` (fonctions/procédures), `\dv` (vues), `\di` (index).

```bash
# Extensions installées (pgcrypto, pg_trgm, uuid-ossp, pg_cron)
docker compose exec postgres psql -U postgres -d bibliotheque -c '\dx'

# Tâches planifiées pg_cron, puis triggers installés
docker compose exec postgres psql -U postgres -d bibliotheque \
  -c "SELECT jobid, schedule, command FROM cron.job;" \
  -c "SELECT event_object_table, trigger_name FROM information_schema.triggers ORDER BY 1, 2;"
```

### Ouvrir un shell dans un conteneur

```bash
docker compose exec postgres bash    # PostgreSQL (image Debian : bash disponible)
docker compose exec api sh           # API (image Alpine : sh, pas bash)
```

> **`exec` vs `run`.** `exec` entre dans un conteneur **en cours d'exécution**. `docker compose  
> run --rm postgres <cmd>` démarre un conteneur **jetable** à partir de l'image du service  
> (utile ponctuellement). L'option `-T` (ex. dans `scripts/backup.sh`) **désactive l'allocation  
> d'un pseudo-TTY**, indispensable quand on redirige des flux (`| gzip`, `< fichier`).

### Sauvegarde / restauration logique

Le projet fournit des scripts prêts à l'emploi ; sous le capot, ils utilisent `exec` :

```bash
./scripts/backup.sh                                   # → backups/bibliotheque_<horodatage>.sql.gz
./scripts/restore.sh backups/bibliotheque_XXXX.sql.gz # écrase la base courante
```

Sous le capot, PostgreSQL sauvegarde avec **`pg_dump`** et restaure avec **`psql`** (dump « clair »)  
ou **`pg_restore`** (dump au format *custom*). Pour le faire à la main :

```bash
# Dump SQL « clair » (texte) de toute la base
docker exec bibliotheque_postgres pg_dump -U postgres bibliotheque > sauvegarde.sql

# Dump au format CUSTOM (-Fc : compressé, restaurable sélectivement avec pg_restore)
docker exec bibliotheque_postgres pg_dump -U postgres -Fc bibliotheque > sauvegarde.dump

# Rôles GLOBAUX (app_bibliotheque…), NON inclus dans le dump d'une seule base
docker exec bibliotheque_postgres pg_dumpall -U postgres --roles-only > roles.sql
```

```bash
# Restauration depuis un dump SQL « clair » : on rejoue le script avec psql
docker exec -i bibliotheque_postgres psql -U postgres -d bibliotheque < sauvegarde.sql

# Restauration depuis un dump au format custom : pg_restore (--clean recrée les objets)
docker exec -i bibliotheque_postgres pg_restore -U postgres -d bibliotheque --clean sauvegarde.dump
```

> Le `-i` de `docker exec` garde l'entrée standard ouverte : indispensable pour lire le dump  
> **depuis un fichier** (`< sauvegarde.sql`). Sans redirection (cas du `pg_dump` vers `>`), il  
> est inutile.

---

## 5. Inspecter l'état

```bash
docker compose ps                 # conteneurs du projet + état (Up/healthy…)
docker compose top                # processus tournant dans les conteneurs
docker compose config             # affiche la config finale (variables substituées) — pratique pour déboguer .env
docker stats                      # consommation CPU/mémoire en direct
docker volume ls                  # liste des volumes (repérez <projet>_donnees_postgres)
docker network ls                 # liste des réseaux (repérez <projet>_reseau_bibliotheque)
docker image ls                   # liste des images
docker inspect bibliotheque_api   # détail complet (JSON) d'un conteneur
```

`docker compose config` est particulièrement utile : il montre **exactement** ce que Compose va
faire une fois les `${VARIABLES}` de `.env` remplacées (ports, mots de passe masqués, montages).

---

## 6. Supprimer les données (volume)

Les données PostgreSQL vivent dans un **volume nommé** qui **survit** à `docker compose down`. Pour  
les **effacer** (repartir d'une base vierge, recharger le schéma et le seed) :

### Le plus simple (Compose)

```bash
docker compose down -v            # « -v » = supprime AUSSI les volumes du projet (DONNÉES PERDUES)
```

Puis relancer recrée tout de zéro (les scripts `sql/` sont rejoués) :

```bash
docker compose up -d --build
```

Le script `./scripts/reset.sh` enchaîne ces deux étapes en demandant confirmation.

### Supprimer un volume nommé manuellement

```bash
docker compose down               # d'abord détacher le volume (arrêter les conteneurs)
docker volume rm bibliotheque_donnees_postgres
```

- On **ne peut pas** supprimer un volume encore utilisé par un conteneur : d'où le `down` préalable.
- Le nom exact est visible via `docker volume ls`.

### Sauvegarder le volume (copie brute) avant de l'effacer

En plus de la sauvegarde **logique** (`pg_dump`, voir [§4](#4-exécuter-des-commandes-dans-un-conteneur)),  
on peut archiver le volume **fichier à fichier** avec un conteneur jetable qui le monte :

```bash
# La base doit être au repos : on arrête le service le temps de la copie.
docker compose stop postgres

# Archive tar.gz du contenu du volume dans ./backups
docker run --rm \
  -v bibliotheque_donnees_postgres:/data:ro \
  -v "$(pwd)/backups:/sauvegarde" \
  alpine tar czf /sauvegarde/volume_postgres.tar.gz -C /data .

docker compose start postgres
```

Restauration de cette archive brute dans le volume :

```bash
docker compose stop postgres
docker run --rm \
  -v bibliotheque_donnees_postgres:/data \
  -v "$(pwd)/backups:/sauvegarde" \
  alpine sh -c "rm -rf /data/* && tar xzf /sauvegarde/volume_postgres.tar.gz -C /data"
docker compose start postgres
```

> ⚠️ **Irréversible.** Supprimer le volume détruit définitivement toutes les données. Faites une  
> **sauvegarde** au préalable si nécessaire : logique (`./scripts/backup.sh`) ou brute (archive du  
> volume ci-dessus).

---

## 7. Supprimer images et réseaux

### Images

```bash
# Via Compose : supprime les images CONSTRUITES localement par ce projet
# (l'API ET l'image PostgreSQL + pg_cron, toutes deux buildées ici).
docker compose down --rmi local

# « all » retire en plus les images seulement « tirées ». Ici, les deux services
# étant buildés, le résultat est proche de « local ».
docker compose down --rmi all

# Manuellement, par nom/identifiant (voir « docker image ls ») :
docker image rm bibliotheque-api:local
docker image rm postgres:18
```

- `--rmi local` : retire les images **buildées** par le projet — désormais **deux** : celle de l'API
  et l'image PostgreSQL étendue (`postgres:18` + pg_cron).
- `--rmi all` : dans un projet classique, retire aussi les images seulement téléchargées. Les images
  de **base** (`postgres:18`, `golang:1.25-alpine`, `alpine:3.20`) se retirent au besoin à la main.
- `docker image rm` refuse de supprimer une image encore utilisée par un conteneur : supprimez
  d'abord le conteneur (`docker compose down`).

### Réseaux

```bash
# Le « down » supprime déjà le réseau du projet. Pour le faire à la main :
docker network rm bibliotheque_reseau_bibliotheque
```

Un réseau ne se supprime que si **aucun** conteneur n'y est attaché (donc après `down`).

---

## 8. Nettoyage global du système Docker

Ces commandes agissent sur **tout Docker**, pas seulement ce projet. À manier avec prudence.

```bash
docker builder prune          # supprime le cache de build inutilisé
docker builder prune -a       # supprime TOUT le cache de build

docker image prune            # supprime les images « pendantes » (sans tag)
docker image prune -a         # supprime toutes les images non utilisées par un conteneur

docker container prune        # supprime les conteneurs arrêtés
docker volume prune           # supprime les volumes non utilisés (⚠ données !)
docker network prune          # supprime les réseaux non utilisés

docker system prune           # conteneurs arrêtés + réseaux + images pendantes + cache build
docker system prune -a --volumes   # NETTOYAGE MAXIMAL : ajoute images inutilisées ET volumes (⚠⚠)
```

- `prune` demande **confirmation** (sauf avec `-f`).
- `docker system prune -a --volumes` peut supprimer des données d'**autres** projets : réservez-le
  à un vrai grand ménage.

### Nettoyage **limité au projet**

Pour tout supprimer **du projet uniquement** (conteneurs + volume + images locales + réseau) :

```bash
docker compose down --volumes --rmi local --remove-orphans
```

C'est exactement ce que fait `./scripts/clean.sh` (avec une confirmation). `--remove-orphans`  
supprime d'éventuels conteneurs orphelins d'anciennes versions du `docker-compose.yml`.

---

## 9. Reconstruction complète

### Après une modification du **code Go**

Le conteneur exécute un **binaire figé** dans l'image : un simple `restart` ne suffit pas. Il faut
**reconstruire** l'image :

```bash
# Rapide (réutilise le cache quand c'est possible)
docker compose up -d --build --force-recreate      # = make reconstruire

# Complet (sans cache) — après changement de dépendances ou pour lever un doute
docker compose build --no-cache api
docker compose up -d --force-recreate              # = ./scripts/rebuild.sh
```

- `--build` reconstruit l'image avant de démarrer.
- `--force-recreate` recrée le conteneur même si sa configuration n'a pas changé (garantit qu'il
  utilise bien la **nouvelle** image).
- Les **données** de la base sont **conservées** (le volume n'est pas touché).

### Repartir **totalement** de zéro (schéma + données + images)

```bash
docker compose down -v --rmi local --remove-orphans   # tout supprimer (DONNÉES PERDUES)
docker compose up -d --build                          # tout reconstruire et réinitialiser
```

> Après une réinitialisation du volume, les scripts `sql/00…10` (puis `99_mot_de_passe_app.sh`) sont  
> **rejoués** : vous retrouvez les extensions, le schéma complet, les tâches pg_cron et le jeu de  
> données de démonstration.

---

## Tableau de correspondance v2 ↔ v1

| Intention                        | Docker Compose **v2**              | Docker Compose **v1**              |
|----------------------------------|------------------------------------|------------------------------------|
| Démarrer (build + détaché)       | `docker compose up -d --build`     | `docker-compose up -d --build`     |
| Arrêter (garder données)         | `docker compose down`              | `docker-compose down`              |
| Arrêter + supprimer volumes      | `docker compose down -v`           | `docker-compose down -v`           |
| Logs en direct de l'API          | `docker compose logs -f api`       | `docker-compose logs -f api`       |
| Exécuter une commande            | `docker compose exec postgres …`   | `docker-compose exec postgres …`   |
| Reconstruire sans cache          | `docker compose build --no-cache`  | `docker-compose build --no-cache`  |
| État des services                | `docker compose ps`                | `docker-compose ps`                |

Les commandes `docker` « bas niveau » (`docker image rm`, `docker volume rm`, `docker system  
prune`…) sont **identiques** quelle que soit la version de Compose.

---

## Aide-mémoire par intention

| Je veux…                                                    | Commande                                                        |
|-------------------------------------------------------------|----------------------------------------------------------------|
| …tout démarrer                                              | `docker compose up -d --build`                                 |
| …arrêter pour la journée (sans rien perdre)                | `docker compose down`                                          |
| …voir ce qui se passe                                       | `docker compose logs -f api`                                   |
| …ouvrir un client SQL                                       | `docker compose exec postgres psql -U postgres -d bibliotheque` |
| …appliquer une modification de code Go                     | `docker compose up -d --build --force-recreate`               |
| …réinitialiser la base (schéma + seed)                     | `docker compose down -v && docker compose up -d --build`      |
| …reconstruire l'API sans cache                              | `docker compose build --no-cache api`                         |
| …tout supprimer pour ce projet                              | `docker compose down --volumes --rmi local --remove-orphans`  |
| …faire un grand ménage Docker (tout le système)            | `docker system prune -a --volumes`                            |
| …sauvegarder / restaurer la base                            | `./scripts/backup.sh` / `./scripts/restore.sh <fichier>`      |
