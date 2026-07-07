-- =============================================================================
-- 00_extensions.sql — Activation des EXTENSIONS PostgreSQL
-- -----------------------------------------------------------------------------
-- Les EXTENSIONS sont l'une des plus grandes forces de PostgreSQL : des modules
-- qui ajoutent des types, des fonctions, des index ou des mécanismes entiers, à
-- activer d'une simple commande. C'est une différence majeure avec MariaDB, dont
-- l'extensibilité est bien plus limitée.
--
-- « CREATE EXTENSION IF NOT EXISTS » est idempotent : rejouable sans erreur.
--
-- NB : pg_cron n'est PAS activée ici. Contrairement aux autres, elle exige d'être
-- préchargée au démarrage du serveur (shared_preload_libraries) : son activation
-- est donc gérée à part (voir sql/cron/ et docker/postgres).
-- =============================================================================

-- -----------------------------------------------------------------------------
-- pgcrypto — fonctions cryptographiques
--
-- Fournit gen_random_uuid() (génération d'UUID v4 aléatoires), ainsi que digest(),
-- hmac(), crypt()... Depuis PostgreSQL 13, gen_random_uuid() est disponible en
-- natif (sans extension), mais on active pgcrypto pour ses autres fonctions et
-- pour la compatibilité avec les versions antérieures.
-- Cas d'usage : identifiants non devinables, empreintes, signatures.
-- -----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- -----------------------------------------------------------------------------
-- pg_trgm — similarité par TRIGRAMMES
--
-- Découpe le texte en groupes de 3 caractères (« trigrammes ») pour mesurer la
-- similarité entre chaînes. Son intérêt DÉCISIF ici : il permet de créer un index
-- GIN qui accélère les recherches « ILIKE '%terme%' » (avec joker en tête), que
-- les index B-tree classiques ne peuvent PAS optimiser. C'est ce qui rend la
-- recherche du catalogue rapide même sur de gros volumes.
-- Cas d'usage : recherche floue, autocomplétion, correction de fautes de frappe.
-- -----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- -----------------------------------------------------------------------------
-- uuid-ossp — génération d'UUID (alternative historique)
--
-- Fournit uuid_generate_v1(), uuid_generate_v4()... On l'active à but pédagogique
-- et de compatibilité. En pratique, on privilégie gen_random_uuid() (pgcrypto /
-- natif), plus simple et sans dépendance. Le nom contient un tiret : il doit donc
-- être mis entre guillemets doubles.
-- -----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Vérification (visible dans les logs d'initialisation) :
--   SELECT extname, extversion FROM pg_extension ORDER BY extname;
