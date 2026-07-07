-- =============================================================================
-- 02_jsonb.sql — JSONB : le document semi-structuré indexable de PostgreSQL
-- -----------------------------------------------------------------------------
-- OBJET : démonstration COMPLÈTE de JSONB, l'une des vitrines de PostgreSQL.
--   création · lecture (->, ->>, #>, #>>) · mise à jour (jsonb_set, ||, -, #-) ·
--   recherche (@>, ?, ?|, ?&, jsonpath) · agrégation (jsonb_agg, jsonb_build_*) ·
--   index GIN (+ EXPLAIN avant/après) · JSON vs JSONB · cas d'usage.
--
-- On s'appuie sur la table RÉELLE « journal_audit » (colonnes JSONB) en LECTURE,
-- et on crée une table demo_ pour les écritures et l'index. Tout est nettoyé.
--
-- RAPPEL : json conserve le texte brut ; JSONB stocke une forme binaire triée et
-- dédoublonnée, INDEXABLE. On choisit JSONB dans la quasi-totalité des cas.
-- =============================================================================

\echo '========================================================================'
\echo '  02_jsonb.sql — JSONB de A à Z'
\echo '========================================================================'

-- #############################################################################
-- 0) JSON vs JSONB — la différence en une requête
-- -----------------------------------------------------------------------------
-- json : garde espaces, ordre des clés et DOUBLONS. jsonb : normalise tout.
-- Conséquence : seul JSONB peut être indexé et interrogé efficacement.
-- #############################################################################
\echo '\n--- 0) JSON vs JSONB ---'
SELECT
    '{"z":1, "a":2, "a":3}'::json  AS json_brut,     -- {"z":1, "a":2, "a":3}
    '{"z":1, "a":2, "a":3}'::jsonb AS jsonb_normalise; -- {"a": 3, "z": 1}


-- #############################################################################
-- 1) CRÉATION — une table de « profils » JSONB
-- -----------------------------------------------------------------------------
-- Cas d'usage typique de JSONB : des attributs HÉTÉROGÈNES/évolutifs qu'on ne
-- veut pas figer en colonnes (préférences, métadonnées, réglages). Pour des
-- données bien structurées et reliées, on garde de VRAIES colonnes/tables.
-- #############################################################################
\echo '\n--- 1) Création + insertion ---'

DROP TABLE IF EXISTS demo_profils;
CREATE TABLE demo_profils (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profil  jsonb NOT NULL
);

-- to_jsonb / littéraux : plusieurs styles d'insertion.
INSERT INTO demo_profils (profil) VALUES
    ('{"nom":"Alice","age":30,"ville":"Paris","tags":["go","sql"],
       "adresse":{"cp":"75001","pays":"FR"}}'),
    ('{"nom":"Bruno","age":45,"ville":"Lyon","tags":["docker"],
       "adresse":{"cp":"69001","pays":"FR"}}'),
    ('{"nom":"Chloé","age":25,"ville":"Paris","tags":["go","docker","k8s"],
       "adresse":{"cp":"75012","pays":"FR"}}');


-- #############################################################################
-- 2) LECTURE — extraire des valeurs
-- -----------------------------------------------------------------------------
--   ->   : accède à un champ/élément, renvoie du JSONB (garde les guillemets).
--   ->>  : idem mais renvoie du TEXTE (à utiliser pour comparer/afficher).
--   #>   : suit un CHEMIN (array de clés), renvoie du JSONB.
--   #>>  : suit un chemin, renvoie du TEXTE.
-- PIÈGE : « ->> » donne du texte ; pour comparer un nombre, caster (->> ... )::int.
-- #############################################################################
\echo '\n--- 2) Lecture (->, ->>, #>, #>>) ---'
SELECT
    profil -> 'nom'                     AS nom_jsonb,     -- "Alice" (avec guillemets)
    profil ->> 'nom'                    AS nom_texte,     -- Alice   (texte nu)
    (profil ->> 'age')::int             AS age_entier,    -- 30 (cast nécessaire)
    profil -> 'tags' -> 0               AS premier_tag,   -- "go"
    profil #> '{adresse,cp}'            AS cp_jsonb,      -- "75001"
    profil #>> '{adresse,pays}'         AS pays_texte     -- FR
FROM demo_profils
ORDER BY id;


