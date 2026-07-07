-- =============================================================================
-- 05_transactions.sql — TRANSACTIONS, isolation, verrous et concurrence
-- -----------------------------------------------------------------------------
-- OBJET : illustrer la gestion transactionnelle de PostgreSQL (moteur MVCC), un
-- socle de fiabilité pour toute API sérieuse :
--
--   BEGIN / COMMIT / ROLLBACK · SAVEPOINT + ROLLBACK TO SAVEPOINT ·
--   niveaux d'isolation (READ COMMITTED / REPEATABLE READ / SERIALIZABLE) et les
--   anomalies évitées · SELECT ... FOR UPDATE / FOR SHARE / SKIP LOCKED / NOWAIT ·
--   verrouillage pessimiste · MVCC (xmin/xmax/ctid) · gestion de la concurrence.
--
-- LIMITE D'UN SCRIPT SOLO : une seule session ne peut pas VRAIMENT entrer en
-- concurrence avec elle-même. Les scénarios à deux sessions sont donc présentés
-- en BLOCS COMMENTÉS « Session A / Session B », qui décrivent précisément ce qui
-- se passerait. Tout ce qui EST exécuté ci-dessous réussit sans erreur.
--
-- Tout se joue sur une table demo_ (demo_comptes), nettoyée à la fin. Le seed
-- n'est jamais modifié.
-- =============================================================================

\echo '========================================================================'
\echo '  05_transactions.sql — Transactions & concurrence'
\echo '========================================================================'

-- Table de démonstration : des comptes bancaires (le classique du virement).
-- Le CHECK (solde >= 0) sert à provoquer une erreur maîtrisée plus bas.
DROP TABLE IF EXISTS demo_comptes;
CREATE TABLE demo_comptes (
    id        int PRIMARY KEY,
    titulaire text NOT NULL,
    solde     numeric(12,2) NOT NULL,
    CONSTRAINT chk_solde_positif CHECK (solde >= 0)
);
INSERT INTO demo_comptes (id, titulaire, solde) VALUES
    (1, 'Alice', 1000.00),
    (2, 'Bruno',  500.00),
    (3, 'Chloé',  200.00);


-- #############################################################################
-- 1) BEGIN / COMMIT / ROLLBACK — l'atomicité (le « A » de ACID)
-- -----------------------------------------------------------------------------
-- Une transaction est TOUT ou RIEN. Un virement = un débit ET un crédit : les
-- deux réussissent ensemble (COMMIT) ou aucun ne subsiste (ROLLBACK).
-- #############################################################################
\echo '\n--- 1a) ROLLBACK : virement annulé, aucune trace ---'
BEGIN;
    UPDATE demo_comptes SET solde = solde - 300 WHERE id = 1;  -- Alice débitée
    UPDATE demo_comptes SET solde = solde + 300 WHERE id = 2;  -- Bruno crédité
    -- On change d'avis : ROLLBACK défait TOUT le bloc.
ROLLBACK;
SELECT id, titulaire, solde AS solde_apres_rollback FROM demo_comptes ORDER BY id;
-- => soldes inchangés (1000 / 500 / 200).

\echo '\n--- 1b) COMMIT : virement validé, durable ---'
BEGIN;
    UPDATE demo_comptes SET solde = solde - 300 WHERE id = 1;
    UPDATE demo_comptes SET solde = solde + 300 WHERE id = 2;
COMMIT;
SELECT id, titulaire, solde AS solde_apres_commit FROM demo_comptes ORDER BY id;
-- => 700 / 800 / 200.


-- #############################################################################
-- 2) SAVEPOINT + ROLLBACK TO SAVEPOINT — annuler PARTIELLEMENT
-- -----------------------------------------------------------------------------
-- Un SAVEPOINT est un point de reprise DANS une transaction. On peut y revenir
-- (ROLLBACK TO) sans tout perdre. C'est le mécanisme des « transactions imbriquées ».
-- #############################################################################
\echo '\n--- 2) SAVEPOINT : on garde le débit, on refait le crédit sur un autre compte ---'
BEGIN;
    UPDATE demo_comptes SET solde = solde - 100 WHERE id = 1;  -- Alice débitée de 100
    SAVEPOINT apres_debit;

    UPDATE demo_comptes SET solde = solde + 100 WHERE id = 2;  -- crédit Bruno (à annuler)
    -- Finalement, on ne voulait pas créditer Bruno : retour au point de reprise.
    ROLLBACK TO SAVEPOINT apres_debit;                          -- annule le crédit Bruno SEUL

    UPDATE demo_comptes SET solde = solde + 100 WHERE id = 3;  -- on crédite Chloé à la place
