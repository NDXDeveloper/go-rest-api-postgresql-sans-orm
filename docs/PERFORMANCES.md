# PERFORMANCES.md — Performances et concurrence, expliquées

Ce document détaille les choix de **performance** et de **concurrence** du projet : le pool de  
connexions, les requêtes préparées (protocole étendu de PostgreSQL), la propagation du
`context.Context`, les timeouts, la gestion des goroutines, le modèle de concurrence **MVCC**, et les
conseils d'optimisation SQL (index, `EXPLAIN`, `VACUUM`). Chaque réglage est **justifié** : le but est  
de comprendre *pourquoi* il compte.

## Table des matières

- [1. Le pool de connexions](#1-le-pool-de-connexions)
- [2. Requêtes préparées côté serveur](#2-requêtes-préparées-côté-serveur)
- [3. `context.Context` : propagation et annulation](#3-contextcontext--propagation-et-annulation)
- [4. Timeouts en cascade](#4-timeouts-en-cascade)
- [5. Concurrence et goroutines](#5-concurrence-et-goroutines)
- [6. Optimisation SQL : index et `EXPLAIN`](#6-optimisation-sql--index-et-explain)
- [7. Autres bonnes pratiques de performance](#7-autres-bonnes-pratiques-de-performance)
- [Récapitulatif des réglages](#récapitulatif-des-réglages)

---

## 1. Le pool de connexions

**Le point clé à comprendre.** En Go, `*sql.DB` n'est **pas** une connexion : c'est un **pool de
connexions** géré automatiquement. On l'ouvre **une seule fois** au démarrage et on le partage
(par injection) dans toute l'application. Ouvrir/fermer une connexion à **chaque** requête serait
catastrophique — d'autant qu'en PostgreSQL, chaque connexion correspond à un **processus serveur**  
dédié (modèle *un processus par connexion*), ce qui rend le sur-approvisionnement encore plus coûteux.

> `sql.Open("pgx", …)` **n'établit pas** réellement de connexion : il prépare le pool et s'appuie sur  
> le pilote **pgx** (exposé comme un pilote `database/sql` classique via son sous-paquet `stdlib`).  
> C'est `PingContext` qui force une vraie connexion et permet de détecter tôt un problème (mauvais mot  
> de passe, base injoignable). Voir `internal/database/database.go`.

Les réglages du pool (`internal/database/database.go`, valeurs par `internal/config/config.go`) :

```go
db.SetMaxOpenConns(cfg.MaxConnexionsOuvertes)   // BDD_MAX_CONNEXIONS_OUVERTES, défaut 25
db.SetMaxIdleConns(cfg.MaxConnexionsInactives)  // BDD_MAX_CONNEXIONS_INACTIVES, défaut 25
db.SetConnMaxLifetime(cfg.DureeVieMaxConnexion) // BDD_DUREE_VIE_CONNEXION, défaut 5m
db.SetConnMaxIdleTime(cfg.DureeVieMaxConnexion) // même durée pour l'inactivité
```

| Réglage             | Rôle et justification                                                                 |
|---------------------|---------------------------------------------------------------------------------------|
| `SetMaxOpenConns`   | **Plafonne** le nombre de connexions simultanées vers PostgreSQL. **Trop haut** → on sature le serveur (`max_connections`, défaut **100**). **Trop bas** → goulet d'étranglement (les requêtes attendent). On l'aligne sur la capacité de la base. |
| `SetMaxIdleConns`   | Nombre de connexions gardées **ouvertes au repos** pour éviter le coût de réouverture. Le mettre **égal** à `MaxOpenConns` donne un pool réactif (une connexion libérée reste disponible). |
| `SetConnMaxLifetime`| **Recycle** les connexions périodiquement. Indispensable derrière un équilibreur de charge ou un pare-feu qui coupe les connexions longues, et pour éviter des connexions « zombies ». |
| `SetConnMaxIdleTime`| Recycle une connexion restée **inactive** trop longtemps (libère des ressources côté base). |

**Pourquoi `MaxIdleConns == MaxOpenConns` ?** Si `MaxIdleConns` était plus petit, une connexion
tout juste libérée après une requête serait **fermée** faute de place au repos, puis **rouverte** à  
la requête suivante — un gâchis. En les égalisant, une connexion active reste disponible pour la  
prochaine requête.

**Dimensionner le pool face à `max_connections`.** Côté serveur, PostgreSQL borne le nombre **total**
de connexions par le paramètre `max_connections` (**100** par défaut), dont quelques-unes sont  
réservées à l'administration (`superuser_reserved_connections`). Le pool applicatif doit rester
**sous** cette limite. Si l'on déploie *N* instances de l'API, chacune avec `MaxOpenConns = 25`, le
total `N × 25` doit tenir sous `max_connections` : avec 100 connexions disponibles, on fait tourner
~3 instances (3 × 25 = 75) tout en gardant une marge pour l'administration et les tâches `pg_cron`.
Au-delà, on augmente `max_connections` (au prix de plus de mémoire par connexion) ou l'on interpose  
un **pooler** externe comme PgBouncer.

**Surveillance en production.** Un ordonnanceur journalise périodiquement l'état du pool (voir
`cmd/api/main.go`), ce qui permet de détecter une saturation (`WaitCount` qui grimpe = requêtes en
attente d'une connexion) :

```go
stats := db.Stats()
logger.Info("état du pool de connexions",
    slog.Int("connexions_ouvertes", stats.OpenConnections),
    slog.Int("en_utilisation", stats.InUse),
    slog.Int("au_repos", stats.Idle),
    slog.Int64("en_attente", stats.WaitCount),
)
```

Si `en_attente` augmente durablement, il faut soit augmenter `MaxOpenConns` (si PostgreSQL tient  
sous `max_connections`), soit optimiser les requêtes lentes qui monopolisent les connexions.

---

## 2. Requêtes préparées côté serveur

**Ce que fait le pilote.** pgx dialogue avec PostgreSQL via le **protocole étendu** (*extended query
protocol*), en trois temps : **Parse** (le serveur analyse et planifie la requête), **Bind** (il y  
attache les valeurs des paramètres) puis **Execute** (il exécute). Les valeurs ne transitent donc
**jamais** mêlées au texte SQL : elles sont envoyées **séparément**, en binaire. Autre spécificité,
les emplacements de paramètres sont **numérotés** — `$1, $2, $3`… — et non anonymes.

```sql
-- Ce que les repositories envoient (voir internal/repository/) :
SELECT ... FROM vue_livres_details WHERE uuid = $1;
INSERT INTO livres (...) VALUES ($1, $2, ..., $11) RETURNING id;
```

**Un cache de statements côté connexion.** pgx mémorise les requêtes préparées **par connexion** :
lorsqu'une même requête (même texte SQL) revient sur la même connexion du pool, son plan analysé est
**réutilisé**, sans nouvelle phase *Parse*. Sur des requêtes répétées (lectures de liste, `INSERT`
en boucle…), cela évite un *reparse* systématique côté serveur.

**Double bénéfice :**

1. **Sécurité** : les valeurs ne peuvent jamais altérer la structure de la requête → neutralise
   l'injection SQL (voir [SECURITE.md](SECURITE.md)).
2. **Performance** : sur une requête répétée avec des valeurs différentes, le serveur réutilise le
   plan d'exécution, et le passage des paramètres en binaire est efficace.

**Construction du DSN.** La chaîne de connexion est assemblée avec `net/url` (qui échappe
correctement l'utilisateur et le mot de passe), sous forme d'URL `postgres://…` :

```go
// internal/database/database.go — extrait de construireDSN
u := url.URL{
    Scheme: "postgres",
    User:   url.UserPassword(cfg.Utilisateur, cfg.MotDePasse), // échappement sûr
    Host:   fmt.Sprintf("%s:%d", cfg.Hote, cfg.Port),          // ex. 127.0.0.1:5432
    Path:   cfg.Nom,
}
parametres.Set("sslmode", "disable")        // pas de TLS (réseau Docker privé ; « require » en prod)
parametres.Set("connect_timeout", "…")      // délai d'établissement de la connexion, en secondes
parametres.Set("timezone", "UTC")           // horodatages cohérents en UTC (serveur ET session)
```

**Appels de procédures à paramètres `INOUT`.** La procédure d'emprunt renvoie ses valeurs de sortie
via des paramètres `INOUT`. En PostgreSQL, `CALL pr_emprunter_livre($1, $2, $3, NULL, NULL, NULL)`
**renvoie directement une ligne** contenant ces valeurs (on passe `NULL` en entrée pour les `INOUT`) :
un simple `QueryRowContext(...).Scan(...)` **sur le pool** suffit, sans réserver de connexion dédiée  
ni passer par des variables de session liées à une seule connexion. C'est le point à retenir pour les  
procédures `INOUT` (voir `internal/repository/emprunt_repository.go`).

---

## 3. `context.Context` : propagation et annulation

**Le principe.** Chaque requête HTTP porte un `context.Context`. On le **propage** jusqu'aux appels
base de données (`...Context`). Si la requête est **annulée** (client déconnecté, délai dépassé), le  
contexte est annulé et les opérations qui l'observent **s'interrompent d'elles-mêmes**, libérant  
aussitôt les ressources (connexion, mémoire).

**En pratique, partout dans le code :**

```go
// Handlers  -> services -> repositories : r.Context() est transmis de bout en bout.
r.db.QueryRowContext(ctx, requete, uuid)     // et non QueryRow(...) sans contexte
r.db.ExecContext(ctx, requete, args...)
database.EnTransaction(ctx, r.db, ...)        // même une transaction est liée au contexte
```

**Pourquoi c'est crucial ?** Sans propagation du contexte, une requête SQL lente continuerait de
tourner côté base **même si** le client a abandonné ou si le délai a expiré : la connexion resterait  
mobilisée pour rien, réduisant la capacité disponible. Concrètement, si le contexte est annulé  
pendant une requête, pgx émet une **demande d'annulation** (*cancel request*) vers PostgreSQL pour  
interrompre la requête **en cours** côté serveur, et rend la connexion au pool.

> Règle d'or : **toujours** utiliser les variantes `...Context` et propager `r.Context()`. Sans  
> cela, le middleware de timeout (§4) serait sans effet sur les requêtes SQL (voir le commentaire  
> de `internal/middleware/timeout.go`).

**Sondes bornées.** La sonde `/ready` elle-même borne son `ping` à 2 s
(`context.WithTimeout(r.Context(), 2*time.Second)` dans `internal/handler/sante_handler.go`, qui
appelle `database.Verifier` → `PingContext`) : la sonde de disponibilité ne doit **jamais** rester  
bloquée.

---

## 4. Timeouts en cascade

Plusieurs délais se **complètent**, chacun protégeant une phase différente :

| Délai                                | Où                                   | Protège…                                          |
|--------------------------------------|--------------------------------------|---------------------------------------------------|
| `ReadHeaderTimeout` / `ReadTimeout`  | `http.Server` (`cmd/api/main.go`)    | La **lecture** de la requête (anti-Slowloris)     |
| `WriteTimeout`                       | `http.Server`                        | L'**écriture** de la réponse                       |
| `IdleTimeout`                        | `http.Server`                        | Les connexions keep-alive **inactives**           |
| `DelaiTraitement` (middleware)       | `middleware/timeout.go`              | La **durée de traitement** applicatif (contexte)  |
| `DelaiConnexion` (pool)              | `database/database.go`               | L'**établissement** d'une connexion (`connect_timeout`) |
| Ping `/ready` (2 s)                  | `handler/sante_handler.go`           | La sonde de disponibilité                          |

**Cohérence importante.** Le délai de traitement doit rester **inférieur** au `WriteTimeout` du
serveur, pour que le **contexte expire AVANT** que le serveur ne coupe l'écriture de la réponse (on  
préfère renvoyer une erreur propre plutôt qu'une connexion coupée). C'est documenté dans
`internal/config/config.go` (défaut : traitement 10 s < écriture 15 s).

---

## 5. Concurrence et goroutines

Go rend la concurrence naturelle. Le projet en illustre les fondamentaux **proprement**, avec un  
arrêt maîtrisé (pas de goroutine « fuyarde »).

### Le serveur HTTP

`net/http` traite **chaque requête dans sa propre goroutine**. C'est pourquoi le code partagé doit
être **sans état mutable global** : ici, tout est injecté (pool, config, logger…), et les seules
variables au niveau package sont des **constantes immuables** (ex. les `regexp` compilés une fois  
dans `internal/validation/validation.go`, sûrs pour un usage concurrent).

### Le limiteur de débit

`internal/middleware/rate_limiter.go` maintient une `map[string]*clientLimite` (un seau par IP),
protégée par un **`sync.Mutex`** car des requêtes concurrentes y accèdent :

```go
l.mu.Lock()
defer l.mu.Unlock()
// … accès à la map partagée …
```

Une **goroutine de nettoyage** (lancée au démarrage) purge les IP inactives toutes les minutes et  
s'**arrête proprement** quand le `context.Context` global est annulé (arrêt du serveur) :

```go
select {
case <-ctx.Done():
    return                 // arrêt propre, pas de fuite de goroutine
case <-ticker.C:
    // … purge des clients inactifs …
}
```

### L'ordonnanceur de tâches

`internal/scheduler/scheduler.go` illustre le motif complet : **une goroutine par tâche**, un
`time.Ticker` pour la périodicité, un `context.Context` pour l'arrêt, et un **`sync.WaitGroup`** pour
**attendre** la fin de toutes les goroutines lors de l'arrêt gracieux :

```go
func (o *Ordonnanceur) Demarrer(ctx context.Context) {
    for _, tache := range o.taches {
        o.wg.Add(1)
        go o.boucleTache(ctx, tache)   // une goroutine par tâche
    }
}
func (o *Ordonnanceur) Attendre() { o.wg.Wait() }   // arrêt propre
```

À l'arrêt (`cmd/api/main.go`), le contexte est annulé, puis `ordonnanceur.Attendre()` bloque jusqu'à
ce que toutes les tâches soient sorties — aucune goroutine n'est laissée en suspens.

### `-race` : détecter les accès concurrents

Les tests se lancent avec le **détecteur de data races** (`go test -race`, cible `make tester`) : il  
signale tout accès concurrent non synchronisé à une même donnée. C'est le filet de sécurité  
indispensable de tout code concurrent.

### La concurrence côté base : MVCC

La concurrence ne s'arrête pas au code Go : **PostgreSQL** gère lui aussi des accès simultanés, via  
le **MVCC** (*Multi-Version Concurrency Control*). Le principe : plutôt que de verrouiller une ligne  
pour la lire, PostgreSQL conserve **plusieurs versions** d'une même ligne ; chaque transaction voit  
un **instantané** cohérent des données à son instant de départ.

Conséquence directe et fondamentale : **les lecteurs ne bloquent pas les écrivains, et les écrivains  
ne bloquent pas les lecteurs**. Un `SELECT` n'attend jamais qu'un `UPDATE` concurrent se termine — il  
lit simplement la version précédente de la ligne. Seuls deux **écrivains** visant la **même** ligne  
s'attendent l'un l'autre.

C'est pourquoi la transaction de retour de livre (`Rendre`, dans `emprunt_repository.go`) prend un
**verrou explicite** là où c'est nécessaire :

```go
// SELECT ... FOR UPDATE : verrouille la ligne d'emprunt le temps de la transaction, pour
// empêcher deux retours simultanés du même emprunt (voir emprunt_repository.go).
`SELECT id, livre_id, statut::text, date_retour_prevue FROM emprunts WHERE uuid = $1 FOR UPDATE`
```

`FOR UPDATE` sérialise **uniquement** les transactions qui veulent modifier cette ligne précise ; les
lectures simples de l'emprunt, elles, continuent sans attendre.

**La contrepartie du MVCC.** Comme un `UPDATE` ou un `DELETE` ne réécrit pas la ligne sur place mais
crée une **nouvelle version** et marque l'ancienne comme obsolète, la table accumule des **« lignes  
mortes »** (*dead tuples*) : des versions devenues invisibles mais encore présentes sur le disque. Il  
faut les recycler — c'est le rôle de `VACUUM` (voir §6).

### Tâches Go vs tâches `pg_cron`

Deux mécanismes de planification, chacun à sa place (voir aussi [DATABASE.md](../DATABASE.md)) :

- **Tâches `pg_cron`** : maintenance des **données**, au plus près des tables, sans dépendre de
  l'application — marquer les retards, purger les jetons expirés, archiver, calculer les
  statistiques, et la maintenance `VACUUM ANALYZE` (voir `sql/cron/09_cron.sql`).
- **Ordonnanceur Go** : tâches **applicatives** (ici, journaliser l'état du pool de connexions).

---

## 6. Optimisation SQL : index et `EXPLAIN`

**Un index accélère les lectures, ralentit un peu les écritures.** Il évite de parcourir toute la
table pour un `WHERE`, un `JOIN` ou un `ORDER BY`, mais il faut le **maintenir** à chaque écriture et  
il occupe de l'espace. On indexe donc les colonnes **réellement** filtrées ou triées — et l'on  
choisit la **bonne méthode** : là où beaucoup de moteurs se limitent au B-tree, PostgreSQL offre une
**palette** de méthodes, chacune adaptée à un usage (voir `sql/schema/03_tables.sql` et
`sql/schema/04_index.sql`).

### La palette d'index du projet

| Méthode                     | Quand l'utiliser                                            | Exemple réel dans le projet                                   |
|-----------------------------|------------------------------------------------------------|---------------------------------------------------------------|
| **B-tree** (défaut)         | égalité, plages, tri (`=`, `<`, `>`, `ORDER BY`)           | `idx_livres_categorie` (`categorie_id`), `idx_jetons_expire` (`expire_le`) |
| **GIN + `pg_trgm`**         | recherche `ILIKE '%mot%'` (joker en tête, floue)          | `idx_livres_titre_trgm` (`titre`), `idx_auteurs_nom_trgm` (`nom`) |
| **GIN sur JSONB**           | recherche *dans* un document JSON (opérateur `@>`)        | `idx_audit_nouvelles_gin` (`journal_audit.nouvelles_valeurs`) |
| **BRIN**                    | grosse table *append-only*, colonne corrélée à l'ordre physique | `idx_audit_cree_le_brin` (`journal_audit.cree_le`)      |
| **Partiel** (`WHERE …`)     | n'indexer qu'un **sous-ensemble** de lignes                | `idx_utilisateurs_actifs … WHERE supprime_le IS NULL`         |
| **Couvrant** (`INCLUDE`)    | *index-only scan* (répondre sans lire la table)            | `idx_livres_categorie_couvrant … INCLUDE (titre, prix)`       |
| **Multicolonne**            | filtrer/trier sur plusieurs colonnes à la fois             | `idx_emprunts_util_statut (utilisateur_id, statut)`           |

Quelques points saillants :

- **GIN + `pg_trgm` — la star du projet.** Elle rend rapide `titre ILIKE '%terme%'`, la recherche
  exposée par l'API. `pg_trgm` décompose le texte en **trigrammes** (groupes de 3 caractères) et sait
  donc répondre à une recherche « au milieu » du texte — ce qu'un B-tree ne peut **pas** faire à
  cause du joker en tête.
- **GIN sur JSONB.** `idx_audit_nouvelles_gin` permet des recherches **dans** le document d'audit, par
  ex. `WHERE nouvelles_valeurs @> '{"role":"admin"}'`.
- **BRIN — l'index minuscule.** `journal_audit` ne fait que grandir dans l'ordre du temps : `cree_le`
  est donc fortement corrélé à l'ordre physique des blocs. BRIN résume chaque **plage de blocs** et
  occupe une place dérisoire — idéal pour une très grosse table *append-only*, là où un B-tree serait
  énorme.
- **Partiel + couvrant.** `idx_livres_categorie_couvrant` combine les deux : il n'indexe que les
  livres actifs (`WHERE supprime_le IS NULL`) et **inclut** `titre` et `prix` (`INCLUDE`), de sorte
  que « lister les livres d'une catégorie avec leur titre et leur prix » se résout par un
  **index-only scan**, sans toucher la table.

> **Ordre des colonnes d'un index multicolonne.** `(a, b)` sert les requêtes filtrant sur `a` seul,  
> ou sur `a` **et** `b`, mais **pas** sur `b` seul. On place en tête la colonne la plus souvent  
> filtrée. Ainsi `idx_emprunts_util_statut (utilisateur_id, statut)` sert « les emprunts d'un membre »  
> **et** « les emprunts d'un membre par statut ».

**Le compromis lecture/écriture.** Chaque index supplémentaire doit être mis à jour à chaque
`INSERT`/`UPDATE`/`DELETE` et consomme de l'espace disque. On n'indexe donc que ce qui sert
réellement les requêtes du projet — jamais « au cas où ».

### Diagnostiquer avec `EXPLAIN` et `EXPLAIN ANALYZE`

`EXPLAIN` affiche le **plan d'exécution** estimé par le planificateur, **sans** exécuter la requête.
`EXPLAIN (ANALYZE, BUFFERS)` l'exécute **réellement** et rapporte les temps et le nombre de lignes
mesurés (attention : un `EXPLAIN ANALYZE` sur un `UPDATE`/`INSERT`/`DELETE` **modifie** bel et bien  
les données !).

```sql
-- Plan estimé (n'exécute pas la requête) :
EXPLAIN SELECT * FROM vue_livres_details WHERE titre ILIKE '%tolstoï%';

-- Plan réel, avec temps mesurés et accès aux blocs (buffers) :
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM vue_livres_details WHERE titre ILIKE '%tolstoï%';
```

**Lire un plan PostgreSQL.** Les nœuds d'accès à connaître, du moins bon au meilleur pour un filtre
sélectif :

| Nœud                  | Signification                                                                          |
|-----------------------|----------------------------------------------------------------------------------------|
| **Seq Scan**          | parcours **séquentiel** de toute la table — aucun index utilisé (acceptable sur une petite table, à éviter sur une grosse) |
| **Bitmap Heap Scan**  | l'index désigne d'abord les blocs à lire, puis la table est lue par paquets (filtre moyennement sélectif) |
| **Index Scan**        | parcours de l'index puis accès ciblé à la table (filtre sélectif)                       |
| **Index Only Scan**   | tout est lu **dans l'index** (index couvrant), sans toucher la table — le plus rapide   |

Chaque nœud affiche des chiffres à interpréter :

- `cost=0.00..12.34` : coût **estimé** (unité arbitraire du planificateur), sous la forme
  *démarrage..total* ;
- `rows=…` : nombre de lignes **estimé** ;
- avec `ANALYZE`, `actual time=…` et `rows=…` **réels**, ainsi que le nombre de `loops` (boucles).

**L'écart estimé/réel est le signal clé.** Si le planificateur estime `rows=1` mais en trouve
`rows=50000`, ses **statistiques** sont périmées → un `ANALYZE` s'impose (voir plus bas).

**Exemple concret sur ce projet.** La recherche `titre ILIKE '%…%'` (endpoint de liste des livres) ne
peut **pas** utiliser un index B-tree, à cause du joker **en tête**. Sans index adapté, on obtient un
`Seq Scan`. Avec l'index GIN trigramme `idx_livres_titre_trgm`, le plan bascule sur un
`Bitmap Index Scan` suivi d'un `Bitmap Heap Scan` :

```text
Bitmap Heap Scan on livres  (cost=…  rows=…)  (actual time=…  rows=…)
  Recheck Cond: (titre ~~* '%tolstoï%'::text)
  ->  Bitmap Index Scan on idx_livres_titre_trgm  (cost=…)  (actual time=…  rows=…)
        Index Cond: (titre ~~* '%tolstoï%'::text)
```

Le passage de `Seq Scan` à `Bitmap Index Scan on idx_livres_titre_trgm` est la **preuve** que l'index  
trigramme est utilisé (`~~*` est l'opérateur d'`ILIKE` dans les plans) — exactement ce que promettait
`sql/schema/04_index.sql`, et **sans changer une ligne** de la requête applicative.

### `VACUUM`, `ANALYZE` et autovacuum

On l'a vu (§5, MVCC) : les `UPDATE`/`DELETE` laissent des **lignes mortes**. Deux opérations de  
maintenance les prennent en charge — et PostgreSQL les déclenche **automatiquement**.

- **`VACUUM`** récupère l'espace occupé par les lignes mortes pour le **réutiliser** (il ne rend pas
  forcément le disque au système d'exploitation ; `VACUUM FULL`, verrouillant et coûteux, le fait
  mais n'est pas nécessaire ici).
- **`ANALYZE`** met à jour les **statistiques** dont le planificateur se sert pour choisir ses plans
  (taille des tables, distribution des valeurs). Des statistiques fraîches = de meilleurs arbitrages
  `Seq Scan` vs `Index Scan`.
- **L'autovacuum** est un processus d'arrière-plan qui déclenche `VACUUM` **et** `ANALYZE` tout seul,
  dès qu'une table a suffisamment changé. **Dans la grande majorité des cas, on le laisse
  travailler** : il suffit à maintenir la base en bonne santé.

**Un filet de sécurité hebdomadaire.** En complément de l'autovacuum, une tâche `pg_cron`
`bib_maintenance` lance un `VACUUM ANALYZE` global chaque **dimanche à 05h00**
(`sql/cron/09_cron.sql`) :

```sql
SELECT cron.schedule('bib_maintenance', '0 5 * * 0', $$
    VACUUM ANALYZE
$$);
```

**Surveiller le *bloat*.** Si une table accumule trop de lignes mortes (le *bloat*), ses parcours
ralentissent. Pour inspecter en détail ce que fait le nettoyage sur une table :

```sql
VACUUM (VERBOSE, ANALYZE) livres;
```

La sortie indique le nombre de lignes mortes retirées et les blocs récupérés. En production, on  
surveille le ratio de lignes mortes (via `pg_stat_user_tables.n_dead_tup`) pour repérer une table qui
« gonfle » et, au besoin, régler l'autovacuum plus agressivement sur celle-ci. Pour approfondir
`EXPLAIN`, les méthodes d'index et `VACUUM`, voir [DATABASE.md](../DATABASE.md) et
[POSTGRESQL.md](../POSTGRESQL.md).

### Autres leviers SQL du projet

- **Vue pré-jointe** : `vue_livres_details` joint auteurs/catégories et calcule la disponibilité
  **une fois**, ce qui évite au client des allers-retours et centralise la logique de lecture.
- **Vue matérialisée** : `vue_statistiques_livres` **pré-calcule** la popularité des livres (agrégat
  stocké sur disque), rafraîchie hors ligne par `pg_cron` — une lecture instantanée là où l'agrégat
  serait coûteux à recalculer à chaque appel (voir `sql/views/06_vues.sql`).
- **`RETURNING`** : `INSERT INTO livres (…) VALUES (…) RETURNING id` récupère l'identifiant généré
  **dans la même requête**, sans second aller-retour vers la base (voir `Creer`).
- **`COUNT` + `SELECT` paginé** : les listes exécutent un `COUNT(*)` (pour la pagination) puis un
  `SELECT … LIMIT $N OFFSET $M`, en **court-circuitant** le `SELECT` si le total est 0 (voir les
  repositories).
- **Pagination bornée** : `taille` est plafonnée à **100** (`models.TailleMax`), pour éviter qu'un
  client ne demande des millions de lignes d'un coup (protection mémoire **et** performance).

---

## 7. Autres bonnes pratiques de performance

- **Pré-allocation des tranches** : les repositories créent les slices résultats avec une capacité
  initiale (`make([]models.X, 0, params.Taille)`) pour limiter les réallocations pendant le parcours
  des lignes.
- **Fermeture systématique des curseurs** : `defer lignes.Close()` après chaque `QueryContext`, et
  vérification de `lignes.Err()` après la boucle (une erreur d'itération ne se voit pas dans
  `Next()`).
- **Choisir la bonne méthode d'index** (voir §6) : B-tree par défaut, GIN + `pg_trgm` pour
  `ILIKE '%…%'`, BRIN pour une grosse table *append-only*. Un mauvais type d'index laisse un
  `Seq Scan` persister **malgré** l'index.
- **Mesurer avant/après avec `EXPLAIN (ANALYZE)`** : on ne **devine** pas qu'un index sert — on le
  **prouve** en comparant le plan avant et après sa création.
- **Laisser l'autovacuum travailler** : le désactiver conduit au *bloat* et à des statistiques
  périmées (donc à de mauvais plans). On l'**ajuste** plutôt qu'on ne le coupe.
- **Rafraîchir la vue matérialisée sans blocage** : `pg_cron` exécute
  `REFRESH MATERIALIZED VIEW CONCURRENTLY vue_statistiques_livres` — les lectures ne sont **jamais**
  bloquées pendant le rafraîchissement (au prix d'un index unique requis, voir `sql/views/06_vues.sql`).
- **Image Docker minimale** : binaire statique compilé avec `-ldflags "-s -w"` (table des symboles
  retirée), image finale ~20 Mo → **démarrage rapide** et déploiements légers (voir `Dockerfile`).
- **Métriques de latence** : l'histogramme `bibliotheque_http_duree_requete_secondes` (voir
  `internal/observabilite/metriques.go`) permet de suivre les quantiles (p50, p95, p99) et de
  repérer les endpoints lents. Le label `route` utilise le **patron** (`/livres/{id}`), pas le
  chemin réel, pour éviter l'explosion de cardinalité (une série par identifiant).
- **`ON DELETE CASCADE` / `RESTRICT` ciblés** : l'intégrité est gérée par la base (pas de multiples
  requêtes applicatives pour nettoyer les dépendances).

---

## Récapitulatif des réglages

| Réglage                        | Variable / lieu                       | Défaut     | Effet                                          |
|--------------------------------|---------------------------------------|------------|------------------------------------------------|
| Connexions max du pool         | `BDD_MAX_CONNEXIONS_OUVERTES`         | `25`       | Plafond de connexions simultanées à PostgreSQL (à garder sous `max_connections`, défaut 100) |
| Connexions au repos            | `BDD_MAX_CONNEXIONS_INACTIVES`        | `25`       | Connexions gardées prêtes                       |
| Durée de vie d'une connexion   | `BDD_DUREE_VIE_CONNEXION`             | `5m`       | Recyclage périodique                            |
| Délai de connexion             | `BDD_DELAI_CONNEXION`                 | `5s`       | Timeout d'établissement (`connect_timeout`)     |
| Délai de traitement            | `SERVEUR_DELAI_TRAITEMENT`            | `10s`      | Annulation du contexte au-delà                  |
| Lecture / écriture serveur     | `SERVEUR_DELAI_LECTURE` / `_ECRITURE` | `10s`/`15s`| Anti-Slowloris / borne l'écriture               |
| Keep-alive                     | `SERVEUR_DELAI_INACTIF`               | `60s`      | Ferme les connexions inactives                  |
| Débit par IP                   | `RATE_LIMIT_PAR_SECONDE` / `_RAFALE`  | `10`/`20`  | Limite le volume par client                     |
| Taille max du corps            | `REQUETE_TAILLE_MAX_OCTETS`           | `1 Mio`    | Protège la mémoire                              |
| Taille de page max             | `models.TailleMax`                    | `100`      | Borne le volume renvoyé par liste               |

Ces valeurs sont des **points de départ raisonnables**. En production, on les ajuste à partir des
**mesures** (métriques Prometheus, `db.Stats()`, `EXPLAIN (ANALYZE)` et `pg_stat_statements` sur les
requêtes lentes) : on optimise ce que l'on **mesure**, jamais à l'aveugle.
