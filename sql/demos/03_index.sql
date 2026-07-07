-- =============================================================================
-- 03_index.sql — Les INDEX de PostgreSQL (B-Tree, Hash, GIN, GiST, BRIN…)
-- -----------------------------------------------------------------------------
-- OBJET : montrer, EXPLAIN à l'appui, les grandes familles d'index de PostgreSQL,
-- une richesse sans équivalent dans MariaDB. Pour chacune : comment la créer, une
-- PREUVE d'utilisation (EXPLAIN), ses avantages/inconvénients et QUAND la choisir.
--
--   B-Tree · Hash · GIN · GiST · BRIN · partiel · couvrant (INCLUDE) · multicolonne
--
-- MÉTHODE : on crée une table demo_ VOLUMINEUSE (200 000 lignes) — sur une petite
-- table, l'optimiseur préfère (à juste titre) un parcours séquentiel, et aucun
-- index ne « sert ». Le volume rend les démonstrations réalistes.
--
-- RAPPEL : un index ACCÉLÈRE les lectures ciblées mais COÛTE en écriture et en
-- espace disque. On n'indexe que ce qui est réellement interrogé.
--
-- Tous les objets sont préfixés demo_ et supprimés à la fin.
-- =============================================================================

\echo '========================================================================'
\echo '  03_index.sql — Familles d''index PostgreSQL'
\echo '========================================================================'

-- #############################################################################
-- JEU DE DONNÉES — 200 000 « événements » réalistes
-- -----------------------------------------------------------------------------
--   ordre_temps : entier croissant = fortement CORRÉLÉ à l'ordre physique (BRIN).
--   cree_le     : horodatage croissant (B-Tree de plage/tri).
--   categorie   : faible cardinalité, ~8 valeurs (Hash, couvrant, multicolonne).
--   statut      : 'actif' pour 5 % des lignes seulement (index PARTIEL).
--   reference   : haute cardinalité, égalité exacte (Hash).
--   etiquettes  : tableau de tags (GIN).
-- #############################################################################
\echo '\n--- Construction de la table demo (200 000 lignes) ---'

DROP TABLE IF EXISTS demo_evenements;
CREATE TABLE demo_evenements (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ordre_temps bigint      NOT NULL,
    cree_le     timestamptz NOT NULL,
    categorie   text        NOT NULL,
    statut      text        NOT NULL,
    montant     numeric(10,2) NOT NULL,
    reference   text        NOT NULL,
    etiquettes  text[]      NOT NULL
);

INSERT INTO demo_evenements (ordre_temps, cree_le, categorie, statut, montant, reference, etiquettes)
SELECT
    g,
    TIMESTAMPTZ '2020-01-01 00:00:00+00' + (g * INTERVAL '5 minutes'),
    (ARRAY['presse','sport','culture','economie','sciences','voyage','cuisine','tech'])[1 + (g % 8)],
    CASE WHEN g % 20 = 0 THEN 'actif' ELSE 'archive' END,
    (random() * 1000)::numeric(10,2),
    'REF-' || g,
    ARRAY[
        'tag' || (g % 50),
        'tag' || (g % 7)
    ]
FROM generate_series(1, 200000) AS g;

-- VACUUM + ANALYZE : indispensable pour (1) des statistiques à jour pour
-- l'optimiseur et (2) une carte de visibilité complète, sans laquelle un
-- « Index Only Scan » (index couvrant) ne peut pas se déclencher.
VACUUM (ANALYZE) demo_evenements;

-- Astuce d'affichage : on demande à EXPLAIN de rester lisible (sans coûts/timings).


-- #############################################################################
-- 1) B-TREE — l'index universel (défaut)
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : arbre équilibré trié. Gère =, <, <=, >, >=, BETWEEN, IN,
--   ORDER BY et LIKE 'prefixe%'. C'est l'index créé implicitement par PRIMARY KEY
--   et UNIQUE, et le seul type que propose vraiment MariaDB.
-- QUAND : cas général (égalité, plages, tri). 90 % des besoins.
-- INCONVÉNIENT : inutile pour « %motif% » (joker en tête) — voir GIN+pg_trgm.
-- #############################################################################
\echo '\n=== 1) B-TREE : plage + tri ==='

CREATE INDEX idx_demo_evt_cree_le ON demo_evenements (cree_le);