COMMIT;
SELECT id, titulaire, solde AS solde_final FROM demo_comptes ORDER BY id;
-- Alice -100, Chloé +100 ; Bruno inchangé (son crédit a été annulé par le SAVEPOINT).

\echo '\n--- 2 bis) Récupération d''ERREUR via sous-transaction (BEGIN/EXCEPTION plpgsql) ---'
-- En SQL pur, une erreur avorte toute la transaction. En PL/pgSQL, un bloc
-- BEGIN ... EXCEPTION crée un SAVEPOINT IMPLICITE : l'erreur est rattrapée et
-- SEULES les écritures du sous-bloc sont annulées. Démonstration : on tente un
-- découvert (interdit par le CHECK), on le rattrape, la transaction survit.
DO $$
BEGIN
    -- Sous-bloc protégé = savepoint implicite.
    BEGIN
        UPDATE demo_comptes SET solde = solde - 999999 WHERE id = 1;  -- viole CHECK
    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'Découvert refusé et rattrapé (le CHECK a joué son rôle).';
    END;
    RAISE NOTICE 'La transaction continue : Alice a toujours son solde initial.';
END$$;


-- #############################################################################
-- 3) NIVEAUX D'ISOLATION — quelles anomalies sont évitées ?
-- -----------------------------------------------------------------------------
-- PostgreSQL propose 3 niveaux EFFECTIFS (READ UNCOMMITTED = READ COMMITTED ici :
-- les lectures sales n'existent PAS en PostgreSQL, jamais). Rappel des anomalies :
--
--   - Lecture SALE (dirty read)        : lire une écriture non validée. JAMAIS en PG.
--   - Lecture NON RÉPÉTABLE            : relire une ligne et la voir MODIFIÉE.
--   - Lecture FANTÔME (phantom)        : relire un ensemble et voir APPARAÎTRE des lignes.
--   - Anomalie de SÉRIALISATION        : deux transactions concurrentes produisent
--                                        un résultat impossible en exécution en série.
--
--   READ COMMITTED (défaut) : voit les COMMITs des autres au fil de l'eau
--                             (chaque requête a son instantané). Évite le dirty read.
--   REPEATABLE READ         : instantané FIGÉ au début de la transaction. Évite en
--                             plus non-répétable ET fantômes (grâce au MVCC de PG).
--   SERIALIZABLE            : comme si les transactions s'exécutaient EN SÉRIE (SSI).
--                             Peut échouer avec « could not serialize » -> à REJOUER.
-- #############################################################################
\echo '\n--- 3) Les trois niveaux (on affiche le niveau effectif) ---'

BEGIN;
    SET TRANSACTION ISOLATION LEVEL READ COMMITTED;
    SHOW transaction_isolation;             -- read committed
    SELECT count(*) AS livres_visibles FROM livres WHERE supprime_le IS NULL;
COMMIT;

BEGIN;
    SET TRANSACTION ISOLATION LEVEL REPEATABLE READ;
    SHOW transaction_isolation;             -- repeatable read
    -- Dans CETTE transaction, deux lectures successives verront TOUJOURS la même
    -- photo, même si une autre session COMMIT entre-temps (instantané figé).
    SELECT count(*) AS photo_figee FROM livres;
COMMIT;

BEGIN;
    SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
    SHOW transaction_isolation;             -- serializable
    SELECT sum(solde) AS total_demo_comptes FROM demo_comptes;
COMMIT;


-- #############################################################################
-- 4) VERROUILLAGE PESSIMISTE — SELECT ... FOR UPDATE / FOR SHARE
-- -----------------------------------------------------------------------------
-- « Pessimiste » = je VERROUILLE la ligne AVANT de la modifier, pour empêcher
-- toute écriture concurrente entre ma lecture et mon écriture (évite la « mise à
-- jour perdue »). C'est le patron d'un virement sûr, d'un décompte de stock, etc.
--   FOR UPDATE : verrou exclusif (lecture+écriture bloquées pour les autres).
--   FOR SHARE  : verrou partagé (les autres peuvent LIRE FOR SHARE, pas écrire).
-- #############################################################################
\echo '\n--- 4) FOR UPDATE : on verrouille une ligne, on montre le verrou détenu ---'
BEGIN;
    -- On verrouille le compte d'Alice. Toute autre session voulant l'écrire ATTENDRA.
    SELECT id, titulaire, solde
    FROM demo_comptes
    WHERE id = 1
    FOR UPDATE;

    -- Preuve concrète : un verrou de ligne est bien détenu par NOTRE session.
    SELECT locktype, mode, granted
    FROM pg_locks
    WHERE pid = pg_backend_pid()
      AND locktype IN ('tuple', 'transactionid')
    ORDER BY locktype;

    -- Ici, en toute sécurité, on ferait le calcul puis l'UPDATE.
    UPDATE demo_comptes SET solde = solde - 50 WHERE id = 1;
