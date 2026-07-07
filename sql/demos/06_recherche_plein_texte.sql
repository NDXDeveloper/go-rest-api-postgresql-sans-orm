-- =============================================================================
-- 06_recherche_plein_texte.sql — RECHERCHE PLEIN TEXTE (Full Text Search)
-- -----------------------------------------------------------------------------
-- OBJET : la recherche plein texte NATIVE de PostgreSQL, bien plus riche que le
-- FULLTEXT de MariaDB : lemmatisation par langue, pondération, classement par
-- pertinence, surlignage. On l'applique aux TITRES et RÉSUMÉS des livres.
--
--   tsvector · tsquery · to_tsvector('french', …) · to_tsquery / plainto_tsquery /
--   phraseto_tsquery / websearch_to_tsquery · colonne tsvector GÉNÉRÉE + index GIN
--   (avec EXPLAIN) · ts_rank (pertinence) · ts_headline (surlignage) · pondération
--   multicritère (titre vs résumé) · bonus unaccent (recherche insensible aux accents).
--
-- On crée une table demo_ alimentée depuis « livres » (+ du volume synthétique
-- pour que l'index GIN ait un effet mesurable). Tout est nettoyé à la fin.
--
-- IDÉE-CLÉ : le texte est transformé en tsvector (liste de LEXÈMES normalisés :
-- « misérables » -> « miser »). La requête devient un tsquery. L'opérateur « @@ »
-- teste la correspondance. La langue (« french ») pilote radicalisation et
-- mots-vides (« le », « de », « et »… sont ignorés).
-- =============================================================================

\echo '========================================================================'
\echo '  06_recherche_plein_texte.sql — Full Text Search'
\echo '========================================================================'

-- #############################################################################
-- 0) LES BRIQUES : tsvector et tsquery
-- -----------------------------------------------------------------------------
-- to_tsvector normalise un texte en lexèmes (avec position). to_tsquery bâtit une
-- requête avec opérateurs & (ET), | (OU), ! (SAUF), <-> (adjacence).
-- #############################################################################
\echo '\n--- 0) tsvector / tsquery (radicalisation « french ») ---'
SELECT to_tsvector('french', 'Les Misérables racontent une grande histoire') AS tsvector_exemple;
--   -> 'grand':5 'histoir':6 'miser':2 'racontent':3  (mots-vides « les/une » ignorés)

SELECT
    to_tsquery('french', 'misérable & histoire') AS requete_et,
    to_tsvector('french', 'Une histoire de misérables') @@ to_tsquery('french', 'misérable & histoire') AS correspond;  -- true


-- #############################################################################
-- 1) TABLE de démonstration : colonne tsvector GÉNÉRÉE + index GIN
-- -----------------------------------------------------------------------------
-- BONNE PRATIQUE MODERNE : une colonne « GENERATED ALWAYS AS (...) STORED » qui
-- calcule le tsvector AUTOMATIQUEMENT à chaque écriture (plus besoin de trigger).
-- On PONDÈRE : poids 'A' pour le titre (fort), 'B' pour le résumé (moindre).
-- Contrainte : l'expression doit être IMMUTABLE -> on fige la config ('french').
-- #############################################################################
\echo '\n--- 1) Construction de la table + colonne tsvector générée + index GIN ---'

DROP TABLE IF EXISTS demo_catalogue;
CREATE TABLE demo_catalogue (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    titre     text NOT NULL,
    resume    text,
    -- La colonne indexable : titre (poids A) + résumé (poids B), en un seul tsvector.
    recherche tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('french', coalesce(titre, '')),  'A') ||
        setweight(to_tsvector('french', coalesce(resume, '')), 'B')
    ) STORED
);

-- (a) Les VRAIS livres du seed : contenu intéressant pour le classement/surlignage.
INSERT INTO demo_catalogue (titre, resume)
SELECT titre, resume FROM livres WHERE supprime_le IS NULL;