-- Requête de PLAGE + tri + LIMIT : le B-Tree fournit les lignes déjà triées.
\echo '>> EXPLAIN (attendu : Index Scan using idx_demo_evt_cree_le) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT id, cree_le
FROM demo_evenements
WHERE cree_le >= TIMESTAMPTZ '2020-06-01+00'
  AND cree_le <  TIMESTAMPTZ '2020-06-02+00'
ORDER BY cree_le
LIMIT 20;


-- #############################################################################
-- 2) INDEX MULTICOLONNE — plusieurs colonnes, dans l'ORDRE
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : trie sur (col1, col2, …). RÈGLE DU PRÉFIXE GAUCHE : l'index
--   sert les requêtes filtrant col1 (seule) ou col1 ET col2, mais PAS col2 seule.
-- QUAND : filtre récurrent combinant deux colonnes (ici « catégorie + montant »).
-- BONNE PRATIQUE : mettre en tête la colonne testée en ÉGALITÉ, puis celle testée
--   en plage/tri. Ici : categorie (=) puis montant (>=).
-- #############################################################################
\echo '\n=== 2) MULTICOLONNE (categorie, montant) ==='

CREATE INDEX idx_demo_evt_cat_montant ON demo_evenements (categorie, montant);

-- Filtre sur les DEUX colonnes : l'index les satisfait d'un seul tenant. Il se
-- POSITIONNE (seek) sur categorie = 'tech', puis balaie la plage montant. Très
-- sélectif : parcours ciblé de l'index multicolonne.
\echo '>> EXPLAIN categorie + montant (parcours ciblé de l''index multicolonne) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS ON)
SELECT id
FROM demo_evenements
WHERE categorie = 'tech'
  AND montant >= 995;

-- RÈGLE DU PRÉFIXE GAUCHE — nuance PostgreSQL importante :
-- Sans la 1re colonne (categorie), l'index ne peut pas SE POSITIONNER d'un seul
-- coup sur montant. Contrairement à MySQL/MariaDB (qui ignorerait l'index),
-- PostgreSQL 18 s'en sert quand même via une série de recherches (une par valeur
-- de categorie : « skip scan »). Comparez les lignes « Index Searches » et
-- « Buffers » : 1 seule recherche et 3 blocs pour le seek ciblé ci-dessus, contre
-- de MULTIPLES recherches et davantage de blocs ici. Le ciblage optimal est perdu.
-- Leçon : pour un accès EFFICACE, fournir la colonne de tête reste la bonne pratique.
\echo '>> EXPLAIN montant SEUL (pas de seek unique : voir « Index Searches ») :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS ON)
SELECT id
FROM demo_evenements
WHERE montant >= 999.5;

-- On retire cet index pour que les démonstrations suivantes restent SANS AMBIGUÏTÉ
-- (sinon il pourrait « voler la vedette » à l'index couvrant de la section 3).
DROP INDEX idx_demo_evt_cat_montant;


-- #############################################################################
-- 3) INDEX COUVRANT (INCLUDE) — viser l'« Index Only Scan »
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : on ajoute avec INCLUDE des colonnes NON clés, juste stockées
--   dans l'index. Si la requête ne lit QUE des colonnes présentes dans l'index,
--   PostgreSQL répond SANS ouvrir la table (« Index Only Scan ») — très rapide.
-- QUAND : requête chaude renvoyant peu de colonnes toujours les mêmes.
-- PRÉREQUIS : table VACUUMée (carte de visibilité à jour), déjà fait plus haut.
-- #############################################################################
\echo '\n=== 3) COUVRANT : (categorie) INCLUDE (montant) ==='

CREATE INDEX idx_demo_evt_couvrant
    ON demo_evenements (categorie) INCLUDE (montant);

\echo '>> EXPLAIN (attendu : Index Only Scan — pas d''accès à la table) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT categorie, sum(montant)
FROM demo_evenements
WHERE categorie = 'sport'
GROUP BY categorie;


-- #############################################################################
-- 4) INDEX PARTIEL — n'indexer qu'un SOUS-ENSEMBLE de lignes
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : une clause WHERE limite l'index à certaines lignes. Il est
--   donc PLUS PETIT et plus rapide. Le projet l'utilise (emprunts/utilisateurs actifs).
-- QUAND : on n'interroge presque toujours qu'une fraction des lignes (ici 5 % « actif »).
-- CONDITION : la requête doit contenir la même condition (statut = 'actif').
-- #############################################################################
\echo '\n=== 4) PARTIEL : WHERE statut = ''actif'' (5 % des lignes) ==='

