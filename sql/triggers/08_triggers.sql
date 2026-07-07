-- =============================================================================
-- 08_triggers.sql — Triggers (déclencheurs)
-- -----------------------------------------------------------------------------
-- MODÈLE POSTGRESQL : contrairement à MariaDB (où le corps du trigger est écrit
-- directement dans CREATE TRIGGER), PostgreSQL sépare DEUX choses :
--   1. une FONCTION trigger (RETURNS trigger) qui contient la logique ;
--   2. un CREATE TRIGGER qui l'attache à une table, un moment (BEFORE/AFTER/
--      INSTEAD OF) et un événement (INSERT/UPDATE/DELETE).
-- Avantage : une même fonction peut être RÉUTILISÉE par plusieurs triggers.
--
-- Variables spéciales dans une fonction trigger :
--   NEW / OLD          : la nouvelle / l'ancienne ligne ;
--   TG_OP              : 'INSERT' | 'UPDATE' | 'DELETE' ;
--   TG_TABLE_NAME      : nom de la table déclenchante.
--
-- Un trigger BEFORE renvoie NEW (éventuellement modifié) ; un trigger AFTER
-- renvoie NULL (valeur ignorée). RAISE EXCEPTION annule l'opération (équivalent
-- du SIGNAL de MariaDB) : c'est capté côté Go comme un 409 avec message métier.
-- =============================================================================

-- =========================== Utilitaires génériques ==========================

-- fn_maj_modifie_le : met à jour la colonne modifie_le à chaque modification.
-- REMARQUE : MariaDB offrait « ON UPDATE CURRENT_TIMESTAMP » sur la colonne.
-- PostgreSQL n'a pas cet automatisme : on le reproduit avec ce trigger BEFORE UPDATE,
-- réutilisé sur toutes les tables concernées.
CREATE OR REPLACE FUNCTION fn_maj_modifie_le()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.modifie_le := now();
    RETURN NEW;
END;
$$;

-- fn_audit : journalise TOUTE opération dans journal_audit, sous forme JSONB.
-- UNE seule fonction générique, attachée à plusieurs tables — c'est toute la
-- puissance de PostgreSQL : to_jsonb(NEW) sérialise la ligne entière en JSONB,
-- et l'opérateur «  - 'cle' » retire une clé (ici le hash de mot de passe, qu'on
-- ne veut jamais journaliser).
CREATE OR REPLACE FUNCTION fn_audit()
    RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    v_cle       bigint;
    v_anciennes jsonb;
    v_nouvelles jsonb;
BEGIN
    IF TG_OP = 'DELETE' THEN
        v_cle       := OLD.id;
        v_anciennes := to_jsonb(OLD) - 'mot_de_passe_hash';
    ELSIF TG_OP = 'UPDATE' THEN
        v_cle       := NEW.id;
        v_anciennes := to_jsonb(OLD) - 'mot_de_passe_hash';
        v_nouvelles := to_jsonb(NEW) - 'mot_de_passe_hash';
    ELSE -- INSERT
        v_cle       := NEW.id;
        v_nouvelles := to_jsonb(NEW) - 'mot_de_passe_hash';
    END IF;

    INSERT INTO journal_audit (table_concernee, operation, cle_enregistrement,
                               anciennes_valeurs, nouvelles_valeurs, acteur_sql)
    VALUES (TG_TABLE_NAME, TG_OP::operation_audit, v_cle, v_anciennes, v_nouvelles, current_user);

    RETURN NULL; -- trigger AFTER : la valeur de retour est ignorée
END;
$$;

-- =========================== TABLE utilisateurs ==============================

-- BEFORE INSERT/UPDATE : normalise l'e-mail (minuscules + sans espaces).
CREATE OR REPLACE FUNCTION fn_utilisateurs_normaliser()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.email := lower(trim(NEW.email));
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_utilisateurs_normaliser
    BEFORE INSERT OR UPDATE ON utilisateurs
    FOR EACH ROW EXECUTE FUNCTION fn_utilisateurs_normaliser();

CREATE TRIGGER trg_utilisateurs_modifie
    BEFORE UPDATE ON utilisateurs
    FOR EACH ROW EXECUTE FUNCTION fn_maj_modifie_le();

