-- =============================================================================
-- 09_cron.sql — Tâches planifiées avec pg_cron
-- -----------------------------------------------------------------------------
-- POURQUOI pg_cron ?
--
-- Contrairement à MariaDB (qui embarque un « Event Scheduler »), PostgreSQL n'a
-- PAS d'ordonnanceur de tâches intégré. La solution recommandée dans l'écosystème
-- est l'extension pg_cron : elle exécute des commandes SQL selon une planification
-- de type cron, DIRECTEMENT dans la base, sans service externe.
--
-- INSTALLATION : pg_cron doit être PRÉCHARGÉE au démarrage du serveur
-- (shared_preload_libraries = 'pg_cron' dans postgresql.conf, voir docker/postgres).
-- C'est pourquoi cette extension n'est pas activée avec les autres (00_extensions).
--
-- COMPARAISON (détaillée dans POSTGRESQL.md) :
--   - Event Scheduler MariaDB : intégré, mais propre à MariaDB.
--   - pg_cron                 : simple, au plus près des données. Limite : une
--                               instance (pas de coordination multi-nœuds native).
--   - cron Linux              : hors base, planifie n'importe quel programme.
--   - Kubernetes CronJob      : idéal en environnement conteneurisé/distribué.
--
-- Format de planification : « minute heure jour mois jour_semaine » (5 champs).
-- =============================================================================

-- Activation (nécessite le préchargement, voir ci-dessus).
CREATE EXTENSION IF NOT EXISTS pg_cron;

-- -----------------------------------------------------------------------------
-- Fonctions de maintenance appelées par les tâches (logique multi-instructions).
-- -----------------------------------------------------------------------------

-- fn_archiver_emprunts_anciens : déplace les emprunts rendus depuis plus d'un an
-- vers l'archive. Démontre une CTE « qui MODIFIE les données » (data-modifying
-- WITH) combinée à DELETE ... RETURNING : on supprime ET réinsère en UNE requête.
CREATE OR REPLACE FUNCTION fn_archiver_emprunts_anciens()
    RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
    v_nb integer;
BEGIN
    WITH deplaces AS (
        DELETE FROM emprunts
        WHERE statut = 'rendu'
          AND date_retour_effective < CURRENT_DATE - INTERVAL '1 year'
        RETURNING *
    )
    INSERT INTO emprunts_archive
        (id, uuid, utilisateur_id, livre_id, date_emprunt, date_retour_prevue,
         date_retour_effective, statut, penalite, cree_le)
    SELECT id, uuid, utilisateur_id, livre_id, date_emprunt, date_retour_prevue,
           date_retour_effective, statut::text, penalite, cree_le
    FROM deplaces;

    GET DIAGNOSTICS v_nb = ROW_COUNT;
    RETURN v_nb;
END;
$$;

-- fn_calculer_statistiques_quotidiennes : agrège les indicateurs du jour.
-- Démontre l'UPSERT « INSERT ... ON CONFLICT DO UPDATE » avec la pseudo-table
-- EXCLUDED (les valeurs qu'on aurait insérées).
CREATE OR REPLACE FUNCTION fn_calculer_statistiques_quotidiennes()
    RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO statistiques_quotidiennes
        (date_statistique, nb_emprunts_actifs, nb_emprunts_en_retard,
         nb_livres, nb_exemplaires_dispo, nb_utilisateurs_actifs)
    VALUES (
        CURRENT_DATE,
        (SELECT count(*) FROM emprunts WHERE statut IN ('en_cours', 'en_retard')),
        (SELECT count(*) FROM emprunts WHERE statut = 'en_retard'),
        (SELECT count(*) FROM livres WHERE supprime_le IS NULL),
        (SELECT COALESCE(sum(exemplaires_disponibles), 0) FROM livres WHERE supprime_le IS NULL),
        (SELECT count(*) FROM utilisateurs WHERE actif = true AND supprime_le IS NULL)
    )
    ON CONFLICT (date_statistique) DO UPDATE SET
        nb_emprunts_actifs     = EXCLUDED.nb_emprunts_actifs,
        nb_emprunts_en_retard  = EXCLUDED.nb_emprunts_en_retard,
        nb_livres              = EXCLUDED.nb_livres,
        nb_exemplaires_dispo   = EXCLUDED.nb_exemplaires_dispo,
        nb_utilisateurs_actifs = EXCLUDED.nb_utilisateurs_actifs;
END;
$$;

-- -----------------------------------------------------------------------------
-- PLANIFICATION DES TÂCHES (cron.schedule('nom', 'planning', 'commande SQL'))
-- Équivalents directs des EVENTS de la version MariaDB, plus deux tâches
-- spécifiques à PostgreSQL (rafraîchissement de vue matérialisée, VACUUM).
-- -----------------------------------------------------------------------------

-- 1) DÉTECTION DES RETARDS — chaque jour à 01h00.
SELECT cron.schedule('bib_marquer_retards', '0 1 * * *', $$
    UPDATE emprunts SET statut = 'en_retard'
    WHERE statut = 'en_cours' AND date_retour_prevue < CURRENT_DATE
$$);

-- 2) PURGE des jetons expirés/révoqués — toutes les heures.
SELECT cron.schedule('bib_purger_jetons', '0 * * * *', $$
    DELETE FROM jetons_rafraichissement WHERE expire_le < now() OR revoque = true
$$);

-- 3) ARCHIVAGE des emprunts anciens — chaque jour à 02h30.
SELECT cron.schedule('bib_archiver', '30 2 * * *', $$
    SELECT fn_archiver_emprunts_anciens()
$$);

-- 4) STATISTIQUES quotidiennes — chaque jour à 03h00.
SELECT cron.schedule('bib_statistiques', '0 3 * * *', $$
    SELECT fn_calculer_statistiques_quotidiennes()
$$);

-- 5) OPTIMISATION — rafraîchit la vue matérialisée de popularité, chaque jour à 03h15.
--    CONCURRENTLY : sans bloquer les lectures (requiert l'index unique, voir 06_vues).
SELECT cron.schedule('bib_refresh_stats', '15 3 * * *', $$
    REFRESH MATERIALIZED VIEW CONCURRENTLY vue_statistiques_livres
$$);

-- 6) NETTOYAGE du journal d'audit (rétention 90 jours) — le dimanche à 04h00.
SELECT cron.schedule('bib_nettoyer_audit', '0 4 * * 0', $$
    DELETE FROM journal_audit WHERE cree_le < now() - INTERVAL '90 days'
$$);

-- 7) MAINTENANCE — VACUUM ANALYZE hebdomadaire, le dimanche à 05h00.
--    VACUUM récupère l'espace des lignes mortes (MVCC) ; ANALYZE met à jour les
--    statistiques du planificateur. pg_cron exécute VACUUM hors transaction.
SELECT cron.schedule('bib_maintenance', '0 5 * * 0', $$
    VACUUM ANALYZE
$$);

-- Pour lister les tâches :   SELECT jobid, jobname, schedule, command FROM cron.job;
-- Pour voir l'historique :   SELECT * FROM cron.job_run_details ORDER BY start_time DESC;
-- Pour supprimer une tâche : SELECT cron.unschedule('bib_maintenance');
