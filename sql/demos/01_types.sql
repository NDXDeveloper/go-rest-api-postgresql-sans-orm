-- =============================================================================
-- 01_types.sql — Les TYPES de données de PostgreSQL (démonstration autonome)
-- -----------------------------------------------------------------------------
-- OBJET : illustrer, exemples à l'appui, la richesse du système de types de
-- PostgreSQL, l'une de ses forces majeures face à MariaDB. On y couvre :
--
--   UUID · JSON vs JSONB · ARRAY · ENUM · DOMAIN · BYTEA ·
--   TIMESTAMP WITH TIME ZONE (vs timestamp) · INTERVAL · NUMERIC (vs float)
--
-- POUR CHAQUE TYPE : à quoi il sert, comment le créer/manipuler, les bonnes
-- pratiques et les pièges classiques.
--
-- AUTONOMIE : ce script ne touche PAS au schéma applicatif. Il crée ses propres
-- objets, tous préfixés « demo_ », et les SUPPRIME à la fin. Aucune donnée du
-- seed n'est modifiée (les rares écritures se font sur des tables demo_).
--
-- LANCEMENT :
--   docker exec -i pg-test psql -U postgres -d bibliotheque \
--       -v ON_ERROR_STOP=1 -f - < sql/demos/01_types.sql
-- =============================================================================

\echo '========================================================================'
\echo '  01_types.sql — Types de données PostgreSQL'
\echo '========================================================================'

-- #############################################################################
-- 1) UUID — identifiant universellement unique (128 bits)
-- -----------------------------------------------------------------------------
-- POURQUOI : un UUID est non devinable et générable côté client SANS aller-retour
-- avec la base (pas de collision pratique). Idéal comme identifiant PUBLIC exposé
-- par l'API (anti-énumération), ce que fait la colonne « uuid » du projet.
--
-- PIÈGE fréquent : stocker un UUID en CHAR(36). C'est 2,25× plus gros (36 octets
-- de texte) et non typé. Le type natif « uuid » n'occupe que 16 octets.
-- #############################################################################
\echo '\n--- 1) UUID ---'

-- Deux façons de générer un UUID v4 (aléatoire) :
--   - gen_random_uuid() : natif depuis PG 13 (et fourni par pgcrypto) — À PRÉFÉRER.
--   - uuid_generate_v4() : historique, via l'extension uuid-ossp.
SELECT
    gen_random_uuid()   AS via_natif_pgcrypto,
    uuid_generate_v4()  AS via_uuid_ossp;

-- Preuve du gain de place : 16 octets (type uuid) contre 36+ (texte).
SELECT
    pg_column_size(gen_random_uuid())        AS octets_type_uuid,   -- 16
    pg_column_size(gen_random_uuid()::text)  AS octets_en_texte;    -- 40 (36 + entête)

-- Un littéral texte se caste en uuid ; la comparaison est insensible à la casse
-- et aux tirets (les deux écritures ci-dessous sont ÉGALES).
SELECT
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::uuid
        = 'A0EEBC999C0B4EF8BB6D6BB9BD380A11'::uuid   AS sont_egaux;  -- true


-- #############################################################################
-- 2) JSON vs JSONB — document semi-structuré
-- -----------------------------------------------------------------------------
-- json  : stocke le TEXTE tel quel (espaces, ordre des clés, doublons conservés).
--         Aucune indexation possible ; reparsé à chaque lecture.
-- jsonb : stocke une forme BINAIRE normalisée (clés triées, espaces et doublons
--         supprimés). Indexable en GIN, opérateurs riches (@>, ?, etc.).
--
-- RÈGLE : en pratique on choisit JSONB (99 % des cas). JSON ne se justifie que
-- si l'on doit restituer le document TEXTUEL à l'octet près. Détails en 02_jsonb.sql.
-- #############################################################################
\echo '\n--- 2) JSON vs JSONB ---'

-- Même entrée, avec espaces, doublon de clé « a » et ordre b/a/a :
WITH entree AS (SELECT '  {"b":1, "a":2, "a":3}  ' AS brut)
SELECT
    brut::json  AS json_conserve_tout,   -- garde les espaces, l'ordre et le doublon
    brut::jsonb AS jsonb_normalise        -- => {"a": 3, "b": 1} (trié, dédoublonné)