CREATE INDEX idx_demo_evt_actifs
    ON demo_evenements (cree_le)
    WHERE statut = 'actif';

-- Un index partiel est bien plus PETIT qu'un index complet équivalent :
SELECT
    pg_size_pretty(pg_relation_size('idx_demo_evt_cree_le')) AS taille_index_complet,
    pg_size_pretty(pg_relation_size('idx_demo_evt_actifs'))  AS taille_index_partiel;

\echo '>> EXPLAIN (attendu : Index Scan using idx_demo_evt_actifs) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT id, cree_le
FROM demo_evenements
WHERE statut = 'actif'
  AND cree_le >= TIMESTAMPTZ '2021-01-01+00';


-- #############################################################################
-- 5) HASH — égalité STRICTE uniquement
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : indexe le HACHÉ de la valeur. Gère « = » seulement (jamais <,
--   >, ORDER BY, ni LIKE). Crash-safe et journalisé depuis PostgreSQL 10.
-- QUAND : égalité pure sur des valeurs longues/à haute cardinalité, sans besoin
--   d'ordre. En pratique un B-Tree fait aussi bien et PLUS (ordre, plages) : le
--   Hash reste un cas de niche. On le montre pour la complétude.
-- #############################################################################
\echo '\n=== 5) HASH : égalité exacte sur reference ==='

CREATE INDEX idx_demo_evt_ref_hash ON demo_evenements USING hash (reference);

\echo '>> EXPLAIN égalité (attendu : Index Scan using idx_demo_evt_ref_hash) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT id FROM demo_evenements WHERE reference = 'REF-123456';

-- IMPORTANT : un index Hash NE PEUT PAS servir une PLAGE ni un tri. La requête
-- ci-dessous (préfixe) l'ignore forcément — illustration de sa limite.
\echo '>> EXPLAIN plage sur reference (le Hash est INUTILISABLE ici) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT count(*) FROM demo_evenements WHERE reference LIKE 'REF-9%';


-- #############################################################################
-- 6) GIN — indexer des valeurs MULTIPLES par ligne (tableaux, jsonb, texte)
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : « index inversé » — pour chaque ÉLÉMENT (tag, clé jsonb, mot),
--   la liste des lignes qui le contiennent. Gère @>, &&, ? … et la recherche
--   plein texte (voir 06). Le projet s'en sert sur jsonb et via pg_trgm.
-- QUAND : colonnes tableau, jsonb, tsvector, ou recherche « %motif% » (pg_trgm).
-- INCONVÉNIENT : écriture plus lente (option fastupdate atténue), index volumineux.
-- #############################################################################
\echo '\n=== 6) GIN : tableau d''étiquettes (opérateur @>) ==='

CREATE INDEX idx_demo_evt_tags_gin ON demo_evenements USING gin (etiquettes);

\echo '>> EXPLAIN (attendu : Bitmap Index Scan sur le GIN) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT count(*) FROM demo_evenements WHERE etiquettes @> ARRAY['tag3'];


-- #############################################################################
-- 7) BRIN — index MINUSCULE pour données corrélées à l'ordre physique
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : ne stocke qu'un RÉSUMÉ (min/max) par plage de blocs. Efficace
--   UNIQUEMENT si la colonne est corrélée à l'ordre de stockage (append-only :
--   horodatage, id croissant). Le projet l'utilise sur journal_audit.cree_le.
-- FORCE : une taille dérisoire (des Ko là où un B-Tree ferait des Mo).
-- INCONVÉNIENT : inutile si les données ne sont pas corrélées (ordre aléatoire).
-- #############################################################################
\echo '\n=== 7) BRIN : colonne corrélée ordre_temps ==='

-- Pour COMPARER, on crée d'abord un B-Tree puis un BRIN sur la MÊME colonne.
CREATE INDEX idx_demo_evt_ordre_btree ON demo_evenements (ordre_temps);
CREATE INDEX idx_demo_evt_ordre_brin  ON demo_evenements USING brin (ordre_temps);

-- Le BRIN est SPECTACULAIREMENT plus petit que le B-Tree équivalent.
SELECT
    pg_size_pretty(pg_relation_size('idx_demo_evt_ordre_btree')) AS taille_btree,
    pg_size_pretty(pg_relation_size('idx_demo_evt_ordre_brin'))  AS taille_brin;