-- AFTER INSERT/UPDATE/DELETE : audit (couvre les trois événements d'un coup).
CREATE TRIGGER trg_utilisateurs_audit
    AFTER INSERT OR UPDATE OR DELETE ON utilisateurs
    FOR EACH ROW EXECUTE FUNCTION fn_audit();

-- =============================== TABLE livres ================================

-- BEFORE INSERT : normalise l'ISBN (retire tirets/espaces) avant la vérification
-- du DOMAIN isbn13. Les triggers BEFORE s'exécutent AVANT les contraintes.
CREATE OR REPLACE FUNCTION fn_livres_normaliser()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.isbn := translate(NEW.isbn, '- ', '');
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_livres_normaliser
    BEFORE INSERT OR UPDATE ON livres
    FOR EACH ROW EXECUTE FUNCTION fn_livres_normaliser();

-- BEFORE UPDATE : VALIDATION métier avec RAISE EXCEPTION (message exposable).
-- La contrainte CHECK le garantit déjà ; le trigger fournit un message clair.
CREATE OR REPLACE FUNCTION fn_livres_valider_stock()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.exemplaires_disponibles > NEW.nombre_exemplaires THEN
        RAISE EXCEPTION 'Incohérence de stock : exemplaires disponibles > total.';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_livres_valider_stock
    BEFORE UPDATE ON livres
    FOR EACH ROW EXECUTE FUNCTION fn_livres_valider_stock();

CREATE TRIGGER trg_livres_modifie
    BEFORE UPDATE ON livres
    FOR EACH ROW EXECUTE FUNCTION fn_maj_modifie_le();

-- AFTER UPDATE : audit des changements de stock/prix.
CREATE TRIGGER trg_livres_audit
    AFTER UPDATE ON livres
    FOR EACH ROW EXECUTE FUNCTION fn_audit();

-- ============================= TABLE emprunts ================================

-- BEFORE INSERT : calcule une date de retour par défaut si absente (démontre un
-- calcul par trigger, comme la version MariaDB).
CREATE OR REPLACE FUNCTION fn_emprunts_date_retour()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.date_retour_prevue IS NULL THEN
        NEW.date_retour_prevue := NEW.date_emprunt + INTERVAL '14 days';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_emprunts_date_retour
    BEFORE INSERT ON emprunts
    FOR EACH ROW EXECUTE FUNCTION fn_emprunts_date_retour();

CREATE TRIGGER trg_emprunts_modifie
    BEFORE UPDATE ON emprunts
    FOR EACH ROW EXECUTE FUNCTION fn_maj_modifie_le();

-- AFTER INSERT/UPDATE : audit.
CREATE TRIGGER trg_emprunts_audit
    AFTER INSERT OR UPDATE ON emprunts
    FOR EACH ROW EXECUTE FUNCTION fn_audit();

-- BEFORE DELETE : RÈGLE MÉTIER — interdit de supprimer un emprunt encore actif.
CREATE OR REPLACE FUNCTION fn_emprunts_interdire_suppr_active()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.statut IN ('en_cours', 'en_retard') THEN
        RAISE EXCEPTION 'Suppression interdite : cet emprunt est encore actif.';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_emprunts_interdire_suppr
    BEFORE DELETE ON emprunts
    FOR EACH ROW EXECUTE FUNCTION fn_emprunts_interdire_suppr_active();

-- =========================== Catégories & auteurs ============================
-- (modifie_le uniquement ; pas d'audit métier nécessaire.)
CREATE TRIGGER trg_categories_modifie
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION fn_maj_modifie_le();

CREATE TRIGGER trg_auteurs_modifie
    BEFORE UPDATE ON auteurs
    FOR EACH ROW EXECUTE FUNCTION fn_maj_modifie_le();

-- =========================== Trigger INSTEAD OF ==============================
-- DÉMONSTRATION propre à PostgreSQL : un trigger INSTEAD OF permet de rendre une
-- VUE modifiable. Ici, supprimer une ligne de la vue « vue_livres_actifs »
-- déclenche en réalité une suppression LOGIQUE (soft delete) sur la table livres.
-- (Cette vue n'est pas utilisée par l'API : c'est un exemple pédagogique.)
CREATE OR REPLACE VIEW vue_livres_actifs AS
    SELECT id, uuid, titre, isbn FROM livres WHERE supprime_le IS NULL;

CREATE OR REPLACE FUNCTION fn_livres_actifs_instead_delete()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE livres SET supprime_le = now() WHERE id = OLD.id;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_livres_actifs_instead_delete
    INSTEAD OF DELETE ON vue_livres_actifs
    FOR EACH ROW EXECUTE FUNCTION fn_livres_actifs_instead_delete();
