-- =============================================================================
-- 04_sql_avance.sql — SQL AVANCÉ avec PostgreSQL
-- -----------------------------------------------------------------------------
-- OBJET : illustrer les constructions SQL puissantes que PostgreSQL exécute avec
-- brio, sur les données RÉELLES du seed (livres, emprunts, auteurs, catégories) :
--
--   WITH (CTE) · CTE RÉCURSIVE · RETURNING · UPSERT (ON CONFLICT) · DISTINCT ON ·
--   fonctions FENÊTRE (OVER / PARTITION BY, row_number/rank/dense_rank/lag/lead,
--   sum() OVER) · FILTER · GROUPING SETS · ROLLUP · CUBE · LATERAL · EXISTS ·
--   ANY · ALL.
--
-- Les lectures ne modifient rien. Les rares écritures se font sur une table demo_
-- (ou en transaction annulée) et sont nettoyées à la fin.
-- =============================================================================

\echo '========================================================================'
\echo '  04_sql_avance.sql — SQL avancé'
\echo '========================================================================'

-- #############################################################################
-- 1) WITH — Common Table Expression (CTE)
-- -----------------------------------------------------------------------------
-- Une CTE nomme un sous-résultat en tête de requête (« WITH x AS (...) »), ce qui
-- DÉCOUPE une requête complexe en étapes LISIBLES, réutilisables plus bas.
-- BONNE PRATIQUE : privilégier la clarté ; PostgreSQL sait « inliner » les CTE
-- simples (depuis PG 12), donc peu ou pas de surcoût.
-- #############################################################################
\echo '\n--- 1) WITH (CTE) : prix moyen par catégorie, puis livres au-dessus ---'
WITH prix_moyen_categorie AS (
    SELECT categorie_id, avg(prix) AS prix_moyen
    FROM livres
    WHERE supprime_le IS NULL
    GROUP BY categorie_id
)
SELECT c.nom AS categorie, l.titre, l.prix, round(p.prix_moyen, 2) AS moyenne_cat
FROM livres l
    JOIN prix_moyen_categorie p ON p.categorie_id = l.categorie_id
    JOIN categories c           ON c.id = l.categorie_id
WHERE l.prix > p.prix_moyen          -- plus cher que la moyenne de SA catégorie
ORDER BY c.nom, l.prix DESC
LIMIT 8;


-- #############################################################################
-- 2) CTE RÉCURSIVE — parcourir une HIÉRARCHIE ou générer une SÉRIE
-- -----------------------------------------------------------------------------
-- « WITH RECURSIVE » = un terme d'ANCRAGE (non récursif) UNION [ALL] un terme
-- RÉCURSIF qui se référence lui-même, jusqu'à ne plus produire de lignes.
-- Cas typiques : organigrammes, arborescences de catégories, nomenclatures, séries.
-- PIÈGE : sans condition d'arrêt, la récursion est infinie — toujours borner.
-- #############################################################################
\echo '\n--- 2a) CTE récursive : organigramme (table demo_org) ---'

DROP TABLE IF EXISTS demo_org;
CREATE TABLE demo_org (
    id         int PRIMARY KEY,
    nom        text NOT NULL,
    manager_id int REFERENCES demo_org(id)
);
INSERT INTO demo_org (id, nom, manager_id) VALUES
    (1, 'Direction',        NULL),
    (2, 'Pôle Technique',   1),
    (3, 'Pôle Contenus',    1),
    (4, 'Équipe Backend',   2),
    (5, 'Équipe Frontend',  2),
    (6, 'Catalogue',        3);

-- On calcule le NIVEAU et le CHEMIN depuis la racine.
WITH RECURSIVE hierarchie AS (
    -- ANCRAGE : la ou les racines (sans manager).
    SELECT id, nom, manager_id, 1 AS niveau, nom::text AS chemin
    FROM demo_org
    WHERE manager_id IS NULL
    UNION ALL
    -- RÉCURSION : chaque enfant hérite du niveau+1 et du chemin de son parent.
    SELECT e.id, e.nom, e.manager_id, h.niveau + 1, h.chemin || ' > ' || e.nom
    FROM demo_org e
        JOIN hierarchie h ON h.id = e.manager_id
)
SELECT niveau, repeat('    ', niveau - 1) || nom AS arborescence, chemin
FROM hierarchie
ORDER BY chemin;