-- #############################################################################
-- 3) MISE À JOUR — modifier un document
-- -----------------------------------------------------------------------------
--   jsonb_set(doc, chemin, valeur[, create_missing]) : remplace/ajoute à un chemin.
--   ||  : fusionne/concatène (les clés de droite écrasent celles de gauche).
--   -   : retire une clé (texte) ou un élément de tableau (indice).
--   #-  : retire à un CHEMIN profond.
-- Ces opérateurs renvoient un NOUVEAU jsonb : on l'affecte via UPDATE ... SET.
-- #############################################################################
\echo '\n--- 3) Mise à jour (jsonb_set, ||, -, #-) ---'

-- On travaille dans une transaction annulée : la démo ne laisse aucune trace.
BEGIN;

-- a) jsonb_set : passer l'âge d'Alice à 31.
UPDATE demo_profils
SET profil = jsonb_set(profil, '{age}', '31')
WHERE profil ->> 'nom' = 'Alice';

-- b) || : ajouter/mettre à jour plusieurs clés d'un coup (fusion).
UPDATE demo_profils
SET profil = profil || '{"actif":true,"ville":"Marseille"}'::jsonb
WHERE profil ->> 'nom' = 'Bruno';

-- c) - : retirer une clé de premier niveau.
UPDATE demo_profils
SET profil = profil - 'tags'
WHERE profil ->> 'nom' = 'Chloé';

-- d) #- : retirer une clé imbriquée (adresse.cp).
UPDATE demo_profils
SET profil = profil #- '{adresse,cp}'
WHERE profil ->> 'nom' = 'Alice';

SELECT jsonb_pretty(profil) AS apres_modifs FROM demo_profils ORDER BY id;

ROLLBACK;  -- on annule : la table retrouve son état initial.


-- #############################################################################
-- 4) RECHERCHE — les opérateurs qui font la force de JSONB
-- -----------------------------------------------------------------------------
--   @>  : CONTENANCE — « le document contient-il ce sous-document ? » (le + utile).
--   ?   : la clé existe-t-elle au premier niveau ?
--   ?|  : au moins UNE de ces clés existe ?
--   ?&  : TOUTES ces clés existent ?
--   @@ / jsonb_path_* : requêtes SQL/JSON path (filtres riches).
-- @>, ?, ?|, ?& sont ACCÉLÉRABLES par un index GIN (voir section 6).
-- #############################################################################
\echo '\n--- 4) Recherche (@>, ?, ?|, ?&, jsonpath) ---'

-- @> : profils habitant Paris (sous-document contenu).
SELECT profil ->> 'nom' AS parisiens
FROM demo_profils
WHERE profil @> '{"ville":"Paris"}';

-- @> fonctionne aussi dans les tableaux : profils ayant le tag "go".
SELECT profil ->> 'nom' AS adeptes_go
FROM demo_profils
WHERE profil @> '{"tags":["go"]}';

-- ? / ?| / ?& : présence de clés.
SELECT
    '{"a":1,"b":2}'::jsonb ? 'a'                 AS a_existe,        -- true
    '{"a":1,"b":2}'::jsonb ?| ARRAY['x','b']     AS au_moins_une,    -- true
    '{"a":1,"b":2}'::jsonb ?& ARRAY['a','b']     AS toutes,          -- true
    '{"a":1,"b":2}'::jsonb ?& ARRAY['a','z']     AS toutes_bis;      -- false

-- jsonpath : requête déclarative dans le document (ici, tags de plus de 2 lettres).
SELECT jsonb_path_query_array(profil, '$.tags[*] ? (@.type() == "string")') AS tous_les_tags
FROM demo_profils
WHERE profil ? 'tags'
ORDER BY id;


-- #############################################################################
-- 5) AGRÉGATION & CONSTRUCTION — bâtir du JSON depuis des lignes
-- -----------------------------------------------------------------------------
--   jsonb_build_object(...) : construit un objet clé/valeur.
--   jsonb_agg(...)          : agrège des lignes en TABLEAU JSON.
--   jsonb_object_agg(k, v)  : agrège en OBJET.
-- Très pratique pour renvoyer une réponse d'API imbriquée en UNE requête.
-- #############################################################################
\echo '\n--- 5) Agrégation (jsonb_build_object, jsonb_agg) ---'

-- Construire un objet à la volée.
SELECT jsonb_build_object(
    'total_profils', count(*),
    'genere_le',     now()
) AS resume
FROM demo_profils;

