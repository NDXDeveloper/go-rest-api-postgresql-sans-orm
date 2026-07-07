-- =============================================================================
-- 02_types.sql — Types personnalisés : ENUM et DOMAIN
-- -----------------------------------------------------------------------------
-- PostgreSQL permet de définir ses PROPRES TYPES, gage de lisibilité et
-- d'intégrité. On en utilise deux familles :
--
--   - ENUM   : une liste fermée de valeurs textuelles (comme l'ENUM de MariaDB,
--              mais ici c'est un vrai type réutilisable et ordonné).
--   - DOMAIN : un type de base (text, int...) ASSORTI d'une contrainte CHECK. La
--              règle est ainsi définie UNE fois et réutilisée partout où le type
--              est employé. MariaDB ne possède pas d'équivalent aussi élégant.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- ENUM — rôles applicatifs.
-- L'ordre de déclaration définit l'ordre de tri du type (admin < bibliothecaire
-- < membre). Ajouter une valeur plus tard se fait avec « ALTER TYPE ... ADD VALUE ».
-- -----------------------------------------------------------------------------
CREATE TYPE role_utilisateur AS ENUM ('admin', 'bibliothecaire', 'membre');

-- -----------------------------------------------------------------------------
-- ENUM — statut d'un emprunt.
-- -----------------------------------------------------------------------------
CREATE TYPE statut_emprunt AS ENUM ('en_cours', 'rendu', 'en_retard');

-- -----------------------------------------------------------------------------
-- ENUM — type d'opération pour le journal d'audit.
-- -----------------------------------------------------------------------------
CREATE TYPE operation_audit AS ENUM ('INSERT', 'UPDATE', 'DELETE');

-- -----------------------------------------------------------------------------
-- DOMAIN — adresse e-mail.
-- Un DOMAIN « courriel » = du text validé par une expression régulière. Toute
-- colonne déclarée de ce type hérite AUTOMATIQUEMENT de la validation : on ne
-- peut pas y insérer une valeur mal formée. C'est une défense en profondeur qui
-- complète la validation applicative (côté Go).
--
-- Le motif : au moins un caractère, un « @ », un domaine, un point, une extension.
-- « ~* » signifie « correspond à la regex, sans tenir compte de la casse ».
-- -----------------------------------------------------------------------------
CREATE DOMAIN courriel AS text
    CHECK (VALUE ~* '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$');

-- -----------------------------------------------------------------------------
-- DOMAIN — ISBN-13.
-- Exactement 13 chiffres (après normalisation applicative qui retire les tirets).
-- La clé de contrôle, elle, est vérifiée côté Go (validation métier « intelligente »).
-- -----------------------------------------------------------------------------
CREATE DOMAIN isbn13 AS text
    CHECK (VALUE ~ '^[0-9]{13}$');

-- Les commentaires de documentation (COMMENT ON) sont ajoutés avec les tables.
COMMENT ON TYPE role_utilisateur IS 'Rôles applicatifs pour l''autorisation.';
COMMENT ON TYPE statut_emprunt IS 'États possibles d''un emprunt.';
COMMENT ON DOMAIN courriel IS 'Adresse e-mail validée par expression régulière.';
COMMENT ON DOMAIN isbn13 IS 'ISBN-13 sous forme canonique (13 chiffres).';