\echo '\n--- 2b) CTE récursive : série des 6 premiers mois de 2026 ---'
-- (Pour une simple série, generate_series() est plus court ; la récursion montre
--  le mécanisme et gère des progressions non triviales.)
WITH RECURSIVE mois AS (
    SELECT DATE '2026-01-01' AS premier_jour
    UNION ALL
    SELECT (premier_jour + INTERVAL '1 month')::date
    FROM mois
    WHERE premier_jour < DATE '2026-06-01'
)
SELECT premier_jour, to_char(premier_jour, 'TMMonth YYYY') AS libelle
FROM mois;


-- #############################################################################
-- 3) RETURNING — récupérer les lignes AFFECTÉES par une écriture
-- -----------------------------------------------------------------------------
-- INSERT / UPDATE / DELETE ... RETURNING renvoie les lignes touchées (valeurs
-- générées comprises : id, uuid, DEFAULT). Évite un SELECT supplémentaire — le
-- projet s'en sert pour récupérer l'id/uuid d'une ligne juste créée.
-- #############################################################################
\echo '\n--- 3) RETURNING ---'

DROP TABLE IF EXISTS demo_panier;
CREATE TABLE demo_panier (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    article  text NOT NULL,
    quantite int  NOT NULL DEFAULT 1,
    ajoute_le timestamptz NOT NULL DEFAULT now()
);

-- INSERT ... RETURNING : on récupère l'id ET l'horodatage générés par la base.
INSERT INTO demo_panier (article, quantite)
VALUES ('Le Petit Prince', 2), ('Dune', 1)
RETURNING id, article, ajoute_le;

-- UPDATE ... RETURNING : voir l'ancienne/nouvelle valeur en un seul appel.
UPDATE demo_panier SET quantite = quantite + 10
WHERE article = 'Dune'
RETURNING id, article, quantite AS nouvelle_quantite;


-- #############################################################################
-- 4) UPSERT — INSERT ... ON CONFLICT (insérer OU mettre à jour)
-- -----------------------------------------------------------------------------
-- « Insère, mais en cas de conflit sur une contrainte unique, fais autre chose ».
--   ON CONFLICT ... DO NOTHING   : ignore le doublon silencieusement.
--   ON CONFLICT ... DO UPDATE SET: fusionne (accès à EXCLUDED = la ligne proposée).
-- Idéal pour des compteurs, des caches, des imports idempotents.
-- #############################################################################
\echo '\n--- 4) UPSERT (ON CONFLICT) : compteur de vues ---'

DROP TABLE IF EXISTS demo_compteur;
CREATE TABLE demo_compteur (
    page    text PRIMARY KEY,
    vues    int NOT NULL DEFAULT 0,
    maj_le  timestamptz NOT NULL DEFAULT now()
);

-- Trois « visites » de la même page : la 1re insère, les suivantes incrémentent.
INSERT INTO demo_compteur (page, vues) VALUES ('/accueil', 1)
ON CONFLICT (page) DO UPDATE
    SET vues = demo_compteur.vues + EXCLUDED.vues,  -- EXCLUDED = valeurs proposées
        maj_le = now();
INSERT INTO demo_compteur (page, vues) VALUES ('/accueil', 1)
ON CONFLICT (page) DO UPDATE
    SET vues = demo_compteur.vues + EXCLUDED.vues, maj_le = now();
INSERT INTO demo_compteur (page, vues) VALUES ('/accueil', 1)
ON CONFLICT (page) DO UPDATE
    SET vues = demo_compteur.vues + EXCLUDED.vues, maj_le = now();

-- DO NOTHING : une réinsertion ne provoque pas d'erreur, elle est ignorée.
INSERT INTO demo_compteur (page, vues) VALUES ('/accueil', 999)
ON CONFLICT (page) DO NOTHING;

SELECT page, vues FROM demo_compteur;  -- vues = 3 (et non 999)


