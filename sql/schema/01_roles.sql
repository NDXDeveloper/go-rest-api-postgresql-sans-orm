-- =============================================================================
-- 01_roles.sql — Rôle applicatif et privilèges (principe du MOINDRE PRIVILÈGE)
-- -----------------------------------------------------------------------------
-- MODÈLE DE SÉCURITÉ POSTGRESQL
--
-- PostgreSQL unifie « utilisateurs » et « groupes » sous la notion de RÔLE. Un
-- rôle avec l'attribut LOGIN peut se connecter (c'est un « utilisateur »).
--
-- Principe fondamental : l'application ne se connecte JAMAIS avec le superutilisateur
-- (« postgres »). On crée un rôle dédié « app_bibliotheque » au périmètre limité.
--
-- DIFFÉRENCE AVEC MariaDB : MariaDB raisonne en « utilisateur@hôte » et en GRANT
-- par table. PostgreSQL raisonne en rôles, schémas, et propose ALTER DEFAULT
-- PRIVILEGES pour appliquer automatiquement des droits aux futurs objets.
--
-- REMARQUE : le mot de passe ci-dessous est une valeur de DÉVELOPPEMENT. En
-- production (Docker), un script d'initialisation applique le secret réel issu
-- des variables d'environnement (ALTER ROLE ... PASSWORD).
-- =============================================================================

-- -----------------------------------------------------------------------------
-- CREATE ROLE (idempotent grâce au bloc DO : PostgreSQL n'a pas « IF NOT EXISTS »
-- pour CREATE ROLE).
-- -----------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_bibliotheque') THEN
        CREATE ROLE app_bibliotheque LOGIN PASSWORD 'changez_moi_app';
    END IF;
END
$$;

-- -----------------------------------------------------------------------------
-- REVOKE — on retire les droits trop larges accordés par défaut au pseudo-rôle
-- PUBLIC (tout le monde). Défense en profondeur.
-- -----------------------------------------------------------------------------
-- Empêche la création d'objets dans le schéma public par n'importe qui.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
-- Retire les privilèges implicites sur la base.
REVOKE ALL ON DATABASE bibliotheque FROM PUBLIC;

-- -----------------------------------------------------------------------------
-- GRANT — on accorde le STRICT nécessaire au rôle applicatif.
-- -----------------------------------------------------------------------------
GRANT CONNECT ON DATABASE bibliotheque TO app_bibliotheque;
GRANT USAGE ON SCHEMA public TO app_bibliotheque;

-- -----------------------------------------------------------------------------
-- ALTER DEFAULT PRIVILEGES — la clé de voûte : ces droits s'appliqueront
-- AUTOMATIQUEMENT à tous les objets créés ENSUITE (tables, séquences, fonctions)
-- dans le schéma public par le rôle courant (postgres, lors de l'initialisation).
-- On n'a donc pas à faire un GRANT après chaque CREATE TABLE.
--
-- On accorde SELECT/INSERT/UPDATE/DELETE (le CRUD) mais PAS de DROP/ALTER/TRUNCATE :
-- même en cas d'injection SQL, l'attaquant ne pourrait pas détruire le schéma.
-- -----------------------------------------------------------------------------
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_bibliotheque;
-- Les colonnes d'identité (GENERATED AS IDENTITY) s'appuient sur des séquences :
-- l'application a besoin de USAGE/SELECT dessus pour insérer.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_bibliotheque;
-- EXECUTE couvre les fonctions ET les procédures (les « routines ») : nécessaire
-- pour appeler nos fonctions PL/pgSQL et nos procédures (CALL).
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO app_bibliotheque;

-- -----------------------------------------------------------------------------
-- ALTER ROLE — configuration par rôle (paramètres de session appliqués à chaque
-- connexion de ce rôle). Exemple : un garde-fou anti-requête-folle.
-- -----------------------------------------------------------------------------
ALTER ROLE app_bibliotheque SET statement_timeout = '30s';
