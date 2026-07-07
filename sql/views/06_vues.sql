-- =============================================================================
-- 06_vues.sql — Vues simples et vue MATÉRIALISÉE
-- -----------------------------------------------------------------------------
-- Une VUE est une requête nommée, recalculée à chaque interrogation. Une VUE
-- MATÉRIALISÉE, elle, STOCKE physiquement le résultat : lecture instantanée, mais
-- il faut la RAFRAÎCHIR (REFRESH) pour la remettre à jour. C'est une fonctionnalité
-- PostgreSQL absente de MariaDB, idéale pour des agrégats coûteux consultés souvent.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- vue_livres_details : le catalogue « prêt à afficher » (jointures + disponibilité).
-- C'est la vue effectivement interrogée par l'application (repository livres).
--
-- prix est casté en « double precision » pour un scan direct côté Go (le stockage
-- reste en numeric). L'ordre des colonnes correspond au scan du repository.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE VIEW vue_livres_details AS
SELECT
    l.id,
    l.uuid,
    l.titre,
    l.isbn,
    l.annee_publication,
    l.nombre_exemplaires,
    l.exemplaires_disponibles,
    fn_est_disponible(l.id)              AS disponible,
    l.prix::float8                       AS prix,
    l.langue,
    l.resume,
    a.uuid                               AS auteur_uuid,
    concat_ws(' ', a.prenom, a.nom)      AS auteur_nom_complet,
    c.uuid                               AS categorie_uuid,
    c.nom                                AS categorie_nom,
    l.cree_le,
    l.modifie_le
FROM livres l
    JOIN auteurs    a ON a.id = l.auteur_id
    JOIN categories c ON c.id = l.categorie_id
WHERE l.supprime_le IS NULL;

COMMENT ON VIEW vue_livres_details IS 'Catalogue enrichi (auteur, catégorie, disponibilité).';

-- -----------------------------------------------------------------------------
-- vue_emprunts_en_cours : emprunts non rendus, avec noms.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE VIEW vue_emprunts_en_cours AS
SELECT
    e.id,
    e.uuid,
    e.date_emprunt,
    e.date_retour_prevue,
    e.statut,
    u.uuid                          AS utilisateur_uuid,
    concat_ws(' ', u.prenom, u.nom) AS utilisateur_nom_complet,
    u.email                         AS utilisateur_email,
    l.uuid                          AS livre_uuid,
    l.titre                         AS livre_titre
FROM emprunts e
    JOIN utilisateurs u ON u.id = e.utilisateur_id
    JOIN livres       l ON l.id = e.livre_id
WHERE e.statut IN ('en_cours', 'en_retard');

-- -----------------------------------------------------------------------------
-- vue_emprunts_en_retard : emprunts en retard, avec pénalité courante calculée.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE VIEW vue_emprunts_en_retard AS
SELECT
    e.id,
    e.uuid,
    e.date_emprunt,
    e.date_retour_prevue,
    (CURRENT_DATE - e.date_retour_prevue)               AS jours_de_retard,
    fn_calculer_penalite(e.date_retour_prevue, NULL)::float8 AS penalite_courante,
    u.uuid                          AS utilisateur_uuid,
    concat_ws(' ', u.prenom, u.nom) AS utilisateur_nom_complet,
    u.email                         AS utilisateur_email,
    l.uuid                          AS livre_uuid,
    l.titre                         AS livre_titre
FROM emprunts e
    JOIN utilisateurs u ON u.id = e.utilisateur_id
    JOIN livres       l ON l.id = e.livre_id
WHERE e.statut = 'en_retard'
   OR (e.statut = 'en_cours' AND e.date_retour_prevue < CURRENT_DATE);

-- -----------------------------------------------------------------------------
-- vue_statistiques_livres : VUE MATÉRIALISÉE de popularité.
--
-- Démonstrations SQL avancées combinées :
--   - COUNT(...) FILTER (WHERE ...)   : agrégat conditionnel élégant ;
--   - rank() OVER (ORDER BY ...)      : FONCTION FENÊTRE (classement) ;
--   - MATERIALIZED VIEW               : résultat stocké, à rafraîchir.
--
-- On la rafraîchit périodiquement via pg_cron (voir sql/cron/). L'index UNIQUE
-- ci-dessous autorise « REFRESH MATERIALIZED VIEW CONCURRENTLY » (rafraîchissement
-- sans bloquer les lectures).
-- -----------------------------------------------------------------------------
CREATE MATERIALIZED VIEW vue_statistiques_livres AS
SELECT
    l.id,
    l.uuid,
    l.titre,
    count(e.id)                                                       AS nombre_emprunts_total,
    count(e.id) FILTER (WHERE e.statut IN ('en_cours', 'en_retard'))  AS nombre_emprunts_actifs,
    rank() OVER (ORDER BY count(e.id) DESC)                           AS rang_popularite
FROM livres l
    LEFT JOIN emprunts e ON e.livre_id = l.id
WHERE l.supprime_le IS NULL
GROUP BY l.id, l.uuid, l.titre
WITH DATA;

-- Index UNIQUE requis pour le REFRESH ... CONCURRENTLY.
CREATE UNIQUE INDEX idx_vue_stats_livres_id ON vue_statistiques_livres (id);

COMMENT ON MATERIALIZED VIEW vue_statistiques_livres IS 'Popularité des livres (agrégat matérialisé, rafraîchi par pg_cron).';
