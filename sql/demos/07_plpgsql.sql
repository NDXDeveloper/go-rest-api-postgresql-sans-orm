-- =============================================================================
-- 07_plpgsql.sql — PROGRAMMER dans la base avec PL/pgSQL
-- -----------------------------------------------------------------------------
-- OBJET : le langage procédural natif de PostgreSQL. Bien plus expressif que les
-- routines stockées de MariaDB (types composites, gestion fine des exceptions,
-- curseurs, RETURNS TABLE/SETOF…). On y montre, sur des fonctions demo_ :
--
--   variables · IF/THEN/ELSIF · CASE · boucles FOR (entier ET sur requête) /
--   WHILE / LOOP · CURSEUR explicite (DECLARE/OPEN/FETCH/CLOSE) · gestion des
--   EXCEPTIONS (unique_violation, others) · RAISE NOTICE/WARNING/EXCEPTION ·
--   RETURN · RETURNS TABLE · RETURNS SETOF · type COMPOSITE et RECORD ·
--   bonus : une PROCÉDURE (CALL).
--
-- INTÉRÊT : encapsuler une règle métier réutilisable, exécutée AU PLUS PRÈS des
-- données (moins d'allers-retours réseau). Le projet applique ce principe avec
-- ses fonctions fn_* et procédures pr_*.
--
-- Le corps d'une fonction est délimité par le « dollar-quoting » ($$ … $$) : pas
-- de DELIMITER à manipuler comme en MariaDB. Tout est nettoyé à la fin.
-- =============================================================================

\echo '========================================================================'
\echo '  07_plpgsql.sql — PL/pgSQL'
\echo '========================================================================'

-- #############################################################################
-- 1) VARIABLES · IF/ELSIF/ELSE · CASE — une fonction de classification
-- -----------------------------------------------------------------------------
-- DECLARE introduit des variables typées (« := » pour affecter). IF/ELSIF/ELSE
-- pour les branchements ; CASE pour un aiguillage sur valeur/condition.
-- « IMMUTABLE » ci-dessous : la sortie ne dépend que de l'entrée (aide l'optimiseur).
-- #############################################################################
\echo '\n--- 1) Variables, IF/ELSIF, CASE ---'

CREATE OR REPLACE FUNCTION demo_fn_categoriser_prix(p_prix numeric)
    RETURNS text
    LANGUAGE plpgsql
    IMMUTABLE
AS $$
DECLARE
    v_gamme  text;
    v_symbole text;
BEGIN
    -- IF / ELSIF / ELSE : déterminer la gamme de prix.
    IF p_prix < 10 THEN
        v_gamme := 'économique';
    ELSIF p_prix < 20 THEN
        v_gamme := 'standard';
    ELSE
        v_gamme := 'premium';
    END IF;

    -- CASE : associer un symbole à la gamme (aiguillage sur valeur).
    v_symbole := CASE v_gamme
                     WHEN 'économique' THEN '€'
                     WHEN 'standard'   THEN '€€'
                     ELSE                   '€€€'
                 END;

    RETURN format('%s (%s)', v_gamme, v_symbole);
END;
$$;

-- Démonstration sur de vrais prix du catalogue.
SELECT titre, prix, demo_fn_categoriser_prix(prix) AS gamme
FROM livres
WHERE supprime_le IS NULL
ORDER BY prix
LIMIT 6;


-- #############################################################################
-- 2) BOUCLES — FOR (entier), WHILE, LOOP/EXIT
-- -----------------------------------------------------------------------------
-- Trois formes complémentaires :
--   FOR i IN 1..n : itère sur un intervalle d'entiers (le plus courant).
--   WHILE cond    : tant que la condition reste vraie.
--   LOOP … EXIT   : boucle « infinie » qu'on quitte explicitement (EXIT WHEN …).
-- #############################################################################
\echo '\n--- 2) Boucles FOR / WHILE / LOOP ---'

-- FOR entier : factorielle n! (accumulation dans une variable).
CREATE OR REPLACE FUNCTION demo_fn_factorielle(p_n int)
    RETURNS bigint
    LANGUAGE plpgsql
    IMMUTABLE
AS $$
DECLARE
    v_resultat bigint := 1;
BEGIN
    FOR i IN 1..p_n LOOP        -- i est déclarée implicitement par le FOR
        v_resultat := v_resultat * i;
    END LOOP;
    RETURN v_resultat;
END;
$$;

-- WHILE : somme 1+2+…+n (démontre la condition en tête de boucle).
CREATE OR REPLACE FUNCTION demo_fn_somme_while(p_n int)
    RETURNS int
    LANGUAGE plpgsql
    IMMUTABLE