ROLLBACK;   -- on annule : la démo ne modifie pas les soldes.

\echo '\n--- 4 bis) FOR SHARE : verrou partagé (autorise d''autres lecteurs FOR SHARE) ---'
BEGIN;
    SELECT id, titulaire FROM demo_comptes WHERE id = 2 FOR SHARE;
ROLLBACK;


-- #############################################################################
-- 5) SKIP LOCKED / NOWAIT — variantes non bloquantes (files de travail)
-- -----------------------------------------------------------------------------
--   SKIP LOCKED : ignore les lignes déjà verrouillées par d'autres. Patron IDÉAL
--                 d'une FILE DE TÂCHES : chaque worker prend un lot DIFFÉRENT sans
--                 attendre ni se marcher dessus.
--   NOWAIT      : au lieu d'attendre un verrou, échoue IMMÉDIATEMENT (utile pour
--                 ne pas bloquer une requête interactive).
-- Seul (sans concurrence), rien n'est verrouillé : les requêtes s'exécutent normalement.
-- #############################################################################
\echo '\n--- 5) SKIP LOCKED : prendre 2 « tâches » sans jamais attendre ---'
BEGIN;
    -- Chaque worker exécuterait CECI : il rafle 2 lignes libres et saute celles
    -- qu'un autre worker a déjà verrouillées.
    SELECT id, titulaire
    FROM demo_comptes
    ORDER BY id
    FOR UPDATE SKIP LOCKED
    LIMIT 2;
ROLLBACK;

-- Bloc COMMENTÉ — ce que SKIP LOCKED donnerait à DEUX workers simultanés :
--   Session A : SELECT ... FOR UPDATE SKIP LOCKED LIMIT 2;  -> obtient {1, 2}
--   Session B : SELECT ... FOR UPDATE SKIP LOCKED LIMIT 2;  -> obtient {3} (saute 1,2)
--   => aucun blocage, aucun doublon : parallélisme parfait pour une file de jobs.