-- #############################################################################
-- 5) DISTINCT ON — UNE ligne par groupe (la « première » selon un tri)
-- -----------------------------------------------------------------------------
-- Spécificité PostgreSQL : « DISTINCT ON (cle) ... ORDER BY cle, critere » garde,
-- pour chaque valeur de cle, la PREMIÈRE ligne selon l'ORDER BY. Parfait pour
-- « le dernier X par Y » sans sous-requête ni fonction fenêtre.
-- RÈGLE : l'ORDER BY doit COMMENCER par la/les colonnes du DISTINCT ON.
-- #############################################################################
\echo '\n--- 5) DISTINCT ON : dernier emprunt de chaque utilisateur ---'
SELECT DISTINCT ON (e.utilisateur_id)
    e.utilisateur_id,
    concat_ws(' ', u.prenom, u.nom) AS emprunteur,
    e.date_emprunt,
    e.statut
FROM emprunts e
    JOIN utilisateurs u ON u.id = e.utilisateur_id
ORDER BY e.utilisateur_id, e.date_emprunt DESC;  -- DESC => le plus récent


-- #############################################################################
-- 6) FONCTIONS FENÊTRE (WINDOW) — calculer SANS regrouper les lignes
-- -----------------------------------------------------------------------------
-- Une fonction fenêtre calcule sur un « voisinage » (la FENÊTRE, définie par OVER)
-- tout en CONSERVANT chaque ligne (contrairement à GROUP BY qui les fusionne).
--   PARTITION BY : découpe en groupes.   ORDER BY : ordonne dans la fenêtre.
--   row_number/rank/dense_rank : numérotation/classement.
--   lag/lead : valeur de la ligne précédente/suivante.   sum() OVER : cumul.
-- #############################################################################
\echo '\n--- 6a) Classements par catégorie (row_number, rank, dense_rank) ---'
SELECT
    c.nom AS categorie,
    l.titre,
    l.prix,
    row_number() OVER w AS num,        -- numéro unique (1,2,3,4…)
    rank()       OVER w AS rang,        -- ex æquo => sauts (1,1,3…)
    dense_rank() OVER w AS rang_dense   -- ex æquo => sans saut (1,1,2…)
FROM livres l
    JOIN categories c ON c.id = l.categorie_id
WHERE l.supprime_le IS NULL
WINDOW w AS (PARTITION BY l.categorie_id ORDER BY l.prix DESC)
ORDER BY c.nom, l.prix DESC
LIMIT 12;

\echo '\n--- 6b) lag/lead + cumul (sum OVER) sur les emprunts, par ordre de date ---'
SELECT
    e.date_emprunt,
    e.statut,
    lag(e.date_emprunt)  OVER chrono AS emprunt_precedent,
    lead(e.date_emprunt) OVER chrono AS emprunt_suivant,
    count(*)             OVER chrono AS cumul_emprunts   -- total courant (running count)
FROM emprunts e
WINDOW chrono AS (ORDER BY e.date_emprunt, e.id)
ORDER BY e.date_emprunt, e.id
LIMIT 12;


-- #############################################################################
-- 7) FILTER — un agrégat CONDITIONNEL, élégant
-- -----------------------------------------------------------------------------
-- « agg(...) FILTER (WHERE condition) » n'agrège que les lignes remplissant la
-- condition. Plus lisible que « sum(CASE WHEN ... THEN 1 ELSE 0 END) ». Le projet
-- l'utilise dans la vue matérialisée de popularité.
-- #############################################################################
\echo '\n--- 7) FILTER : ventilation des emprunts par statut ---'
SELECT
    count(*)                                          AS total,
    count(*) FILTER (WHERE statut = 'en_cours')       AS en_cours,
    count(*) FILTER (WHERE statut = 'en_retard')      AS en_retard,
    count(*) FILTER (WHERE statut = 'rendu')          AS rendus,
    round(avg(penalite) FILTER (WHERE penalite > 0), 2) AS penalite_moy_si_penalise
FROM emprunts;


