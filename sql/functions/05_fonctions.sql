-- =============================================================================
-- 05_fonctions.sql — Fonctions PL/pgSQL
-- -----------------------------------------------------------------------------
-- PL/pgSQL est le langage procédural natif de PostgreSQL (équivalent du langage
-- des routines stockées de MariaDB, mais bien plus riche : types composites,
-- gestion fine des exceptions, curseurs, etc.).
--
-- INTÉRÊT DES FONCTIONS : encapsuler un calcul RÉUTILISABLE, appelé aussi bien
-- depuis une requête SQL que depuis une vue, une autre fonction ou l'application.
-- On écrit la règle UNE fois → cohérence garantie.
--
-- MOTS-CLÉS DE VOLATILITÉ (aident l'optimiseur) :
--   - IMMUTABLE : même entrée => même sortie, ne lit pas la base (ex. calcul pur).
--   - STABLE    : ne modifie pas la base, résultat stable dans une même requête
--                 (nos fonctions de lecture).
--   - VOLATILE  : peut tout faire (défaut).
--
-- DIFFÉRENCE AVEC MariaDB : pas de « DELIMITER » à manipuler. On délimite le corps
-- avec le « dollar-quoting » ($$ ... $$), bien plus lisible.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- fn_est_disponible : un livre a-t-il au moins un exemplaire disponible ?
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fn_est_disponible(p_livre_id bigint)
    RETURNS boolean
    LANGUAGE plpgsql
    STABLE
AS $$
DECLARE
    v_dispo integer;
BEGIN
    SELECT exemplaires_disponibles
        INTO v_dispo
        FROM livres
        WHERE id = p_livre_id AND supprime_le IS NULL;

    -- COALESCE : si le livre n'existe pas, v_dispo est NULL -> on renvoie FALSE.
    RETURN COALESCE(v_dispo, 0) > 0;
END;
$$;

COMMENT ON FUNCTION fn_est_disponible(bigint) IS 'Renvoie TRUE si le livre a au moins un exemplaire disponible.';

-- -----------------------------------------------------------------------------
-- fn_calculer_penalite : montant dû pour un retour, selon le retard.
--
-- Règle métier : 0,50 € par jour de retard entamé (identique à la version MariaDB).
-- Si p_date_effective est NULL (livre pas encore rendu), on calcule « à aujourd'hui ».
--
-- Astuce PostgreSQL : la soustraction de deux DATE renvoie directement un entier
-- (nombre de jours) — plus simple que DATEDIFF.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fn_calculer_penalite(p_date_prevue date, p_date_effective date)
    RETURNS numeric
    LANGUAGE plpgsql
    STABLE
AS $$
DECLARE
    c_tarif_jour   constant numeric := 0.50; -- paramètre métier, centralisé ici
    v_reference    date;
    v_jours_retard integer;
BEGIN
    v_reference := COALESCE(p_date_effective, CURRENT_DATE);
    -- GREATEST(..., 0) : pas de « bonus » si le livre est rendu en avance.
    v_jours_retard := GREATEST(v_reference - p_date_prevue, 0);
    RETURN v_jours_retard * c_tarif_jour;
END;
$$;

COMMENT ON FUNCTION fn_calculer_penalite(date, date) IS 'Pénalité de retard : 0,50 € par jour.';

-- -----------------------------------------------------------------------------
-- fn_nb_emprunts_actifs : nombre d'emprunts en cours ou en retard d'un membre.
-- Sert à faire respecter un QUOTA d'emprunts simultanés (voir pr_emprunter_livre).
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fn_nb_emprunts_actifs(p_utilisateur_id bigint)
    RETURNS integer
    LANGUAGE plpgsql
    STABLE
AS $$
DECLARE
    v_nb integer;
BEGIN
    SELECT COUNT(*)
        INTO v_nb
        FROM emprunts
        WHERE utilisateur_id = p_utilisateur_id
          AND statut IN ('en_cours', 'en_retard');

    RETURN v_nb;
END;
$$;

COMMENT ON FUNCTION fn_nb_emprunts_actifs(bigint) IS 'Nombre d''emprunts actifs (en cours ou en retard) d''un utilisateur.';