-- On retire le B-Tree pour PROUVER que le BRIN seul sert la plage.
DROP INDEX idx_demo_evt_ordre_btree;

\echo '>> EXPLAIN plage (attendu : Bitmap Index Scan sur le BRIN) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT count(*) FROM demo_evenements
WHERE ordre_temps BETWEEN 100000 AND 100500;


-- #############################################################################
-- 8) GiST — index GÉNÉRALISÉ (plages, géométrie, exclusion, KNN)
-- -----------------------------------------------------------------------------
-- FONCTIONNEMENT : arbre « équilibré généralisé » adapté aux données où l'on
--   cherche des CHEVAUCHEMENTS ou des PROXIMITÉS : types range (daterange…),
--   géométriques (point, box), tsvector, etc.
-- CAS D'USAGE PHARE : une CONTRAINTE D'EXCLUSION garantissant qu'aucune réservation
--   d'une même salle ne se CHEVAUCHE — impossible à exprimer avec un simple UNIQUE.
--
-- Combiner « égalité » (salle) ET « chevauchement » (période) dans une exclusion
-- nécessite l'extension btree_gist (elle apporte les classes d'opérateurs B-Tree
-- au sein d'un index GiST). C'est une facilité PARTAGÉE, idempotente et additive.
-- #############################################################################
\echo '\n=== 8) GiST : contrainte d''exclusion anti-chevauchement ==='

CREATE EXTENSION IF NOT EXISTS btree_gist;

DROP TABLE IF EXISTS demo_reservations;
CREATE TABLE demo_reservations (
    id     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    salle  text        NOT NULL,
    -- daterange : intervalle de dates. '[)' = borne basse incluse, haute exclue.
    periode daterange  NOT NULL,
    -- La contrainte d'exclusion s'appuie sur un index GiST : pour une MÊME salle
    -- (=), deux périodes qui se CHEVAUCHENT (&&) sont INTERDITES.
    EXCLUDE USING gist (salle WITH =, periode WITH &&)
);

-- Deux réservations compatibles (salles différentes, ou périodes disjointes) : OK.
INSERT INTO demo_reservations (salle, periode) VALUES
    ('A', daterange('2026-01-10', '2026-01-15')),
    ('A', daterange('2026-01-15', '2026-01-20')),   -- accolée mais sans chevauchement
    ('B', daterange('2026-01-12', '2026-01-18'));    -- autre salle : sans conflit

-- Tentative de CHEVAUCHEMENT sur la salle A : la contrainte GiST la REJETTE.
DO $$
BEGIN
    INSERT INTO demo_reservations (salle, periode)
    VALUES ('A', daterange('2026-01-14', '2026-01-16'));   -- chevauche 10-15
    RAISE NOTICE 'Inattendu : le chevauchement aurait dû être refusé.';
EXCEPTION
    WHEN exclusion_violation THEN
        RAISE NOTICE 'OK, chevauchement refusé par la contrainte GiST. Détail : %', SQLERRM;
END$$;

-- Requête de chevauchement (&&) accélérée par l'index GiST de la contrainte.
\echo '>> Réservations de la salle A chevauchant le 14 au 16 janvier :'
SELECT salle, periode
FROM demo_reservations
WHERE salle = 'A'
  AND periode && daterange('2026-01-14', '2026-01-16');


-- #############################################################################
-- SYNTHÈSE (mémo « quel index choisir ? »)
--   B-Tree      : =, plages, tri, préfixe. Le défaut universel.
--   Multicolonne: filtres combinés (respecter le préfixe gauche).
--   Couvrant    : Index Only Scan sur une requête chaude à colonnes fixes.
--   Partiel     : on n'interroge qu'un sous-ensemble stable de lignes.
--   Hash        : égalité pure (niche ; B-Tree fait souvent aussi bien).
--   GIN         : tableaux, jsonb, plein texte, « %motif% » (pg_trgm).
--   BRIN        : très grosses tables append-only corrélées (coût mémoire minime).
--   GiST        : plages/géométrie, contraintes d'exclusion, KNN.
-- #############################################################################

-- #############################################################################
-- NETTOYAGE (les index tombent avec leurs tables).
-- NB : on LAISSE l'extension btree_gist installée — la retirer pourrait affecter
--      d'autres objets ; elle est inoffensive et son ajout est idempotent.
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
DROP TABLE IF EXISTS demo_reservations;
DROP TABLE IF EXISTS demo_evenements;

\echo '\n03_index.sql : terminé sans erreur.'
