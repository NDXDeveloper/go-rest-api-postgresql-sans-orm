# Démonstrations SQL PostgreSQL (`sql/demos/`)

Ce dossier rassemble des **démonstrations pédagogiques autonomes** qui mettent en  
valeur les **forces propres à PostgreSQL** exigées par le cahier des charges du  
projet. Chaque fichier est un **cours pratique** : abondamment commenté (ce que  
fait le code, *pourquoi*, les bonnes pratiques et les erreurs à éviter), il  
s'exécute seul et illustre une famille de fonctionnalités.

> Ces scripts sont **indépendants de l'API et du schéma applicatif**. Ils  
> **lisent** les tables existantes (le seed « bibliothèque ») et/ou créent leurs  
> **propres objets temporaires** — tous préfixés `demo_` — qu'ils **suppriment à  
> la fin**. Les rares écritures se font sur ces objets `demo_` ou dans une  
> transaction annulée (`ROLLBACK`). **Aucune donnée du seed n'est modifiée.**

## Contenu

| Fichier | Sujet | Notions clés |
|---|---|---|
| [`01_types.sql`](01_types.sql) | **Types de données** | UUID · JSON vs JSONB · ARRAY · ENUM · DOMAIN · BYTEA · `timestamptz` vs `timestamp` · INTERVAL · NUMERIC vs float |
| [`02_jsonb.sql`](02_jsonb.sql) | **JSONB** | lecture (`->`, `->>`, `#>`, `#>>`) · écriture (`jsonb_set`, `\|\|`, `-`, `#-`) · recherche (`@>`, `?`, `?\|`, `?&`, jsonpath) · agrégation · index **GIN** (+ EXPLAIN) · cas réel `journal_audit` |
| [`03_index.sql`](03_index.sql) | **Index** | **B-Tree · Hash · GIN · GiST · BRIN** · partiel · couvrant (`INCLUDE`) · multicolonne — avec `EXPLAIN (ANALYZE)` et comparaisons de taille |
| [`04_sql_avance.sql`](04_sql_avance.sql) | **SQL avancé** | CTE · **CTE récursive** · `RETURNING` · **UPSERT** (`ON CONFLICT`) · `DISTINCT ON` · **fonctions fenêtre** · `FILTER` · `GROUPING SETS`/`ROLLUP`/`CUBE` · `LATERAL` · `EXISTS`/`ANY`/`ALL` |
| [`05_transactions.sql`](05_transactions.sql) | **Transactions** | `BEGIN`/`COMMIT`/`ROLLBACK` · `SAVEPOINT` · niveaux d'**isolation** et anomalies · `FOR UPDATE`/`FOR SHARE`/`SKIP LOCKED` · **MVCC** · concurrence (scénarios à deux sessions) |
| [`06_recherche_plein_texte.sql`](06_recherche_plein_texte.sql) | **Recherche plein texte** | `tsvector`/`tsquery` · `to_tsvector('french', …)` · `plainto_`/`phraseto_`/`websearch_to_tsquery` · colonne tsvector **générée** + index **GIN** · `ts_rank` (pertinence) · `ts_headline` (surlignage) · pondération · `unaccent` |
| [`07_plpgsql.sql`](07_plpgsql.sql) | **PL/pgSQL** | variables · `IF`/`CASE` · boucles `FOR`/`WHILE`/`LOOP` · **curseur** explicite · **exceptions** + `RAISE` · `RETURNS TABLE`/`SETOF` · type composite/`RECORD` · procédure (`CALL`) |

Ces démonstrations **complètent** les objets réellement utilisés par l'API
(voir `sql/schema/`, `sql/functions/`, `sql/views/`, `sql/triggers/`). La
planification de tâches (**pg_cron**) est traitée à part, dans
[`sql/cron/`](../cron/), car elle requiert une configuration au démarrage du
serveur.

## Comment les lancer

Chaque fichier est **rejouable** et doit s'exécuter **sans aucune erreur**. Le  
drapeau `-v ON_ERROR_STOP=1` interrompt (et signale) la moindre erreur : c'est le  
test de validation.

### Option A — conteneur de test dédié (`pg-test`)

```bash
# Un fichier précis :
docker exec -i pg-test psql -U postgres -d bibliotheque \
    -v ON_ERROR_STOP=1 -f - < sql/demos/02_jsonb.sql

# Toutes les démos, dans l'ordre :
for f in sql/demos/0*.sql; do
    echo "=== $f ==="
    docker exec -i pg-test psql -U postgres -d bibliotheque \
        -v ON_ERROR_STOP=1 -f - < "$f" || break
done
```

### Option B — pile Docker Compose du projet

Le service PostgreSQL s'appelle `postgres` (conteneur `bibliotheque_postgres`) :

```bash
# La pile doit être démarrée : docker compose up -d
docker compose exec -T postgres psql -U postgres -d bibliotheque \
    -v ON_ERROR_STOP=1 -f - < sql/demos/03_index.sql
```

### Option C — session interactive

Pour lire les résultats pas à pas (les `\echo` structurent la sortie) :

```bash
docker exec -it pg-test psql -U postgres -d bibliotheque
-- puis, dans psql :
\i sql/demos/04_sql_avance.sql
```

## Bonnes pratiques illustrées

- **Autonomie & idempotence** : chaque script crée puis **supprime** ses objets
  `demo_*` (`DROP … IF EXISTS`) ; on peut le rejouer autant de fois qu'on veut.
- **Non-destructivité** : les écritures sur les tables du seed sont faites en
  transaction `ROLLBACK`, ou remplacées par des tables `demo_`.
- **Preuves à l'appui** : les démonstrations d'index et de JSONB utilisent
  `EXPLAIN (ANALYZE)` sur un **volume réaliste** (dizaines/centaines de milliers de
  lignes) — sur une petite table, l'optimiseur préfère à juste titre un parcours
  séquentiel et aucun index ne « sert ».

## Note sur les extensions

Deux extensions **contrib** standard sont activées (idempotemment) par les démos  
et **laissées installées** (inoffensives, elles ne modifient aucune donnée) :

- `btree_gist` — utilisée par `03_index.sql` pour une **contrainte d'exclusion**
  GiST (anti-chevauchement de réservations) ;
- `unaccent` — utilisée par `06_recherche_plein_texte.sql` pour une recherche
  **insensible aux accents**.

Les extensions du projet lui-même (`pgcrypto`, `pg_trgm`, `uuid-ossp`) sont, elles,  
activées par [`sql/extensions/`](../extensions/).
