-- =============================================================================
-- 07_procedures.sql — Procédures PL/pgSQL
-- -----------------------------------------------------------------------------
-- PROCÉDURE vs FONCTION en PostgreSQL :
--   - une FONCTION renvoie une valeur et s'utilise dans une expression SQL ;
--   - une PROCÉDURE s'appelle avec CALL, peut renvoyer plusieurs valeurs via des
--     paramètres INOUT, et peut (dans certains contextes) piloter la transaction.
--
-- APPEL DEPUIS Go (repository) : « CALL proc($1, $2, NULL, NULL) ». Les arguments
-- correspondant aux paramètres INOUT sont fournis (NULL en entrée) et la ligne de
-- résultat contient leurs valeurs de sortie. Un simple QueryRow().Scan() récupère
-- tout — bien plus simple que les variables de session @ de MariaDB.
--
-- GESTION DES ERREURS : le bloc « EXCEPTION WHEN ... » est l'équivalent du HANDLER
-- de MariaDB. En PL/pgSQL, chaque bloc BEGIN...EXCEPTION crée un SAVEPOINT
-- implicite : si une exception survient, les modifications du bloc sont ANNULÉES
-- automatiquement. On s'appuie sur cette atomicité plutôt que sur un
-- START TRANSACTION/COMMIT explicite (incompatible avec un CALL en autocommit).
-- =============================================================================

-- -----------------------------------------------------------------------------
-- pr_emprunter_livre : enregistre un emprunt de manière ATOMIQUE et sûre.
--
-- Comportement, codes de retour et messages STRICTEMENT IDENTIQUES à la version
-- MariaDB (parité) :
--   0 = succès ; 1 = livre introuvable ; 2 = utilisateur introuvable/inactif ;
--   3 = indisponible ; 4 = quota atteint ; 99 = erreur inattendue.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE PROCEDURE pr_emprunter_livre(
    p_utilisateur_uuid uuid,
    p_livre_uuid       uuid,
    p_duree_jours      integer,
    INOUT p_emprunt_uuid  uuid,
    INOUT p_code_resultat integer,
    INOUT p_message       text
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_utilisateur_id bigint;
    v_livre_id       bigint;
    v_dispo          integer;
    v_nb_actifs      integer;
    v_quota_max      constant integer := 5;  -- quota d'emprunts simultanés (paramètre métier)
    v_duree          integer;
    v_uuid           uuid;
BEGIN
    p_emprunt_uuid  := NULL;
    p_code_resultat := 0;

    -- Normalisation de la durée (bornes de sécurité).
    v_duree := COALESCE(NULLIF(p_duree_jours, 0), 14);
    IF v_duree < 1  THEN v_duree := 14; END IF;
    IF v_duree > 90 THEN v_duree := 90; END IF;

    -- 1) L'utilisateur existe-t-il et est-il actif ?
    SELECT id INTO v_utilisateur_id
        FROM utilisateurs
        WHERE uuid = p_utilisateur_uuid AND supprime_le IS NULL AND actif = true;
    IF v_utilisateur_id IS NULL THEN
        p_code_resultat := 2;
        p_message := 'Utilisateur introuvable ou inactif.';
        RETURN;
    END IF;

    -- 2) Verrouillage du livre (FOR UPDATE) : sérialise les emprunts concurrents
    --    du dernier exemplaire (verrouillage pessimiste).
    SELECT id, exemplaires_disponibles
        INTO v_livre_id, v_dispo
        FROM livres
        WHERE uuid = p_livre_uuid AND supprime_le IS NULL
        FOR UPDATE;
    IF v_livre_id IS NULL THEN
        p_code_resultat := 1;
        p_message := 'Livre introuvable.';
        RETURN;
    END IF;
    IF v_dispo <= 0 THEN
        p_code_resultat := 3;
        p_message := 'Aucun exemplaire disponible actuellement.';
        RETURN;
    END IF;

    -- 3) Respect du quota d'emprunts simultanés.
    v_nb_actifs := fn_nb_emprunts_actifs(v_utilisateur_id);
    IF v_nb_actifs >= v_quota_max THEN
        p_code_resultat := 4;
        p_message := format('Quota d''emprunts simultanés atteint (%s).', v_quota_max);
        RETURN;
    END IF;

    -- 4) Création de l'emprunt et décrément du stock (2 tables, atomique).
    --    On calcule la date de retour avec un INTERVAL (démonstration du type).
    v_uuid := gen_random_uuid();
    INSERT INTO emprunts (uuid, utilisateur_id, livre_id, date_emprunt, date_retour_prevue, statut)
        VALUES (v_uuid, v_utilisateur_id, v_livre_id, CURRENT_DATE,
                (CURRENT_DATE + v_duree * INTERVAL '1 day')::date, 'en_cours');

    UPDATE livres
        SET exemplaires_disponibles = exemplaires_disponibles - 1
        WHERE id = v_livre_id;

    p_emprunt_uuid  := v_uuid;
    p_code_resultat := 0;
    p_message       := 'Emprunt enregistré avec succès.';

EXCEPTION
    -- Filet de sécurité : toute erreur SQL imprévue annule les modifications du
    -- bloc (savepoint implicite) et renvoie un code générique.
    WHEN OTHERS THEN
        p_emprunt_uuid  := NULL;
        p_code_resultat := 99;
        p_message       := 'Erreur inattendue : l''emprunt a été annulé.';
END;
$$;

COMMENT ON PROCEDURE pr_emprunter_livre(uuid, uuid, integer, uuid, integer, text)
    IS 'Emprunt atomique : vérifie disponibilité et quota, crée l''emprunt, décrémente le stock.';

-- -----------------------------------------------------------------------------
-- pr_statistiques_utilisateur : renvoie plusieurs indicateurs via des INOUT.
-- Utilise COUNT(*) FILTER (WHERE ...) : un agrégat conditionnel très lisible.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE PROCEDURE pr_statistiques_utilisateur(
    p_utilisateur_uuid uuid,
    INOUT p_nb_total        integer,
    INOUT p_nb_en_cours     integer,
    INOUT p_nb_en_retard    integer,
    INOUT p_total_penalites double precision
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_id bigint;
BEGIN
    SELECT id INTO v_id FROM utilisateurs WHERE uuid = p_utilisateur_uuid;

    SELECT
        count(*),
        count(*) FILTER (WHERE statut = 'en_cours'),
        count(*) FILTER (WHERE statut = 'en_retard'),
        COALESCE(sum(penalite), 0)::float8
    INTO p_nb_total, p_nb_en_cours, p_nb_en_retard, p_total_penalites
    FROM emprunts
    WHERE utilisateur_id = v_id;
END;
$$;

COMMENT ON PROCEDURE pr_statistiques_utilisateur(uuid, integer, integer, integer, double precision)
    IS 'Indicateurs d''emprunt d''un utilisateur (plusieurs valeurs via INOUT).';

-- -----------------------------------------------------------------------------
-- pr_ajuster_disponibilite : exemple minimal de paramètre INOUT « pur ».
--   CALL pr_ajuster_disponibilite(3, -1);  -- renvoie 2
-- -----------------------------------------------------------------------------
CREATE OR REPLACE PROCEDURE pr_ajuster_disponibilite(
    INOUT p_disponibles integer,
    p_delta integer
)
LANGUAGE plpgsql
AS $$
BEGIN
    p_disponibles := GREATEST(p_disponibles + p_delta, 0);
END;
$$;

COMMENT ON PROCEDURE pr_ajuster_disponibilite(integer, integer)
    IS 'Exemple de paramètre INOUT : ajuste un stock en le bornant à zéro.';