-- #############################################################################
-- 6) MVCC — comment PostgreSQL évite (souvent) les verrous en LECTURE
-- -----------------------------------------------------------------------------
-- MVCC = Multi-Version Concurrency Control. Chaque UPDATE ne modifie pas la ligne
-- « sur place » : il écrit une NOUVELLE VERSION et marque l'ancienne comme périmée.
-- Colonnes système utiles :
--   xmin : transaction qui a CRÉÉ cette version.   ctid : emplacement physique.
--   xmax : transaction qui l'a PÉRIMÉE (0 si vivante).
-- Conséquence : les LECTEURS ne bloquent JAMAIS les ÉCRIVAINS et inversement —
-- un immense avantage de débit face à un verrouillage systématique.
-- (Le revers : les versions mortes s'accumulent -> VACUUM les recycle.)
-- #############################################################################
\echo '\n--- 6) MVCC : un UPDATE change la VERSION physique (ctid) de la ligne ---'
BEGIN;
    SELECT ctid, xmin, xmax, solde AS avant FROM demo_comptes WHERE id = 3;
    UPDATE demo_comptes SET solde = solde + 1 WHERE id = 3;
    -- Après l'UPDATE : ctid DIFFÉRENT (nouvelle version) et xmin = notre transaction.
    SELECT ctid, xmin, xmax, solde AS apres FROM demo_comptes WHERE id = 3;
ROLLBACK;


-- #############################################################################
-- 7) CONCURRENCE — scénarios À DEUX SESSIONS (blocs commentés)
-- -----------------------------------------------------------------------------
-- Ces séquences ne peuvent pas s'exécuter dans une seule session : on décrit ce
-- qui se produirait, chronologiquement, avec deux clients simultanés.
-- #############################################################################
\echo '\n--- 7) Scénarios de concurrence (voir les commentaires du fichier) ---'

-- ┌─────────────────────────────────────────────────────────────────────────┐
-- │ 7a. MISE À JOUR PERDUE (lost update) et sa PRÉVENTION par FOR UPDATE      │
-- └─────────────────────────────────────────────────────────────────────────┘
--   SANS verrou (les deux lisent 1000, puis écrivent) :
--     A: BEGIN;                                   B: BEGIN;
--     A: SELECT solde FROM ... id=1;  -- 1000     B: SELECT solde FROM ... id=1;  -- 1000
--     A: UPDATE ... solde = 1000-100; -- 900      B: UPDATE ... solde = 1000-200; -- attend A
--     A: COMMIT;                                  B: (débloqué) écrit 800, COMMIT;
--     => Le retrait de 100 est ÉCRASÉ : solde final 800 au lieu de 700. ANOMALIE.
--
--   AVEC « SELECT ... FOR UPDATE » (verrouillage pessimiste) :
--     A: BEGIN; SELECT solde ... id=1 FOR UPDATE;   -- verrouille, lit 1000
--     B: BEGIN; SELECT solde ... id=1 FOR UPDATE;   -- ATTEND que A libère
--     A: UPDATE ... 1000-100=900; COMMIT;           -- libère le verrou
--     B: (débloqué) RELIT 900, UPDATE 900-200=700; COMMIT;
--     => solde final CORRECT (700). Le verrou a sérialisé les deux virements.

-- ┌─────────────────────────────────────────────────────────────────────────┐
-- │ 7b. SERIALIZABLE : détection d'anomalie et REJEU                          │
-- └─────────────────────────────────────────────────────────────────────────┘
--     A et B en SERIALIZABLE lisent le même total puis insèrent chacune une ligne
--     dépendant de ce total. À l'un des COMMIT, PostgreSQL détecte que le résultat
--     n'équivaut à aucune exécution EN SÉRIE et rejette :
--        ERROR: could not serialize access due to read/write dependencies
--        HINT:  The transaction might succeed if retried.
--     BONNE PRATIQUE : encapsuler la transaction dans une BOUCLE DE REJEU côté Go
--     (réessayer sur SQLSTATE 40001). C'est le prix de la garantie la plus forte.

-- ┌─────────────────────────────────────────────────────────────────────────┐
-- │ 7c. INTERBLOCAGE (deadlock) : verrouiller dans le MÊME ORDRE             │
-- └─────────────────────────────────────────────────────────────────────────┘
--     A: UPDATE ... id=1;   B: UPDATE ... id=2;
--     A: UPDATE ... id=2;   -- attend B        B: UPDATE ... id=1;  -- attend A => CYCLE
--     PostgreSQL DÉTECTE le cycle et tue l'une des transactions (SQLSTATE 40P01).
--     PRÉVENTION : toujours verrouiller les ressources dans un ORDRE COHÉRENT
--     (ex. par id croissant), afin qu'aucun cycle ne puisse se former.


-- #############################################################################
-- NETTOYAGE
-- #############################################################################
\echo '\n--- Nettoyage des objets demo_ ---'
DROP TABLE IF EXISTS demo_comptes;

\echo '\n05_transactions.sql : terminé sans erreur.'