AS $$
DECLARE
    v_i     int := 1;
    v_somme int := 0;
BEGIN
    WHILE v_i <= p_n LOOP
        v_somme := v_somme + v_i;
        v_i := v_i + 1;
    END LOOP;
    RETURN v_somme;
END;
$$;

-- LOOP / EXIT WHEN : compter les puissances de 2 sous un plafond.
CREATE OR REPLACE FUNCTION demo_fn_puissances_sous(p_plafond int)
    RETURNS int
    LANGUAGE plpgsql
    IMMUTABLE
AS $$
DECLARE
    v_valeur int := 1;
    v_nb     int := 0;
BEGIN
    LOOP
        EXIT WHEN v_valeur >= p_plafond;   -- sortie explicite
        v_nb := v_nb + 1;
        v_valeur := v_valeur * 2;
    END LOOP;
    RETURN v_nb;
END;
$$;

SELECT
    demo_fn_factorielle(6)        AS "6!",              -- 720
    demo_fn_somme_while(100)      AS "somme_1_a_100",   -- 5050
    demo_fn_puissances_sous(1000) AS "puissances_2_<1000"; -- 10 (1,2,4,…,512)


-- #############################################################################
-- 3) FOR sur REQUÊTE + RECORD — itérer sur des lignes
-- -----------------------------------------------------------------------------
-- « FOR rec IN SELECT … LOOP » parcourt un résultat ligne à ligne. « rec » est un
-- RECORD (structure polymorphe dont les champs suivent le SELECT). On l'utilise
-- pour un traitement impératif ligne par ligne quand une requête ensembliste ne
-- suffit pas. (Ici on construit un petit compte-rendu texte.)
-- #############################################################################
\echo '\n--- 3) FOR sur requête + RECORD ---'

CREATE OR REPLACE FUNCTION demo_fn_apercu_categories()
    RETURNS text
    LANGUAGE plpgsql
    STABLE
AS $$
DECLARE
    rec         RECORD;          -- structure déduite du SELECT
    v_rapport   text := '';
BEGIN
    FOR rec IN
        SELECT c.nom, count(l.id) AS nb
        FROM categories c
        LEFT JOIN livres l ON l.categorie_id = c.id AND l.supprime_le IS NULL
        GROUP BY c.nom
        ORDER BY nb DESC, c.nom
        LIMIT 4
    LOOP
        v_rapport := v_rapport || format('%s=%s  ', rec.nom, rec.nb);
    END LOOP;
    RETURN trim(v_rapport);
END;
$$;

SELECT demo_fn_apercu_categories() AS apercu;


-- #############################################################################
-- 4) CURSEUR EXPLICITE — DECLARE / OPEN / FETCH / CLOSE
-- -----------------------------------------------------------------------------
-- Un curseur permet de PARCOURIR un résultat à la main, ligne par ligne. Le
-- « FOR … IN SELECT » ci-dessus en est la forme IMPLICITE et concise. Le curseur
-- EXPLICITE est utile pour un contrôle fin (FETCH conditionnel, gros volumes
-- traités par lots sans tout charger en mémoire).
-- #############################################################################
\echo '\n--- 4) Curseur explicite (OPEN/FETCH/CLOSE) ---'

CREATE OR REPLACE FUNCTION demo_fn_inventaire_curseur()
    RETURNS text
    LANGUAGE plpgsql
    STABLE
AS $$
DECLARE
    -- Déclaration du curseur (lié à une requête).
    c_livres CURSOR FOR
        SELECT titre, exemplaires_disponibles
        FROM livres
        WHERE supprime_le IS NULL
        ORDER BY titre;
    v_ligne   RECORD;
    v_total   int  := 0;
    v_premier text;
BEGIN
    OPEN c_livres;                       -- ouverture
    LOOP
        FETCH c_livres INTO v_ligne;     -- lecture d'une ligne
        EXIT WHEN NOT FOUND;             -- plus de ligne -> on sort
        v_total := v_total + v_ligne.exemplaires_disponibles;
        IF v_premier IS NULL THEN
            v_premier := v_ligne.titre;  -- mémorise le 1er titre (ordre alphabétique)
        END IF;
    END LOOP;
    CLOSE c_livres;                      -- fermeture (libère les ressources)

    RETURN format('%s exemplaires disponibles au total ; premier titre : « %s ».',
                  v_total, v_premier);
END;
$$;

SELECT demo_fn_inventaire_curseur() AS inventaire;