FROM entree;


-- #############################################################################
-- 3) ARRAY — tableaux natifs (une dimension ou plus)
-- -----------------------------------------------------------------------------
-- POURQUOI : PostgreSQL sait stocker un tableau dans UNE colonne, avec des
-- opérateurs ensemblistes et des fonctions dédiées. Pratique pour des étiquettes
-- (« tags »), sans créer de table de liaison quand la relation reste simple.
--
-- ATTENTION : les tableaux SQL sont indexés à partir de 1 (pas de 0 !).
-- BONNE PRATIQUE : au-delà d'un usage « étiquettes », préférer une vraie table
-- relationnelle (jointures, contraintes) — un tableau ne référence pas de clé.
-- #############################################################################
\echo '\n--- 3) ARRAY ---'

-- Deux écritures équivalentes d'un tableau de texte :
SELECT
    '{roman,policier,classique}'::text[]        AS litteral_accolades,
    ARRAY['roman','policier','classique']       AS constructeur_array;

-- Accès par indice (1-based), découpage (slice), taille et cardinalité.
SELECT
    (ARRAY['a','b','c','d'])[1]        AS premier_element,   -- 'a'
    (ARRAY['a','b','c','d'])[2:3]      AS tranche_2_a_3,     -- {b,c}
    array_length(ARRAY['a','b','c'],1) AS longueur,          -- 3
    cardinality(ARRAY['a','b','c'])    AS cardinalite;       -- 3

-- Opérateurs ensemblistes : contenance (@>, <@), intersection (&&), concat (||).
SELECT
    ARRAY[1,2,3,4] @> ARRAY[2,3]          AS contient,        -- true (contient)
    ARRAY[2,3]     <@ ARRAY[1,2,3,4]      AS est_contenu,     -- true (est inclus)
    ARRAY[1,2,3]   && ARRAY[3,4,5]        AS se_chevauchent,  -- true (élément commun)
    ARRAY[1,2] || ARRAY[3,4]              AS concatenation;   -- {1,2,3,4}

-- ANY / ALL : tester une valeur contre tout un tableau.
SELECT
    'policier' = ANY(ARRAY['roman','policier'])  AS appartient,      -- true
    5 > ALL(ARRAY[1,2,3])                        AS superieur_a_tous; -- true

-- unnest : « déplier » un tableau en lignes (l'inverse : array_agg).
SELECT etiquette
FROM unnest(ARRAY['go','postgresql','docker']) AS etiquette
ORDER BY etiquette;

-- Passage texte <-> tableau : très utile pour des paramètres d'API (CSV).
SELECT
    string_to_array('go,rust,zig', ',')                 AS texte_vers_tableau,
    array_to_string(ARRAY['go','rust','zig'], ' | ')    AS tableau_vers_texte;


-- #############################################################################
-- 4) ENUM — liste fermée de valeurs, ORDONNÉE
-- -----------------------------------------------------------------------------
-- Le projet utilise déjà role_utilisateur, statut_emprunt, operation_audit.
-- Ici on crée un ENUM demo_ pour ne PAS toucher aux types applicatifs.
--
-- FORCE : l'ordre de DÉCLARATION définit l'ordre de tri (< , >, ORDER BY).
-- PIÈGE : retirer une valeur est impossible ; en ajouter une se fait avec
--         « ALTER TYPE ... ADD VALUE » (hors transaction en usage courant).
-- #############################################################################
\echo '\n--- 4) ENUM ---'

DROP TYPE IF EXISTS demo_taille CASCADE;
CREATE TYPE demo_taille AS ENUM ('petit', 'moyen', 'grand');

-- L'ordre déclaré fait foi : petit < moyen < grand.
SELECT 'petit'::demo_taille < 'grand'::demo_taille AS petit_avant_grand;  -- true

-- enum_range() liste les valeurs dans l'ordre.
SELECT enum_range(NULL::demo_taille) AS valeurs_possibles;  -- {petit,moyen,grand}

-- Ajout d'une valeur EN FIN (ou AFTER/BEFORE une valeur existante).
ALTER TYPE demo_taille ADD VALUE IF NOT EXISTS 'geant' AFTER 'grand';
SELECT enum_range(NULL::demo_taille) AS apres_ajout;  -- {petit,moyen,grand,geant}


