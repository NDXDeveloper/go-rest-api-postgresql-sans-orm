-- =============================================================================
-- 03_tables.sql — Création de toutes les tables du domaine « Bibliothèque »
-- -----------------------------------------------------------------------------
-- MODÈLE DE DONNÉES (identique, fonctionnellement, à la version MariaDB) :
--
--   auteurs 1 ───< livres >─── 1 categories
--                    │
--   utilisateurs 1 ──< emprunts >── 1 livres
--   utilisateurs 1 ──< jetons_rafraichissement
--   (tables techniques : journal_audit, emprunts_archive, statistiques_quotidiennes)
--
-- CONVENTIONS (parité avec le projet MariaDB) :
--   - Clé primaire technique « id » (jamais exposée à l'API).
--   - Identifiant public « uuid » (exposé à la place de l'id : anti-énumération).
--   - Horodatage « cree_le » / « modifie_le ».
--
-- CHOIX DE TYPES PROPRES À POSTGRESQL :
--   - GENERATED ALWAYS AS IDENTITY : la façon MODERNE et recommandée de déclarer
--     une clé auto-incrémentée (norme SQL), préférable au vieux SERIAL. « ALWAYS »
--     interdit d'imposer une valeur (sauf OVERRIDING SYSTEM VALUE, utilisé au seed).
--   - uuid : type natif 128 bits (plus compact et typé qu'un CHAR(36)).
--   - timestamptz : horodatage AVEC fuseau, stocké en UTC (contre les ambiguïtés).
--   - numeric(8,2) : décimal EXACT (idéal pour la monnaie, aucune erreur d'arrondi).
--   - role_utilisateur / statut_emprunt : nos types ENUM.
--   - courriel / isbn13 : nos DOMAIN (validation intégrée).
--   - jsonb : JSON binaire indexable (pour le journal d'audit).
-- =============================================================================

-- -----------------------------------------------------------------------------
-- TABLE utilisateurs
-- -----------------------------------------------------------------------------
CREATE TABLE utilisateurs (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uuid              uuid NOT NULL DEFAULT gen_random_uuid(),
    email             courriel NOT NULL,
    mot_de_passe_hash text NOT NULL,
    nom               varchar(100) NOT NULL,
    prenom            varchar(100) NOT NULL,
    role              role_utilisateur NOT NULL DEFAULT 'membre',
    actif             boolean NOT NULL DEFAULT true,
    cree_le           timestamptz NOT NULL DEFAULT now(),
    modifie_le        timestamptz NOT NULL DEFAULT now(),
    supprime_le       timestamptz,
    CONSTRAINT uq_utilisateurs_uuid  UNIQUE (uuid),
    CONSTRAINT uq_utilisateurs_email UNIQUE (email)
);

COMMENT ON TABLE  utilisateurs IS 'Comptes applicatifs (authentification, rôles, suppression logique).';
COMMENT ON COLUMN utilisateurs.uuid IS 'Identifiant public non devinable (exposé à la place de l''id).';
COMMENT ON COLUMN utilisateurs.mot_de_passe_hash IS 'Haché bcrypt (jamais le mot de passe en clair).';
COMMENT ON COLUMN utilisateurs.supprime_le IS 'Non NULL => compte supprimé logiquement.';

-- -----------------------------------------------------------------------------
-- TABLE categories
-- -----------------------------------------------------------------------------
CREATE TABLE categories (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uuid        uuid NOT NULL DEFAULT gen_random_uuid(),
    nom         varchar(100) NOT NULL,
    description varchar(500) NOT NULL DEFAULT '',
    cree_le     timestamptz NOT NULL DEFAULT now(),
    modifie_le  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_categories_uuid UNIQUE (uuid),
    CONSTRAINT uq_categories_nom  UNIQUE (nom)
);

COMMENT ON TABLE categories IS 'Catégories thématiques des livres.';

-- -----------------------------------------------------------------------------
-- TABLE auteurs
-- -----------------------------------------------------------------------------
CREATE TABLE auteurs (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uuid           uuid NOT NULL DEFAULT gen_random_uuid(),
    nom            varchar(100) NOT NULL,
    prenom         varchar(100) NOT NULL DEFAULT '',
    nationalite    varchar(100) NOT NULL DEFAULT '',
    date_naissance date,
    biographie     text,
    cree_le        timestamptz NOT NULL DEFAULT now(),
    modifie_le     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_auteurs_uuid UNIQUE (uuid)
);

COMMENT ON TABLE auteurs IS 'Auteurs des ouvrages.';

-- -----------------------------------------------------------------------------
-- TABLE livres — porte les clés étrangères et la gestion du stock.
-- -----------------------------------------------------------------------------
CREATE TABLE livres (
    id                      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uuid                    uuid NOT NULL DEFAULT gen_random_uuid(),
    titre                   varchar(255) NOT NULL,
    isbn                    isbn13 NOT NULL,
    auteur_id               bigint NOT NULL,
    categorie_id            bigint NOT NULL,
    annee_publication       smallint NOT NULL,
    nombre_exemplaires      integer NOT NULL DEFAULT 1,
    exemplaires_disponibles integer NOT NULL DEFAULT 1,
    resume                  text,
    prix                    numeric(8,2) NOT NULL DEFAULT 0,
    langue                  varchar(50) NOT NULL DEFAULT 'français',
    cree_le                 timestamptz NOT NULL DEFAULT now(),
    modifie_le              timestamptz NOT NULL DEFAULT now(),
    supprime_le             timestamptz,
    CONSTRAINT uq_livres_uuid UNIQUE (uuid),
    CONSTRAINT uq_livres_isbn UNIQUE (isbn),
    -- Clés étrangères : ON DELETE RESTRICT protège contre les livres orphelins.
    CONSTRAINT fk_livres_auteur
        FOREIGN KEY (auteur_id)    REFERENCES auteurs(id)    ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_livres_categorie
        FOREIGN KEY (categorie_id) REFERENCES categories(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    -- Contraintes CHECK : invariants métier garantis par la base.
    CONSTRAINT chk_livres_stock        CHECK (exemplaires_disponibles <= nombre_exemplaires),
    CONSTRAINT chk_livres_stock_positif CHECK (exemplaires_disponibles >= 0 AND nombre_exemplaires >= 0),
    CONSTRAINT chk_livres_annee        CHECK (annee_publication BETWEEN 1400 AND 2200),
    CONSTRAINT chk_livres_prix         CHECK (prix >= 0)
);

COMMENT ON TABLE  livres IS 'Catalogue des ouvrages et gestion du stock.';
COMMENT ON COLUMN livres.exemplaires_disponibles IS 'Diminue à l''emprunt, augmente au retour ; borné par CHECK.';

-- -----------------------------------------------------------------------------
-- TABLE emprunts — cœur métier.
-- -----------------------------------------------------------------------------
CREATE TABLE emprunts (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uuid                  uuid NOT NULL DEFAULT gen_random_uuid(),
    utilisateur_id        bigint NOT NULL,
    livre_id              bigint NOT NULL,
    date_emprunt          date NOT NULL DEFAULT CURRENT_DATE,
    -- NULLable À DESSEIN : si l'appelant ne la fournit pas, un trigger BEFORE INSERT
    -- la calcule (date_emprunt + 14 jours). Voir 08_triggers.
    date_retour_prevue    date,
    date_retour_effective date,
    statut                statut_emprunt NOT NULL DEFAULT 'en_cours',
    penalite              numeric(8,2) NOT NULL DEFAULT 0,
    cree_le               timestamptz NOT NULL DEFAULT now(),
    modifie_le            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_emprunts_uuid UNIQUE (uuid),
    CONSTRAINT fk_emprunts_utilisateur
        FOREIGN KEY (utilisateur_id) REFERENCES utilisateurs(id) ON DELETE CASCADE  ON UPDATE CASCADE,
    CONSTRAINT fk_emprunts_livre
        FOREIGN KEY (livre_id)       REFERENCES livres(id)       ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT chk_emprunts_penalite CHECK (penalite >= 0),
    CONSTRAINT chk_emprunts_dates    CHECK (date_retour_prevue IS NULL OR date_retour_prevue >= date_emprunt)
);

COMMENT ON TABLE emprunts IS 'Prêts de livres aux utilisateurs.';

-- -----------------------------------------------------------------------------
-- TABLE jetons_rafraichissement — refresh tokens (hachés).
-- -----------------------------------------------------------------------------
CREATE TABLE jetons_rafraichissement (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    utilisateur_id bigint NOT NULL,
    jeton_hash     char(64) NOT NULL,
    expire_le      timestamptz NOT NULL,
    revoque        boolean NOT NULL DEFAULT false,
    cree_le        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_jetons_hash UNIQUE (jeton_hash),
    CONSTRAINT fk_jetons_utilisateur
        FOREIGN KEY (utilisateur_id) REFERENCES utilisateurs(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE  jetons_rafraichissement IS 'Jetons de rafraîchissement (hachés SHA-256) pour renouveler les JWT.';
COMMENT ON COLUMN jetons_rafraichissement.jeton_hash IS 'SHA-256 hexadécimal du jeton (jamais le jeton en clair).';

-- -----------------------------------------------------------------------------
-- TABLE journal_audit — alimentée UNIQUEMENT par des triggers. Utilise JSONB.
-- -----------------------------------------------------------------------------
CREATE TABLE journal_audit (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    table_concernee    varchar(64) NOT NULL,
    operation          operation_audit NOT NULL,
    cle_enregistrement bigint,
    -- JSONB : JSON stocké en binaire, indexable (GIN) et interrogeable avec les
    -- opérateurs ->, ->>, @>, etc. On y met une photo des valeurs avant/après.
    anciennes_valeurs  jsonb,
    nouvelles_valeurs  jsonb,
    acteur_sql         varchar(128) NOT NULL DEFAULT current_user,
    cree_le            timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE  journal_audit IS 'Journal d''audit (JSONB) alimenté par les triggers.';
COMMENT ON COLUMN journal_audit.nouvelles_valeurs IS 'Photo JSONB des nouvelles valeurs (indexable en GIN).';

-- -----------------------------------------------------------------------------
-- TABLE emprunts_archive — reçoit les emprunts anciens (déplacés par pg_cron).
-- -----------------------------------------------------------------------------
CREATE TABLE emprunts_archive (
    id                    bigint PRIMARY KEY,
    uuid                  uuid NOT NULL,
    utilisateur_id        bigint NOT NULL,
    livre_id              bigint NOT NULL,
    date_emprunt          date NOT NULL,
    date_retour_prevue    date,
    date_retour_effective date,
    statut                varchar(20) NOT NULL,
    penalite              numeric(8,2) NOT NULL DEFAULT 0,
    cree_le               timestamptz NOT NULL,
    archive_le            timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE emprunts_archive IS 'Archive des emprunts anciens (déplacés par une tâche pg_cron).';

-- -----------------------------------------------------------------------------
-- TABLE statistiques_quotidiennes — instantané journalier calculé par pg_cron.
-- -----------------------------------------------------------------------------
CREATE TABLE statistiques_quotidiennes (
    id                     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    date_statistique       date NOT NULL,
    nb_emprunts_actifs     integer NOT NULL DEFAULT 0,
    nb_emprunts_en_retard  integer NOT NULL DEFAULT 0,
    nb_livres              integer NOT NULL DEFAULT 0,
    nb_exemplaires_dispo   integer NOT NULL DEFAULT 0,
    nb_utilisateurs_actifs integer NOT NULL DEFAULT 0,
    cree_le                timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_stats_date UNIQUE (date_statistique)
);

COMMENT ON TABLE statistiques_quotidiennes IS 'Statistiques agrégées calculées quotidiennement par pg_cron.';
