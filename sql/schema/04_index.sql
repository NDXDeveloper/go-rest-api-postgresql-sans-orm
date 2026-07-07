-- =============================================================================
-- 04_index.sql — Index (l'un des grands terrains de jeu de PostgreSQL)
-- -----------------------------------------------------------------------------
-- Là où MariaDB propose essentiellement des index B-tree (et FULLTEXT),
-- PostgreSQL offre une PALETTE riche de méthodes d'index, chacune adaptée à un
-- usage. Ce fichier crée les index RÉELLEMENT UTILES au projet, en illustrant :
--   - B-tree      : le cas général (égalité, ordre, plages) ;
--   - GIN + pg_trgm : recherche « ILIKE '%mot%' » rapide (impossible en B-tree) ;
--   - GIN + jsonb : recherche à l'intérieur de documents JSONB ;
--   - BRIN        : index minuscule pour colonnes corrélées à l'ordre physique ;
--   - partiel     : n'indexe qu'un sous-ensemble de lignes (WHERE) ;
--   - couvrant    : INCLUDE des colonnes pour un « index-only scan ».
--
-- Rappel : un index accélère les LECTURES mais coûte en écriture et en espace.
-- On vérifie l'usage réel avec «  EXPLAIN (ANALYZE) SELECT ... ».
-- (Hash et GiST, moins utiles ici, sont illustrés dans sql/demos/.)
-- =============================================================================

-- -----------------------------------------------------------------------------
-- utilisateurs
-- -----------------------------------------------------------------------------
-- B-tree simple sur le rôle (filtrage fréquent).
CREATE INDEX idx_utilisateurs_role ON utilisateurs (role);
-- Index PARTIEL : on n'indexe que les comptes actifs (non supprimés). Plus petit
-- et plus rapide, puisque les requêtes ne portent que sur ces lignes-là.
CREATE INDEX idx_utilisateurs_actifs ON utilisateurs (cree_le) WHERE supprime_le IS NULL;

-- -----------------------------------------------------------------------------
-- auteurs
-- -----------------------------------------------------------------------------
-- Index MULTICOLONNE (nom, prénom) : sert les tris et filtres par nom complet.
CREATE INDEX idx_auteurs_nom_prenom ON auteurs (nom, prenom);
-- Index GIN + trigrammes : accélère « nom ILIKE '%...%' » (recherche floue).
CREATE INDEX idx_auteurs_nom_trgm ON auteurs USING gin (nom gin_trgm_ops);

-- -----------------------------------------------------------------------------
-- livres
-- -----------------------------------------------------------------------------
-- B-tree sur les clés étrangères (jointures et filtres).
CREATE INDEX idx_livres_auteur    ON livres (auteur_id);
CREATE INDEX idx_livres_categorie ON livres (categorie_id);
-- Index GIN + trigrammes sur le titre : c'est LUI qui rend rapide la recherche
-- « titre ILIKE '%terme%' » exposée par l'API (un B-tree ne peut pas l'optimiser
-- à cause du joker en tête). Démonstration concrète de l'intérêt de pg_trgm.
CREATE INDEX idx_livres_titre_trgm ON livres USING gin (titre gin_trgm_ops);
-- Index COUVRANT + PARTIEL : lister les livres d'une catégorie avec leur titre et
-- prix sans lire la table (index-only scan), uniquement pour les livres actifs.
CREATE INDEX idx_livres_categorie_couvrant
    ON livres (categorie_id) INCLUDE (titre, prix)
    WHERE supprime_le IS NULL;

-- -----------------------------------------------------------------------------
-- emprunts
-- -----------------------------------------------------------------------------
CREATE INDEX idx_emprunts_utilisateur   ON emprunts (utilisateur_id);
CREATE INDEX idx_emprunts_livre         ON emprunts (livre_id);
-- Index MULTICOLONNE : requête très fréquente « les emprunts d'un membre par statut ».
CREATE INDEX idx_emprunts_util_statut   ON emprunts (utilisateur_id, statut);
-- Index PARTIEL sur les emprunts ACTIFS (pour la détection des retards).
CREATE INDEX idx_emprunts_actifs
    ON emprunts (date_retour_prevue)
    WHERE statut IN ('en_cours', 'en_retard');

-- -----------------------------------------------------------------------------
-- journal_audit — deux méthodes spécifiques.
-- -----------------------------------------------------------------------------
-- Index GIN sur JSONB : permet des recherches DANS le document, par ex.
--   SELECT * FROM journal_audit WHERE nouvelles_valeurs @> '{"role":"admin"}';
CREATE INDEX idx_audit_nouvelles_gin ON journal_audit USING gin (nouvelles_valeurs);
-- Index BRIN sur l'horodatage : la table ne fait que grandir dans l'ordre du
-- temps, donc cree_le est fortement corrélé à l'ordre physique des blocs. BRIN
-- résume chaque plage de blocs et occupe une place MINUSCULE (idéal pour de très
-- grosses tables append-only, là où un B-tree serait énorme).
CREATE INDEX idx_audit_cree_le_brin ON journal_audit USING brin (cree_le);
-- B-tree classique pour filtrer par table auditée.
CREATE INDEX idx_audit_table ON journal_audit (table_concernee);

-- -----------------------------------------------------------------------------
-- jetons_rafraichissement
-- -----------------------------------------------------------------------------
CREATE INDEX idx_jetons_utilisateur ON jetons_rafraichissement (utilisateur_id);
CREATE INDEX idx_jetons_expire      ON jetons_rafraichissement (expire_le);