-- #############################################################################
-- 5) DOMAIN — un type de base + une contrainte CHECK réutilisable
-- -----------------------------------------------------------------------------
-- Le projet définit déjà les DOMAIN « courriel » et « isbn13 ». On illustre ici
-- le principe avec un domaine demo_, et surtout le REJET automatique d'une valeur
-- invalide (défense en profondeur : la règle vit dans la base, pas seulement en Go).
-- #############################################################################
\echo '\n--- 5) DOMAIN ---'

DROP DOMAIN IF EXISTS demo_note_sur_20 CASCADE;
-- Une note est un numeric OBLIGATOIREMENT compris entre 0 et 20.
CREATE DOMAIN demo_note_sur_20 AS numeric
    CHECK (VALUE >= 0 AND VALUE <= 20);

-- Valeur valide : acceptée.
SELECT 15.5::demo_note_sur_20 AS note_valide;

-- Valeur invalide : le CHECK du domaine la REFUSE. On capture l'erreur pour que
-- le script continue (sinon ON_ERROR_STOP l'interromprait).
DO $$
BEGIN
    PERFORM 42::demo_note_sur_20;   -- 42 > 20 : interdit
    RAISE NOTICE 'Inattendu : 42 aurait dû être rejeté.';
EXCEPTION
    WHEN check_violation THEN
        RAISE NOTICE 'OK, 42 rejeté par le DOMAIN (règle 0..20). Détail : %', SQLERRM;
END$$;


-- #############################################################################
-- 6) BYTEA — données BINAIRES brutes
-- -----------------------------------------------------------------------------
-- POURQUOI : stocker des octets (empreintes, petites images, clés...). Le projet
-- s'en sert indirectement via les hachages. Format d'affichage par défaut : hex.
--
-- BONNE PRATIQUE : ne pas stocker de GROS fichiers en base (préférer un stockage
-- objet + une URL). BYTEA convient aux petites valeurs binaires.
-- #############################################################################
\echo '\n--- 6) BYTEA ---'

-- Écriture d'octets, conversion hex <-> bytea, taille en octets.
SELECT
    '\xDEADBEEF'::bytea            AS octets_litteral,
    decode('DEADBEEF', 'hex')      AS depuis_hex,
    length('\xDEADBEEF'::bytea)    AS nb_octets,          -- 4
    encode('\xDEADBEEF'::bytea, 'base64') AS en_base64;    -- 3q2+7w==

-- Empreinte SHA-256 (via pgcrypto) d'un texte : renvoie 32 octets (bytea).
SELECT
    encode(digest('MotDePasse123!', 'sha256'), 'hex') AS sha256_hex,
    length(digest('MotDePasse123!', 'sha256'))        AS taille_en_octets;  -- 32


-- #############################################################################
-- 7) TIMESTAMP WITH TIME ZONE (timestamptz) vs TIMESTAMP (sans fuseau)
-- -----------------------------------------------------------------------------
-- RÈGLE D'OR (appliquée dans tout le projet) : TOUJOURS timestamptz.
--   - timestamptz : l'instant est stocké en UTC, puis AFFICHÉ dans le fuseau de
--                   la session. C'est un point ABSOLU sur la ligne du temps.
--   - timestamp   : « mur d'horloge » sans fuseau : ambigu, ne bouge jamais à
--                   l'affichage. Source de bugs (heure d'été, serveurs distants).
-- #############################################################################
\echo '\n--- 7) TIMESTAMPTZ vs TIMESTAMP ---'

DROP TABLE IF EXISTS demo_horodatage;
CREATE TABLE demo_horodatage (
    avec_fuseau timestamptz,
    sans_fuseau timestamp
);