-- #############################################################################
-- 8) GROUPING SETS / ROLLUP / CUBE — plusieurs niveaux d'agrégat en UNE requête
-- -----------------------------------------------------------------------------
--   GROUPING SETS : liste EXPLICITE de regroupements à calculer d'un coup.
--   ROLLUP(a,b)   : agrégats hiérarchiques + sous-totaux + total (a, puis a+b, puis ()).
--   CUBE(a,b)     : TOUTES les combinaisons (a,b), (a), (b), ().
-- GROUPING(col) = 1 sur les lignes de sous-total (col « agrégée »). Idéal pour du
-- reporting (tableaux croisés) sans multiplier les requêtes.
-- #############################################################################
\echo '\n--- 8a) ROLLUP : emprunts par statut avec TOTAL général ---'
SELECT
    COALESCE(statut::text, '➤ TOTAL') AS statut,
    count(*)                          AS nb,
    GROUPING(statut)                  AS est_total  -- 1 sur la ligne de total
FROM emprunts
GROUP BY ROLLUP (statut)
ORDER BY GROUPING(statut), statut;

\echo '\n--- 8b) CUBE : croisement rôle x actif des utilisateurs (toutes combinaisons) ---'
SELECT
    COALESCE(role::text, '(tous rôles)') AS role,
    CASE WHEN GROUPING(actif) = 1 THEN '(tous)' ELSE actif::text END AS actif,
    count(*) AS nb
FROM utilisateurs
GROUP BY CUBE (role, actif)
ORDER BY GROUPING(role), role, GROUPING(actif), actif;


-- #############################################################################
-- 9) LATERAL — une sous-requête qui VOIT la ligne courante (Top-N par groupe)
-- -----------------------------------------------------------------------------
-- Une jointure LATERAL peut RÉFÉRENCER les colonnes des tables à sa gauche. Cas
-- phare : « les N meilleurs enfants de chaque parent » (impossible en jointure
-- classique sans astuce). Ici : les 2 livres les plus chers de chaque auteur.
-- #############################################################################
\echo '\n--- 9) LATERAL : les 2 livres les plus chers par auteur ---'
SELECT
    concat_ws(' ', a.prenom, a.nom) AS auteur,
    top.titre,
    top.prix
FROM auteurs a
CROSS JOIN LATERAL (
    SELECT l.titre, l.prix
    FROM livres l
    WHERE l.auteur_id = a.id          -- <-- référence à la ligne « a » : c'est LATERAL
      AND l.supprime_le IS NULL
    ORDER BY l.prix DESC
    LIMIT 2
) AS top
ORDER BY a.nom, top.prix DESC
LIMIT 12;


-- #############################################################################
-- 10) EXISTS / NOT EXISTS — test de PRÉSENCE corrélé (souvent le + efficace)
-- -----------------------------------------------------------------------------
-- EXISTS s'arrête dès la 1re ligne trouvée. NOT EXISTS gère proprement les cas
-- « sans correspondance » (et, contrairement à NOT IN, ne piège pas avec les NULL).
-- #############################################################################
\echo '\n--- 10) NOT EXISTS : livres JAMAIS empruntés ---'
SELECT l.titre
FROM livres l
WHERE l.supprime_le IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM emprunts e WHERE e.livre_id = l.id
  )
ORDER BY l.titre
LIMIT 10;


-- #############################################################################
-- 11) ANY / ALL — comparer une valeur à un ENSEMBLE
-- -----------------------------------------------------------------------------
--   x = ANY(ensemble)   ≡ x IN (ensemble)      (au moins un élément satisfait).
--   x > ALL(ensemble)   : x dépasse TOUS les éléments.
-- L'ensemble peut être une sous-requête OU un tableau.
-- #############################################################################
\echo '\n--- 11a) > ALL : livres plus chers que TOUS les livres de la catégorie Roman ---'
SELECT l.titre, l.prix
FROM livres l
WHERE l.prix > ALL (
    SELECT prix FROM livres
    WHERE categorie_id = (SELECT id FROM categories WHERE nom = 'Roman')
)
ORDER BY l.prix DESC
LIMIT 5;

\echo '\n--- 11b) = ANY(tableau) : utilisateurs privilégiés (équivaut à IN) ---'
SELECT concat_ws(' ', prenom, nom) AS nom, role
FROM utilisateurs
WHERE role::text = ANY (ARRAY['admin', 'bibliothecaire'])
ORDER BY role, nom;


-- #############################################################################
-- NETTOYAGE
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
DROP TABLE IF EXISTS demo_panier;
DROP TABLE IF EXISTS demo_compteur;
DROP TABLE IF EXISTS demo_org;

\echo '\n04_sql_avance.sql : terminé sans erreur.'