-- Regrouper des données RELATIONNELLES du seed en JSON : pour 3 catégories, la
-- liste JSON de leurs livres (jointure réelle livres/categories).
SELECT jsonb_pretty(jsonb_agg(cat)) AS catalogue_json
FROM (
    SELECT jsonb_build_object(
        'categorie', c.nom,
        'livres', (
            SELECT jsonb_agg(l.titre ORDER BY l.titre)
            FROM livres l
            WHERE l.categorie_id = c.id AND l.supprime_le IS NULL
        )
    ) AS cat
    FROM categories c
    WHERE c.nom IN ('Roman', 'Science-fiction', 'Policier')
    ORDER BY c.nom
) s;


-- #############################################################################
-- 6) INDEX GIN — rendre @> rapide (avec EXPLAIN avant/après)
-- -----------------------------------------------------------------------------
-- Un index GIN « éclate » le document et indexe ses clés/valeurs. Il accélère
-- @>, ?, ?|, ?&. On le PROUVE : même requête, plan AVANT puis APRÈS l'index.
--
-- Deux classes d'opérateurs :
--   - jsonb_ops (défaut)  : gère @>, ?, ?|, ?& ; index plus gros.
--   - jsonb_path_ops      : gère @> UNIQUEMENT ; index plus petit et souvent + rapide.
-- #############################################################################
\echo '\n--- 6) Index GIN + EXPLAIN ---'

-- On grossit une table demo pour que l'index ait un intérêt mesurable (20 000 lignes).
DROP TABLE IF EXISTS demo_gros_json;
CREATE TABLE demo_gros_json (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    donnees jsonb NOT NULL
);

INSERT INTO demo_gros_json (donnees)
SELECT jsonb_build_object(
    'ville', (ARRAY['Paris','Lyon','Marseille','Lille','Nice','Rennes'])[1 + (g % 6)],
    'niveau', 1 + (g % 10),
    'actif', (g % 2 = 0)
)
FROM generate_series(1, 20000) AS g;

ANALYZE demo_gros_json;

-- AVANT index : une recherche @> impose un parcours séquentiel (Seq Scan).
\echo '>> Plan AVANT index (attendu : Seq Scan) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT count(*) FROM demo_gros_json WHERE donnees @> '{"ville":"Rennes"}';

-- Création de l'index GIN (classe jsonb_path_ops : ciblée sur @>, compacte).
CREATE INDEX idx_demo_gros_json_gin
    ON demo_gros_json USING gin (donnees jsonb_path_ops);
ANALYZE demo_gros_json;

-- APRÈS index : la même requête passe par un Bitmap Index Scan sur le GIN.
\echo '>> Plan APRÈS index (attendu : Bitmap Index Scan sur le GIN) :'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT count(*) FROM demo_gros_json WHERE donnees @> '{"ville":"Rennes"}';


-- #############################################################################
-- 7) CAS RÉEL — lecture du journal_audit (JSONB du projet)
-- -----------------------------------------------------------------------------
-- journal_audit stocke une PHOTO JSONB des lignes avant/après (alimentée par des
-- triggers). Elle possède déjà un index GIN (idx_audit_nouvelles_gin). On lit
-- ces documents sans rien modifier.
-- #############################################################################
\echo '\n--- 7) Cas réel : journal_audit ---'

-- Extraire des champs du document d'audit (INSERT sur utilisateurs).
SELECT
    cle_enregistrement                       AS id_utilisateur,
    nouvelles_valeurs ->> 'email'            AS email,
    nouvelles_valeurs ->> 'role'             AS role
FROM journal_audit
WHERE table_concernee = 'utilisateurs'
  AND operation = 'INSERT'
ORDER BY cle_enregistrement
LIMIT 5;

-- Recherche par CONTENANCE @> (celle qu'accélère l'index GIN existant) :
-- toutes les écritures d'audit concernant un administrateur.
SELECT count(*) AS ecritures_role_admin
FROM journal_audit
WHERE nouvelles_valeurs @> '{"role":"admin"}';


-- #############################################################################
-- QUAND UTILISER JSONB ? (mémo)
--   OUI : attributs souples/évolutifs, métadonnées, payloads d'audit, réglages.
--   NON : données fortement reliées et contraintes (préférer colonnes + FK).
-- Ne pas « tout mettre en JSONB » : on perd les contraintes et la lisibilité.
-- #############################################################################

-- #############################################################################
-- NETTOYAGE
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
DROP TABLE IF EXISTS demo_gros_json;   -- supprime aussi son index GIN
DROP TABLE IF EXISTS demo_profils;

\echo '\n02_jsonb.sql : terminé sans erreur.'