-- #############################################################################
-- 5) EXCEPTIONS + RAISE — gérer proprement les erreurs
-- -----------------------------------------------------------------------------
-- Un bloc « BEGIN … EXCEPTION WHEN … END » capture les erreurs (et crée un
-- SAVEPOINT implicite : cf. 05_transactions). RAISE émet des messages :
--   RAISE NOTICE   : information (visible côté client).
--   RAISE WARNING  : avertissement.
--   RAISE EXCEPTION: lève une erreur (avec ERRCODE personnalisable).
-- On capture un doublon (unique_violation) et un filet général (WHEN OTHERS).
-- #############################################################################
\echo '\n--- 5) EXCEPTION + RAISE NOTICE/WARNING/EXCEPTION ---'

-- Petite table cible avec une contrainte UNIQUE, pour provoquer le doublon.
DROP TABLE IF EXISTS demo_etiquettes;
CREATE TABLE demo_etiquettes (nom text PRIMARY KEY);

CREATE OR REPLACE FUNCTION demo_fn_ajouter_etiquette(p_nom text)
    RETURNS text
    LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO demo_etiquettes(nom) VALUES (p_nom);
    RAISE NOTICE 'Étiquette « % » ajoutée.', p_nom;
    RETURN 'ajoutée';
EXCEPTION
    WHEN unique_violation THEN
        -- Doublon : on ne plante pas, on prévient et on renvoie un statut.
        RAISE WARNING 'Étiquette « % » déjà présente : ignorée.', p_nom;
        RETURN 'doublon';
    WHEN OTHERS THEN
        -- Filet de sécurité : on récupère code et message via les variables spéciales.
        RAISE WARNING 'Erreur inattendue (%) : %', SQLSTATE, SQLERRM;
        RETURN 'erreur';
END;
$$;

SELECT demo_fn_ajouter_etiquette('nouveauté')  AS tentative_1;  -- ajoutée
SELECT demo_fn_ajouter_etiquette('nouveauté')  AS tentative_2;  -- doublon (WARNING)

-- RAISE EXCEPTION avec un code SQLSTATE personnalisé, puis capture ciblée.
CREATE OR REPLACE FUNCTION demo_fn_racine_carree(p_x numeric)
    RETURNS numeric
    LANGUAGE plpgsql
    IMMUTABLE
AS $$
BEGIN
    IF p_x < 0 THEN
        RAISE EXCEPTION 'Racine d''un nombre négatif impossible : %', p_x
            USING ERRCODE = 'data_exception';   -- code SQLSTATE symbolique
    END IF;
    RETURN sqrt(p_x);
END;
$$;

-- On appelle avec une valeur invalide : l'exception est levée PUIS rattrapée ici.
DO $$
DECLARE
    v numeric;
BEGIN
    v := demo_fn_racine_carree(-4);   -- va lever l'exception
    RAISE NOTICE 'Inattendu : %', v;
EXCEPTION
    WHEN data_exception THEN
        RAISE NOTICE 'Exception rattrapée comme prévu : %', SQLERRM;
END$$;

SELECT demo_fn_racine_carree(144) AS racine_de_144;  -- 12 (cas valide)


-- #############################################################################
-- 6) RETURNS SETOF — renvoyer PLUSIEURS lignes d'un type existant
-- -----------------------------------------------------------------------------
-- RETURNS SETOF <type> + RETURN QUERY : la fonction se comporte comme une table.
-- On l'interroge avec « SELECT * FROM fonction(args) ».
-- #############################################################################
\echo '\n--- 6) RETURNS SETOF ---'

CREATE OR REPLACE FUNCTION demo_fn_titres_categorie(p_categorie text)
    RETURNS SETOF text
    LANGUAGE plpgsql
    STABLE
AS $$
BEGIN
    RETURN QUERY
        -- Cast en text OBLIGATOIRE : livres.titre est varchar(255) et RETURNS SETOF
        -- text exige le type EXACT (PostgreSQL ne convertit pas implicitement ici).
        SELECT l.titre::text
        FROM livres l
        JOIN categories c ON c.id = l.categorie_id
        WHERE c.nom = p_categorie AND l.supprime_le IS NULL
        ORDER BY l.titre;
END;
$$;

SELECT * FROM demo_fn_titres_categorie('Science-fiction');


-- #############################################################################
-- 7) RETURNS TABLE — renvoyer des lignes STRUCTURÉES (colonnes nommées)
-- -----------------------------------------------------------------------------
-- RETURNS TABLE(col type, …) définit un schéma de sortie explicite. Plus lisible
-- que SETOF pour un résultat multi-colonnes. RETURN QUERY alimente la sortie.
-- #############################################################################
\echo '\n--- 7) RETURNS TABLE ---'

