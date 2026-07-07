# PostgreSQL en profondeur

Ce document explique **le moteur** tel qu'il est utilisé dans ce projet, et **ce qui change**  
par rapport au dépôt jumeau MariaDB. L'API est en **parité fonctionnelle stricte** avec la version  
MariaDB : seule la couche base de données diffère. Ici, on détaille ces différences, l'outil
`psql`, les rôles, les schémas, les extensions, le modèle **MVCC**, les sauvegardes, `VACUUM`,
`ANALYZE`, `EXPLAIN`, et la planification des tâches.

Documents liés : [README.md](README.md) · [DATABASE.md](DATABASE.md) (schéma détaillé) ·
[DOCKER.md](DOCKER.md) · [docs/SECURITE.md](docs/SECURITE.md) ·
[docs/PERFORMANCES.md](docs/PERFORMANCES.md).

---

## Table des matières

- [Architecture PostgreSQL](#architecture-postgresql)
- [MariaDB ↔ PostgreSQL : le comparatif](#mariadb--postgresql--le-comparatif)
- [`psql`, le client en ligne de commande](#psql-le-client-en-ligne-de-commande)
- [Rôles et privilèges](#rôles-et-privilèges)
- [Schémas](#schémas)
- [Extensions](#extensions)
- [Transactions et MVCC](#transactions-et-mvcc)
- [`VACUUM` et `ANALYZE`](#vacuum-et-analyze)
- [`EXPLAIN` et `EXPLAIN ANALYZE`](#explain-et-explain-analyze)
- [Sauvegardes : `pg_dump` / `pg_restore`](#sauvegardes--pg_dump--pg_restore)
- [Planifier des tâches : Event Scheduler, `pg_cron`, cron Linux, Kubernetes](#planifier-des-tâches--event-scheduler-pg_cron-cron-linux-kubernetes)

---

## Architecture PostgreSQL

Comprendre la structure logique de PostgreSQL aide à lire le reste :

```
  Instance (un serveur "postgres", un port : 5432)
    │
    ├── Rôles (GLOBAUX à l'instance : utilisateurs + groupes unifiés)
    │     ├── postgres            ← superutilisateur (administration)
    │     └── app_bibliotheque    ← rôle applicatif (moindre privilège)
    │
    └── Bases de données
          └── bibliotheque
                └── Schémas (espaces de noms DANS une base)
                      ├── public   ← nos tables, vues, types, fonctions, procédures
                      ├── cron      ← créé par l'extension pg_cron (cron.job…)
                      └── pg_catalog / information_schema  ← catalogues système
```

Points clés, souvent différents de MariaDB :

- **Une instance héberge plusieurs bases**, mais une connexion est **rattachée à une seule base** :
  on ne fait pas de requête « inter-bases » comme le `base.table` de MySQL/MariaDB. Pour cloisonner
  à l'intérieur d'une base, on utilise des **schémas** (voir plus bas).
- **Les rôles sont globaux** à l'instance (pas « par base ») : un même rôle peut se connecter à
  plusieurs bases.
- **Le processus** : un *postmaster* accepte les connexions et lance **un processus par connexion**
  (architecture multi-processus). D'où l'importance de **limiter le nombre de connexions** (pool
  côté application ; `max_connections` côté serveur, défaut 100).
- **MVCC** (Multi-Version Concurrency Control) : chaque écriture crée une **nouvelle version** de
  ligne au lieu d'écraser l'ancienne. Les lecteurs ne bloquent pas les écrivains, et vice-versa
  (voir [Transactions et MVCC](#transactions-et-mvcc)).

---

## MariaDB ↔ PostgreSQL : le comparatif

Ce tableau résume les adaptations de **dialecte** et de **mécanismes** entre les deux dépôts. Tout  
le reste (schéma logique, endpoints, logique métier) est identique.

| Thème | MariaDB (dépôt jumeau) | PostgreSQL (ce dépôt) |
|-------|------------------------|------------------------|
| **Clé auto-incrémentée** | `INT AUTO_INCREMENT` | `bigint GENERATED ALWAYS AS IDENTITY` (norme SQL ; `ALWAYS` interdit d'imposer une valeur, sauf `OVERRIDING SYSTEM VALUE`) |
| **Paramètres de requête** | `?` anonymes | `$1, $2, …` numérotés |
| **Récupérer l'id inséré** | `LastInsertId()` (résultat) | clause `RETURNING id` + `Scan()` |
| **Ordonnanceur de tâches** | Event Scheduler intégré (`CREATE EVENT`) | extension **`pg_cron`** (`cron.schedule(...)`) — non intégrée, à précharger |
| **Type énuméré** | `ENUM('a','b')` inline sur la colonne | `CREATE TYPE … AS ENUM` : un **vrai type** réutilisable et ordonné |
| **Document JSON** | `JSON` (texte validé) | **`JSONB`** : binaire, **indexable en GIN**, opérateurs `->`, `->>`, `@>` |
| **Recherche insensible à la casse** | `LIKE` (selon collation `_ci`) | **`ILIKE`** (+ index **GIN `pg_trgm`** pour `'%mot%'`) |
| **Doublon (contrainte UNIQUE)** | erreur `1062` | `SQLSTATE` **`23505`** (`unique_violation`) |
| **Clé étrangère (parent absent)** | erreur `1452` | `SQLSTATE` **`23503`** (`foreign_key_violation`) |
| **Suppression bloquée par FK RESTRICT** | erreur `1451` | `SQLSTATE` **`23001`** (`restrict_violation`) — code **distinct** de 23503 ! |
| **Violation de `CHECK`** | erreur `4025`/`3819` | `SQLSTATE` **`23514`** (`check_violation`) |
| **Erreur métier levée à la main** | `SIGNAL SQLSTATE '45000'` | `RAISE EXCEPTION '…'` → `SQLSTATE` **`P0001`** |
| **Délimiteur des routines** | `DELIMITER $$ … $$` | **dollar-quoting** `$$ … $$` (pas de `DELIMITER` à gérer) |
| **Corps procédural** | SQL/PSM | **PL/pgSQL** (`LANGUAGE plpgsql`) |
| **Gestion d'erreur dans une routine** | `DECLARE … HANDLER FOR …` | bloc **`BEGIN … EXCEPTION WHEN … THEN …`** (savepoint implicite) |
| **Paramètres de sortie d'une procédure** | `OUT` + variables de session `@x` + connexion dédiée | **`INOUT`** ; `CALL proc($1, …, NULL, NULL)` renvoie une **ligne** → simple `QueryRow().Scan()` |
| **`modifie_le` automatique** | `ON UPDATE CURRENT_TIMESTAMP` sur la colonne | **trigger** `BEFORE UPDATE` (`fn_maj_modifie_le`) |
| **Vue matérialisée** | absente (émulée) | **native** : `CREATE MATERIALIZED VIEW` + `REFRESH … CONCURRENTLY` |
| **Moindre privilège** | `GRANT` par table à `user@host` | **rôles** + `REVOKE … FROM PUBLIC` + **`ALTER DEFAULT PRIVILEGES`** (droits appliqués aux futurs objets) |
| **Superutilisateur** | `root` | `postgres` |
| **Client CLI** | `mariadb` / `mysql` | **`psql`** |
| **Sauvegarde logique** | `mysqldump` | **`pg_dump`** / `pg_restore` (+ `pg_dumpall` pour les rôles) |
| **Port par défaut** | 3306 | **5432** |
| **Récupération d'espace** | (géré par le moteur/purge) | **`VACUUM`** / autovacuum (dû au MVCC) |

> Ces différences sont matérialisées dans le code : placeholders `$N` et `RETURNING` dans  
> `internal/repository/`, mapping des `SQLSTATE` dans `internal/database/erreurs.go`, `CALL … INOUT`  
> dans `emprunt_repository.go`.

---

## `psql`, le client en ligne de commande

`psql` est le client interactif de PostgreSQL (l'équivalent du client `mysql`/`mariadb`). Dans ce
projet, on l'atteint via le conteneur :

```bash
# Superutilisateur (administration)
docker exec -it bibliotheque_postgres psql -U postgres -d bibliotheque

# Rôle applicatif (droits limités) — utile pour tester le moindre privilège
docker exec -it bibliotheque_postgres psql -U app_bibliotheque -d bibliotheque
```

### Méta-commandes essentielles (les « backslash »)

`psql` offre des raccourcis puissants, absents du client MySQL/MariaDB :

| Commande | Rôle |
|----------|------|
| `\l` (ou `\l+`) | Lister les **bases** de données |
| `\c bibliotheque` | Se **connecter** à une base |
| `\dn` | Lister les **schémas** |
| `\dt` (ou `\dt+`) | Lister les **tables** (avec tailles si `+`) |
| `\d livres` | **Décrire** une table : colonnes, types, index, contraintes, triggers |
| `\d+ livres` | Description **étendue** (commentaires, stockage) |
| `\dv` / `\dm` | Lister les **vues** / **vues matérialisées** |
| `\df` | Lister les **fonctions** ; `\df+ fn_calculer_penalite` pour le détail |
| `\df pr_*` | Lister les **procédures**/fonctions par motif |
| `\dT` | Lister les **types** (nos `ENUM`, `DOMAIN`) |
| `\di` | Lister les **index** |
| `\dx` | Lister les **extensions** installées |
| `\du` | Lister les **rôles** et leurs attributs |
| `\dp livres` | **Privilèges** (ACL) sur une table |
| `\sf fn_est_disponible` | Afficher le **code source** d'une fonction |
| `\x` | Bascule l'affichage **étendu** (une colonne par ligne) — pratique pour les lignes larges |
| `\timing` | Afficher le **temps** de chaque requête |
| `\i fichier.sql` | **Exécuter** un fichier SQL |
| `\watch 5` | Ré-exécuter la dernière requête toutes les 5 s |
| `\q` | Quitter |

### Exemples propres au projet

```sql
-- Vérifier les extensions activées
\dx

-- Détail de la table emprunts (types ENUM, contraintes CHECK, triggers, FK)
\d emprunts

-- Voir les tâches pg_cron planifiées
SELECT jobid, jobname, schedule, active FROM cron.job ORDER BY jobid;

-- Historique d'exécution des tâches pg_cron
SELECT jobid, status, return_message, start_time
FROM cron.job_run_details ORDER BY start_time DESC LIMIT 10;

-- Le code d'une procédure
\sf pr_emprunter_livre
```

### En une seule commande (mode non interactif)

```bash
docker exec -i bibliotheque_postgres \
  psql -U postgres -d bibliotheque -c "SELECT count(*) FROM livres;"

# Exécuter un script (par ex. une démo autonome)
docker exec -i bibliotheque_postgres \
  psql -U postgres -d bibliotheque -v ON_ERROR_STOP=1 -f - < sql/demos/01_types.sql
```

---

## Rôles et privilèges

PostgreSQL **unifie** « utilisateurs » et « groupes » sous la notion de **rôle**. Un rôle avec  
l'attribut `LOGIN` peut se connecter (c'est ce qu'on appelle un « utilisateur »).

Dans ce projet (`sql/schema/01_roles.sql`), on applique le **moindre privilège** :

```sql
-- Rôle applicatif dédié (jamais le superutilisateur pour l'application)
CREATE ROLE app_bibliotheque LOGIN PASSWORD '…';

-- Défense en profondeur : retirer les droits trop larges du pseudo-rôle PUBLIC
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON DATABASE bibliotheque FROM PUBLIC;

-- Le strict nécessaire
GRANT CONNECT ON DATABASE bibliotheque TO app_bibliotheque;
GRANT USAGE   ON SCHEMA public         TO app_bibliotheque;

-- LA clé de voûte : ALTER DEFAULT PRIVILEGES applique AUTOMATIQUEMENT ces droits
-- à tous les objets créés ENSUITE (on n'a pas à faire un GRANT après chaque CREATE).
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app_bibliotheque;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT                  ON SEQUENCES  TO app_bibliotheque;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT EXECUTE                        ON FUNCTIONS  TO app_bibliotheque;

-- Garde-fou par rôle : coupe toute requête qui dépasse 30 s
ALTER ROLE app_bibliotheque SET statement_timeout = '30s';
```

**Ce que ça change par rapport à MariaDB.** MariaDB raisonne en `utilisateur@hôte` et en `GRANT`
table par table. PostgreSQL raisonne en **rôles** (globaux), en **schémas**, et propose
**`ALTER DEFAULT PRIVILEGES`** pour appliquer des droits aux **futurs** objets — impossible aussi
simplement en MariaDB.

**Conséquence sécurité :** le rôle applicatif possède le CRUD (`SELECT/INSERT/UPDATE/DELETE`) mais  
**pas** `DROP`/`ALTER`/`TRUNCATE`, et **pas** `CREATE` sur `public`. Même en cas d'injection SQL,
un attaquant ne peut **pas détruire le schéma**. Voir [docs/SECURITE.md](docs/SECURITE.md).

Attributs de rôle utiles : `LOGIN`/`NOLOGIN`, `SUPERUSER`/`NOSUPERUSER`, `CREATEDB`, `CREATEROLE`,
`INHERIT`, `CONNECTION LIMIT n`, `VALID UNTIL '…'`. Inspectez-les avec `\du`.

---

## Schémas

Un **schéma** est un espace de noms **à l'intérieur d'une base** (comme un dossier). Il permet de  
regrouper et cloisonner des objets sans multiplier les bases.

- Le schéma **`public`** contient nos objets applicatifs (tables, vues, types, fonctions).
- L'extension `pg_cron` crée son propre schéma **`cron`** (table `cron.job`, `cron.job_run_details`).
- Les catalogues système vivent dans `pg_catalog` et `information_schema`.

Le **`search_path`** détermine dans quels schémas PostgreSQL cherche un objet non qualifié (par  
défaut `"$user", public`). On peut qualifier explicitement : `public.livres`, `cron.job`.

```sql
SHOW search_path;
\dn                       -- lister les schémas
SELECT * FROM cron.job;   -- objet qualifié par son schéma
```

> Analogie MariaDB : en MySQL/MariaDB, « database » et « schema » sont synonymes. En PostgreSQL, une  
> **base** contient plusieurs **schémas** : deux niveaux d'organisation, pas un.

---

## Extensions

Les **extensions** sont l'une des grandes forces de PostgreSQL : des modules qui ajoutent des  
types, fonctions, index ou mécanismes entiers, activables d'une commande. `CREATE EXTENSION IF NOT  
EXISTS` est **idempotent**.

| Extension | Fournit | Usage dans le projet |
|-----------|---------|----------------------|
| **`pgcrypto`** | `gen_random_uuid()`, `digest()`, `hmac()`, `crypt()`… | Génération des **UUID publics** (`DEFAULT gen_random_uuid()`) non devinables |
| **`pg_trgm`** | Similarité par **trigrammes** + classe d'opérateurs `gin_trgm_ops` | Index **GIN** rendant rapide `titre ILIKE '%terme%'` (impossible en B-tree) |
| **`uuid-ossp`** | `uuid_generate_v1()`, `uuid_generate_v4()`… | Activée à but pédagogique / compatibilité (on privilégie `gen_random_uuid()`) |
| **`pg_cron`** | Ordonnanceur de tâches SQL (`cron.schedule`) | **7 tâches** de maintenance (retards, purge, archivage, stats, `VACUUM`…) |

### `pg_trgm` : pourquoi c'est décisif

Un index **B-tree** classique ne peut pas accélérer une recherche avec **joker en tête**
(`'%mot%'`), car il indexe des préfixes ordonnés. `pg_trgm` découpe le texte en groupes de 3
caractères et permet un **index GIN** qui, lui, accélère ce motif :

```sql
CREATE INDEX idx_livres_titre_trgm ON livres USING gin (titre gin_trgm_ops);
-- Désormais rapide même sur de gros volumes :
SELECT titre FROM livres WHERE titre ILIKE '%dune%';
```

### `pg_cron` : une contrainte d'installation

Contrairement aux autres, `pg_cron` **doit être préchargée** au démarrage du serveur
(`shared_preload_libraries = 'pg_cron'`) et cibler une base (`cron.database_name = 'bibliotheque'`).
C'est pourquoi elle n'est **pas** activée avec les autres (`sql/extensions/00_extensions.sql`) mais  
dans `sql/cron/09_cron.sql`, et que le paramétrage passe par le `command:` du `docker-compose.yml`  
et une **image Docker sur mesure** (`docker/postgres/Dockerfile`). Détails :
[DOCKER.md](DOCKER.md).

```sql
\dx                                             -- vérifier les extensions
SELECT extname, extversion FROM pg_extension;   -- idem, en SQL
```

---

## Transactions et MVCC

### MVCC en deux mots

PostgreSQL gère la concurrence par **MVCC** (multi-version) : un `UPDATE` ne réécrit pas la ligne  
sur place, il crée une **nouvelle version** et marque l'ancienne comme périmée. Conséquences :

- **Les lecteurs ne bloquent pas les écrivains**, et réciproquement (excellente concurrence).
- Chaque transaction voit un **instantané** cohérent des données (isolation).
- Les anciennes versions (« lignes mortes », *dead tuples*) doivent être nettoyées ensuite : c'est
  le rôle de **`VACUUM`** (voir plus bas). C'est le prix du MVCC, et une notion **spécifique** que
  l'on ne retrouve pas sous cette forme en MariaDB/InnoDB.

### Niveaux d'isolation

`READ COMMITTED` (défaut), `REPEATABLE READ`, `SERIALIZABLE`. On peut les demander par transaction :

```sql
BEGIN ISOLATION LEVEL REPEATABLE READ;
  -- …
COMMIT;
```

### Verrouillage pessimiste : `SELECT … FOR UPDATE`

Quand deux transactions se disputent la **même ligne** (par ex. le dernier exemplaire d'un livre),  
on **sérialise** l'accès avec un verrou de ligne. C'est exactement ce que fait la procédure  
d'emprunt et la transaction de retour de ce projet :

```sql
SELECT id, exemplaires_disponibles
  FROM livres
  WHERE uuid = $1 AND supprime_le IS NULL
  FOR UPDATE;      -- la 2ᵉ transaction attend que la 1ʳᵉ termine
```

### Transactions côté application

Le helper `internal/database/transaction.go` encapsule `BEGIN`/`COMMIT`/`ROLLBACK` avec rollback  
automatique (y compris sur `panic`). Le retour d'un livre (`EmpruntRepository.Rendre`) l'utilise :  
verrou `FOR UPDATE`, calcul de la pénalité par fonction SQL, mise à jour de l'emprunt **et** du  
stock, le tout **atomique**. Détails et second cas (procédure) dans [DATABASE.md](DATABASE.md).

---

## `VACUUM` et `ANALYZE`

Deux opérations de maintenance essentielles, conséquence directe du MVCC et du planificateur.

### `VACUUM` — récupérer l'espace des lignes mortes

`VACUUM` recycle l'espace occupé par les versions de lignes périmées. Sans lui, les tables
« gonflent » (*bloat*).

```sql
VACUUM;                       -- toute la base, en douceur (non bloquant)
VACUUM (VERBOSE, ANALYZE) emprunts;   -- une table + stats, avec détails
VACUUM FULL livres;           -- réécrit la table et REND l'espace à l'OS (verrou EXCLUSIF : rare)
```

> `VACUUM` normal **ne rend pas** l'espace disque à l'OS (il le réutilise en interne). `VACUUM FULL`  
> le fait mais **verrouille** la table : à réserver à des fenêtres de maintenance.

### `ANALYZE` — actualiser les statistiques du planificateur

`ANALYZE` met à jour les statistiques (distribution des valeurs) que le **planificateur** utilise
pour choisir un plan (index vs parcours séquentiel…). Des stats à jour = de meilleurs plans.

```sql
ANALYZE;             -- toute la base
ANALYZE livres;      -- une table
```

### `autovacuum` — le fait pour vous

Un démon **`autovacuum`** exécute automatiquement `VACUUM` et `ANALYZE` en tâche de fond, selon  
l'activité des tables. On le laisse travailler ; on n'intervient à la main que pour des cas précis
(gros import, table très active). Ce projet ajoute aussi une tâche **`pg_cron`** hebdomadaire
`bib_maintenance` qui lance `VACUUM ANALYZE` le dimanche à 05h00 (`sql/cron/09_cron.sql`).

Surveiller le *bloat* et l'activité de vacuum :

```sql
SELECT relname, n_live_tup, n_dead_tup, last_autovacuum
FROM pg_stat_user_tables ORDER BY n_dead_tup DESC;
```

---

## `EXPLAIN` et `EXPLAIN ANALYZE`

`EXPLAIN` montre le **plan** que le planificateur compte suivre (sans exécuter). `EXPLAIN ANALYZE`
**exécute** réellement la requête et affiche les temps et volumes **réels** — l'outil n°1 pour
diagnostiquer une requête lente.

```sql
EXPLAIN                 SELECT * FROM livres WHERE titre ILIKE '%dune%';
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM livres WHERE titre ILIKE '%dune%';
```

> ⚠️ `EXPLAIN ANALYZE` **exécute** la requête : pour un `INSERT`/`UPDATE`/`DELETE`, enveloppez-le  
> dans une transaction annulée (`BEGIN; EXPLAIN ANALYZE …; ROLLBACK;`).

### Lire un plan

Un plan se lit **de l'intérieur vers l'extérieur** (les nœuds les plus indentés d'abord). Chaque  
nœud indique un **type d'accès** et des **estimations** :

```
Index Scan using idx_livres_titre_trgm on livres  (cost=0.28..8.30 rows=1 width=…) (actual time=0.05..0.06 rows=1 loops=1)
  Index Cond: (titre ~~* '%dune%'::text)
Planning Time: 0.20 ms
Execution Time: 0.10 ms
```

- **`cost=démarrage..total`** : coût **estimé** (unité arbitraire). Le premier chiffre = coût avant
  la 1ʳᵉ ligne, le second = coût total.
- **`rows`** : nombre de lignes **estimé**. Comparez-le au `rows` **réel** de la partie
  `actual time` : un gros écart signale des **statistiques périmées** (→ `ANALYZE`).
- **`actual time=démarrage..total`** et **`loops`** : mesures **réelles** (seulement avec `ANALYZE`).
- **`BUFFERS`** : blocs lus en cache (*hit*) vs sur disque (*read*).

Principaux **types de nœuds** :

| Nœud | Signification | Bon / mauvais signe |
|------|---------------|---------------------|
| **Seq Scan** | Parcours **séquentiel** de toute la table | Normal sur petite table ; suspect sur grosse table filtrée (index manquant ?) |
| **Index Scan** | Parcours via un index, puis lecture de la table | Bon pour une sélection ciblée |
| **Index Only Scan** | Tout est dans l'index (**index couvrant**) : pas de lecture de table | Idéal (voir `idx_livres_categorie_couvrant`) |
| **Bitmap Heap Scan** | Index → carte de blocs → lecture groupée | Efficace quand beaucoup de lignes matchent |
| **Nested Loop / Hash Join / Merge Join** | Stratégies de **jointure** | Le planificateur choisit selon les volumes |

### Exemple concret : l'intérêt de l'index trigram

```sql
-- Sans l'index GIN pg_trgm → Seq Scan (lent sur gros volume)
-- Avec idx_livres_titre_trgm → Bitmap Index Scan / Index Scan (rapide)
EXPLAIN ANALYZE SELECT titre FROM livres WHERE titre ILIKE '%prince%';
```

Comparez le plan **avant/après** création d'un index pour vérifier son utilité — un index inutile  
coûte en écriture et en espace. Voir [docs/PERFORMANCES.md](docs/PERFORMANCES.md) pour la palette  
d'index du projet (B-tree, GIN, BRIN, partiels, couvrants).

---

## Sauvegardes : `pg_dump` / `pg_restore`

`pg_dump` produit une sauvegarde **logique** (le SQL nécessaire pour recréer la base) — l'équivalent
de `mysqldump`. Deux formats principaux :

```bash
# 1) Format "plain" (SQL texte) — simple, se restaure avec psql. C'est ce que fait scripts/backup.sh.
docker exec bibliotheque_postgres \
  pg_dump -U postgres -d bibliotheque > sauvegarde.sql
#   → restauration :
docker exec -i bibliotheque_postgres \
  psql -U postgres -d bibliotheque < sauvegarde.sql

# 2) Format "custom" (-Fc) — compressé, sélectif, restauré par pg_restore (recommandé pour la prod)
docker exec bibliotheque_postgres \
  pg_dump -U postgres -Fc -d bibliotheque > sauvegarde.dump
#   → restauration (base propre) :
docker exec -i bibliotheque_postgres \
  pg_restore -U postgres -d bibliotheque --clean --if-exists < sauvegarde.dump
```

Options utiles de `pg_dump` : `-Fc` (format custom), `-Fd` (répertoire, parallélisable),
`--schema-only` / `--data-only`, `-t livres` (une table), `-n public` (un schéma), `-j 4`
(parallélisme avec `-Fd`).

Options utiles de `pg_restore` : `--clean --if-exists` (supprime avant de recréer), `-j 4`
(restauration parallèle), `-t` / `-n` (restauration sélective), `-L` (fichier de liste pour choisir
les objets).

> **Rôles globaux.** `pg_dump` sauvegarde **une base**, pas les rôles (globaux à l'instance). Pour  
> les inclure, utilisez **`pg_dumpall --roles-only`** (ou `pg_dumpall` complet). Dans ce projet, le  
> rôle `app_bibliotheque` est recréé par les scripts d'initialisation, donc une sauvegarde de la  
> base suffit en général.

Le script `scripts/backup.sh` encapsule un `pg_dump` compressé en `.sql.gz` ; `scripts/restore.sh`  
fait l'inverse. Sauvegarde du **volume Docker** (copie brute des fichiers) : voir
[DOCKER.md](DOCKER.md).

---

## Planifier des tâches : Event Scheduler, `pg_cron`, cron Linux, Kubernetes

PostgreSQL n'a **pas** d'ordonnanceur intégré (contrairement à l'**Event Scheduler** de MariaDB).  
Ce projet utilise **`pg_cron`**. Voici comment choisir selon le contexte :

| Solution | Où s'exécute la tâche | Forces | Limites | Quand la préférer |
|----------|-----------------------|--------|---------|-------------------|
| **Event Scheduler** (MariaDB) | **Dans** le serveur MariaDB | Intégré, aucune dépendance, au plus près des données | Propre à MariaDB ; désactivé par défaut | Sur MariaDB, pour de la maintenance de données |
| **`pg_cron`** (ce projet) | **Dans** PostgreSQL (schéma `cron`) | Simple, transactionnel, au plus près des tables ; historique (`cron.job_run_details`) | À **précharger** ; **une instance** (pas de coordination multi-nœuds native) | Maintenance **des données** sur une instance PostgreSQL (purge, archivage, agrégats, `VACUUM`) |
| **cron Linux** | Sur l'**hôte**/un conteneur | Universel, planifie **n'importe quel** programme | Hors base ; gère mal la haute dispo ; secrets à fournir | Tâches **système** ou scripts hors SGBD (rotation de logs, appels d'outils) |
| **Kubernetes `CronJob`** | Dans un **pod** éphémère | Distribué, scalable, observable, réessais/*backoff* | Nécessite un cluster K8s ; surcoût opérationnel | Environnements **conteneurisés/distribués**, tâches applicatives planifiées |

### `pg_cron` dans ce projet

Format de planification classique **cron à 5 champs** : `minute heure jour mois jour_semaine`.

```sql
-- Créer / lister / supprimer une tâche
SELECT cron.schedule('bib_marquer_retards', '0 1 * * *', $$
    UPDATE emprunts SET statut = 'en_retard'
    WHERE statut = 'en_cours' AND date_retour_prevue < CURRENT_DATE
$$);

SELECT jobid, jobname, schedule, active FROM cron.job;
SELECT cron.unschedule('bib_marquer_retards');
```

Les **7 tâches** (`sql/cron/09_cron.sql`) : `bib_marquer_retards` (retards, 01h00),
`bib_purger_jetons` (jetons expirés, chaque heure), `bib_archiver` (archivage > 1 an, 02h30),
`bib_statistiques` (agrégats du jour, 03h00), `bib_refresh_stats`
(`REFRESH MATERIALIZED VIEW CONCURRENTLY`, 03h15), `bib_nettoyer_audit` (rétention 90 j, dimanche
04h00), `bib_maintenance` (`VACUUM ANALYZE`, dimanche 05h00). Détail de chacune dans
[DATABASE.md](DATABASE.md).

> **En pratique.** Sur une seule instance PostgreSQL, `pg_cron` est le choix le plus simple et le  
> plus proche des données. Dès qu'on passe à **plusieurs répliques** ou qu'on veut orchestrer des  
> tâches **applicatives** (au-delà du SQL), on complète (ou remplace) par un `CronJob` Kubernetes  
> ou l'ordonnanceur Go du projet (`internal/scheduler/`), qui reste, lui, côté application.