-- On insère le MÊME instant écrit avec un décalage +02:00 (heure de Paris l'été).
INSERT INTO demo_horodatage VALUES
    ('2026-07-06 14:30:00+02', '2026-07-06 14:30:00');

-- Affichage en UTC : la colonne timestamptz est ramenée à 12:30 UTC ; la colonne
-- timestamp reste littéralement 14:30 (elle ignore tout fuseau).
SET TimeZone = 'UTC';
SELECT avec_fuseau, sans_fuseau FROM demo_horodatage;

-- Affichage à Tokyo (+09:00) : SEULE la colonne timestamptz se décale (21:30) ;
-- la colonne timestamp affiche encore 14:30. D'où le risque d'un timestamp nu.
SET TimeZone = 'Asia/Tokyo';
SELECT avec_fuseau, sans_fuseau FROM demo_horodatage;

-- Conversion explicite d'un instant vers un fuseau donné avec AT TIME ZONE.
SET TimeZone = 'UTC';
SELECT
    now()                                AS maintenant_utc,
    now() AT TIME ZONE 'Europe/Paris'    AS meme_instant_a_paris;


-- #############################################################################
-- 8) INTERVAL — durées et ARITHMÉTIQUE de dates
-- -----------------------------------------------------------------------------
-- POURQUOI : additionner/soustraire des durées à des dates de façon lisible.
-- Le projet calcule par ex. « date_emprunt + 14 jours » (délai de retour).
--
-- ASTUCE : (date - date) donne un ENTIER de jours ; (timestamptz - timestamptz)
-- donne un INTERVAL. age() donne une durée « humaine » (années/mois/jours).
-- #############################################################################
\echo '\n--- 8) INTERVAL ---'

SELECT
    INTERVAL '1 year 2 mons 10 days'         AS duree_litterale,
    CURRENT_DATE + INTERVAL '14 days'        AS retour_prevu,     -- règle des emprunts
    CURRENT_DATE - INTERVAL '1 month'        AS il_y_a_un_mois;

-- Différences de dates : jours (entier) vs intervalle détaillé.
SELECT
    DATE '2026-07-06' - DATE '2026-06-01'    AS nb_jours,          -- 35 (integer)
    age(DATE '2026-07-06', DATE '2000-01-01') AS age_humain,       -- 26 years 6 mons 5 days
    EXTRACT(day FROM INTERVAL '3 days 05:00:00') AS extrait_jours;  -- 3

-- justify_interval : normalise (30 jours -> 1 mois, 24 h -> 1 jour).
SELECT justify_interval(INTERVAL '45 days 26 hours') AS normalise; -- 1 mon 16 days 02:00:00


-- #############################################################################
-- 9) NUMERIC (décimal EXACT) vs FLOAT (binaire APPROCHÉ)
-- -----------------------------------------------------------------------------
-- RÈGLE MÉTIER : pour l'ARGENT, TOUJOURS numeric (la colonne « prix » l'est).
--   - numeric(p,s) : précision décimale exacte, aucun arrondi surprise.
--   - float8       : rapide mais APPROCHÉ (base 2) : 0.1 + 0.2 ≠ 0.3.
-- Utiliser float pour de la monnaie = bug d'arrondi garanti à grande échelle.
-- #############################################################################
\echo '\n--- 9) NUMERIC vs FLOAT ---'

-- La démonstration classique de l'imprécision du flottant :
SELECT
    (0.1::float8 + 0.2::float8) AS somme_float,                 -- 0.30000000000000004
    (0.1::float8 + 0.2::float8) = 0.3::float8   AS egal_en_float,   -- FALSE (!)
    (0.1::numeric + 0.2::numeric) = 0.3::numeric AS egal_en_numeric; -- TRUE

-- Cumul de 1 centime, 1000 fois : le flottant DÉRIVE, le numeric reste exact.
SELECT
    sum(0.01::float8)   AS total_float,     -- 9.999999999999831 au lieu de 10
    sum(0.01::numeric)  AS total_numeric    -- 10.00 exact
FROM generate_series(1, 1000);

-- round() sur numeric : arrondi maîtrisé à 2 décimales (idéal facturation).
SELECT round(19.99::numeric * 3, 2) AS total_ttc;  -- 59.97


-- #############################################################################
-- NETTOYAGE — on retire TOUS les objets demo_ créés par ce script.
-- (Les SELECT ci-dessus n'ont laissé aucune trace ; seuls table/type/domain le sont.)
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
SET TimeZone = 'UTC';                        -- on rétablit un fuseau neutre
DROP TABLE  IF EXISTS demo_horodatage;
DROP TYPE   IF EXISTS demo_taille CASCADE;
DROP DOMAIN IF EXISTS demo_note_sur_20 CASCADE;

\echo '\n01_types.sql : terminé sans erreur.'