-- (b) Du volume SYNTHÉTIQUE (6000 lignes) au vocabulaire neutre : il gonfle la
--     table pour que l'index GIN devienne clairement rentable, sans polluer les
--     mots distinctifs des vrais livres (Nemo, Poirot, prince…).
INSERT INTO demo_catalogue (titre, resume)
SELECT
    'Ouvrage ' || g || ' ' || (ARRAY['pratique','théorique','illustré','commenté'])[1 + g % 4],
    'Un ' || (ARRAY['guide','manuel','essai','recueil'])[1 + g % 4]
        || ' consacré à ' || (ARRAY['la cuisine','le jardinage','la finance','le bricolage','la randonnée','la photographie'])[1 + g % 6]
FROM generate_series(1, 6000) AS g;

-- Index GIN sur le tsvector : c'est LUI qui rend la recherche rapide à l'échelle.
CREATE INDEX idx_demo_catalogue_gin ON demo_catalogue USING gin (recherche);
ANALYZE demo_catalogue;


-- #############################################################################
-- 2) INTERROGER : @@ et l'index GIN (avec EXPLAIN)
-- -----------------------------------------------------------------------------
-- L'opérateur « @@ » (match) confronte le tsvector (colonne) au tsquery (requête).
-- Grâce au GIN, PostgreSQL localise instantanément les documents concernés.
-- #############################################################################
\echo '\n--- 2) Recherche « capitaine Nemo » + EXPLAIN (Bitmap Index Scan GIN attendu) ---'
SELECT titre, resume
FROM demo_catalogue
WHERE recherche @@ plainto_tsquery('french', 'capitaine Nemo');

\echo '>> Plan de la recherche :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF, BUFFERS OFF)
SELECT id FROM demo_catalogue
WHERE recherche @@ plainto_tsquery('french', 'capitaine Nemo');


-- #############################################################################
-- 3) LES CONSTRUCTEURS DE REQUÊTE — lequel choisir ?
-- -----------------------------------------------------------------------------
--   to_tsquery        : syntaxe experte avec opérateurs (& | ! <->). Puissant mais
--                       exige une entrée bien formée (jamais du texte utilisateur brut).
--   plainto_tsquery   : texte libre -> tous les mots reliés par ET. Simple et sûr.
--   phraseto_tsquery  : impose l'ORDRE et l'ADJACENCE des mots (une vraie « phrase »).
--   websearch_to_tsquery : syntaxe « moteur de recherche » (guillemets, OR, -mot).
--                       LE meilleur choix pour une barre de recherche publique.
-- #############################################################################
\echo '\n--- 3) Comparaison des constructeurs de tsquery ---'
SELECT
    plainto_tsquery('french',      'le petit prince')        AS plain,       -- 'petit' & 'princ'
    phraseto_tsquery('french',     'le petit prince')        AS phrase,      -- 'petit' <-> 'princ'
    websearch_to_tsquery('french', '"petit prince" -robot')  AS websearch,   -- ('petit'<->'princ') & !'robot'
    to_tsquery('french',           'prince | robot')         AS booleen;     -- 'princ' | 'robot'

-- phraseto_tsquery distingue l'ORDRE : « prince petit » ne matche PAS « petit prince ».
SELECT
    to_tsvector('french', 'Le Petit Prince') @@ phraseto_tsquery('french', 'petit prince') AS bon_ordre,   -- true
    to_tsvector('french', 'Le Petit Prince') @@ phraseto_tsquery('french', 'prince petit') AS ordre_inverse; -- false


-- #############################################################################
-- 4) PERTINENCE — classer les résultats avec ts_rank
-- -----------------------------------------------------------------------------
-- ts_rank(vector, query) attribue un SCORE de pertinence (fréquence + positions).
-- ts_rank_cd tient compte de la PROXIMITÉ des termes (« cover density »).
-- La pondération A/B/C/D (définie à la génération) influence directement le score :
-- un mot trouvé dans le TITRE (A) pèse plus que dans le RÉSUMÉ (B).
-- #############################################################################
\echo '\n--- 4) Classement par pertinence (ts_rank) : recherche « prince » ---'
SELECT
    titre,
    round(ts_rank(recherche, plainto_tsquery('french', 'prince'))::numeric, 4) AS pertinence