CREATE OR REPLACE FUNCTION demo_fn_stats_par_statut()
    RETURNS TABLE(statut text, nb bigint, penalite_totale numeric)
    LANGUAGE plpgsql
    STABLE
AS $$
BEGIN
    RETURN QUERY
        SELECT e.statut::text, count(*), COALESCE(sum(e.penalite), 0)
        FROM emprunts e
        GROUP BY e.statut
        ORDER BY e.statut::text;
END;
$$;

SELECT * FROM demo_fn_stats_par_statut();


-- #############################################################################
-- 8) TYPE COMPOSITE — renvoyer UNE ligne à plusieurs champs
-- -----------------------------------------------------------------------------
-- Un type composite regroupe plusieurs champs sous un seul type nommé, qu'une
-- fonction peut renvoyer (ou une variable contenir). Alternative typée au RECORD.
-- #############################################################################
\echo '\n--- 8) Type composite ---'

DROP TYPE IF EXISTS demo_livre_resume CASCADE;
CREATE TYPE demo_livre_resume AS (titre text, prix numeric, categorie text);

CREATE OR REPLACE FUNCTION demo_fn_livre_le_plus_cher()
    RETURNS demo_livre_resume
    LANGUAGE plpgsql
    STABLE
AS $$
DECLARE
    v_resultat demo_livre_resume;      -- variable du type composite
BEGIN
    SELECT l.titre, l.prix, c.nom
        INTO v_resultat.titre, v_resultat.prix, v_resultat.categorie
    FROM livres l
    JOIN categories c ON c.id = l.categorie_id
    WHERE l.supprime_le IS NULL
    ORDER BY l.prix DESC
    LIMIT 1;

    RETURN v_resultat;
END;
$$;

-- On peut « éclater » le composite en colonnes avec la notation (fonction()).*
SELECT (demo_fn_livre_le_plus_cher()).* ;


-- #############################################################################
-- 9) BONUS — une PROCÉDURE (CALL) avec boucle FOR
-- -----------------------------------------------------------------------------
-- Différence FONCTION vs PROCÉDURE : une procédure ne renvoie pas de valeur (on
-- l'invoque avec CALL) et peut piloter les transactions (COMMIT/ROLLBACK). Ici,
-- elle remplit une table demo_ à l'aide d'une boucle FOR entière.
-- #############################################################################
\echo '\n--- 9) Procédure (CALL) ---'

DROP TABLE IF EXISTS demo_seances;
CREATE TABLE demo_seances (numero int PRIMARY KEY, libelle text NOT NULL);

CREATE OR REPLACE PROCEDURE demo_pr_generer_seances(p_nb int)
    LANGUAGE plpgsql
AS $$
BEGIN
    FOR i IN 1..p_nb LOOP
        INSERT INTO demo_seances(numero, libelle)
        VALUES (i, 'Séance n°' || i);
    END LOOP;
    RAISE NOTICE '% séances générées.', p_nb;
END;
$$;

CALL demo_pr_generer_seances(5);
SELECT * FROM demo_seances ORDER BY numero;


-- #############################################################################
-- NETTOYAGE — on retire fonctions, procédure, type et tables demo_.
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
DROP FUNCTION  IF EXISTS demo_fn_categoriser_prix(numeric);
DROP FUNCTION  IF EXISTS demo_fn_factorielle(int);
DROP FUNCTION  IF EXISTS demo_fn_somme_while(int);
DROP FUNCTION  IF EXISTS demo_fn_puissances_sous(int);
DROP FUNCTION  IF EXISTS demo_fn_apercu_categories();
DROP FUNCTION  IF EXISTS demo_fn_inventaire_curseur();
DROP FUNCTION  IF EXISTS demo_fn_ajouter_etiquette(text);
DROP FUNCTION  IF EXISTS demo_fn_racine_carree(numeric);
DROP FUNCTION  IF EXISTS demo_fn_titres_categorie(text);
DROP FUNCTION  IF EXISTS demo_fn_stats_par_statut();
DROP FUNCTION  IF EXISTS demo_fn_livre_le_plus_cher();
DROP PROCEDURE IF EXISTS demo_pr_generer_seances(int);
DROP TYPE      IF EXISTS demo_livre_resume CASCADE;
DROP TABLE     IF EXISTS demo_etiquettes;
DROP TABLE     IF EXISTS demo_seances;

\echo '\n07_plpgsql.sql : terminé sans erreur.'