FROM demo_catalogue
WHERE recherche @@ plainto_tsquery('french', 'prince')
ORDER BY pertinence DESC
LIMIT 5;


-- #############################################################################
-- 5) SURLIGNAGE — ts_headline met en évidence les termes trouvés
-- -----------------------------------------------------------------------------
-- ts_headline renvoie un EXTRAIT du texte avec les termes correspondants encadrés
-- (balises paramétrables). Idéal pour afficher un « aperçu » dans une liste de
-- résultats. ATTENTION : c'est coûteux -> à n'appliquer qu'aux lignes affichées.
-- #############################################################################
\echo '\n--- 5) Surlignage (ts_headline) sur le résumé ---'
SELECT
    titre,
    ts_headline(
        'french',
        resume,
        plainto_tsquery('french', 'enquête Poirot'),
        'StartSel=«, StopSel=», MaxWords=20, MinWords=3'
    ) AS extrait_surligne
FROM demo_catalogue
WHERE recherche @@ plainto_tsquery('french', 'enquête Poirot');


-- #############################################################################
-- 6) MULTICRITÈRE PONDÉRÉ — priorité au titre, avec poids explicites
-- -----------------------------------------------------------------------------
-- ts_rank accepte un TABLEAU de poids {D,C,B,A}. Ci-dessous on VALORISE les
-- correspondances de TITRE (A=1.0) par rapport au résumé (B=0.4). Deux livres
-- contenant « prince » : celui qui l'a dans le TITRE remonte en tête.
-- #############################################################################
\echo '\n--- 6) Recherche multicritère pondérée (titre > résumé) ---'
SELECT
    titre,
    round(
        ts_rank('{0.1, 0.2, 0.4, 1.0}'::float4[],  -- poids {D, C, B, A}
                recherche,
                plainto_tsquery('french', 'prince'))::numeric,
    4) AS score_pondere
FROM demo_catalogue
WHERE recherche @@ plainto_tsquery('french', 'prince')
ORDER BY score_pondere DESC
LIMIT 5;


-- #############################################################################
-- 7) BONUS — recherche INSENSIBLE AUX ACCENTS avec unaccent
-- -----------------------------------------------------------------------------
-- En français, l'utilisateur tape souvent « miserables » sans accent. L'extension
-- « unaccent » (contrib standard) retire les diacritiques. On l'applique des DEUX
-- côtés (document et requête) pour une correspondance robuste.
-- REMARQUE : unaccent() n'est pas IMMUTABLE (elle dépend d'un dictionnaire), on ne
-- peut donc pas l'utiliser telle quelle dans une colonne générée — on l'emploie
-- ici à la volée. En production : un dictionnaire FTS « unaccent + french » dédié.
-- #############################################################################
\echo '\n--- 7) Bonus unaccent : « miserables » (sans accent) retrouve « Misérables » ---'
CREATE EXTENSION IF NOT EXISTS unaccent;

SELECT unaccent('Éléphant à la crème brûlée') AS demonstration_unaccent;

SELECT titre
FROM demo_catalogue
WHERE to_tsvector('french', unaccent(titre))
      @@ plainto_tsquery('french', unaccent('miserables'))   -- requête sans accent
LIMIT 5;


-- #############################################################################
-- QUAND UTILISER LA FTS ? (mémo)
--   - Recherche « intelligente » sur du texte : titres, descriptions, articles.
--   - Besoin de pertinence/classement, de multilangue, de surlignage.
-- Pour du « contient la sous-chaîne » simple (ILIKE '%x%'), c'est pg_trgm + GIN
-- qui convient (voir 03_index.sql et l'index idx_livres_titre_trgm du projet).
-- #############################################################################

-- #############################################################################
-- NETTOYAGE
-- NB : l'extension unaccent est laissée installée (inoffensive, ajout idempotent).
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
DROP TABLE IF EXISTS demo_catalogue;   -- supprime aussi son index GIN

\echo '\n06_recherche_plein_texte.sql : terminé sans erreur.'
