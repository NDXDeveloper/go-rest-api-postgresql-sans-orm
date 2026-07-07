-- =============================================================================
-- 10_seed.sql — Jeu de données de démonstration (IDENTIQUE à la version MariaDB)
-- -----------------------------------------------------------------------------
-- Les données sont RIGOUREUSEMENT les mêmes que dans le projet MariaDB (mêmes
-- catégories, auteurs, livres, utilisateurs, emprunts), afin que les scénarios de
-- test fonctionnels soient identiques entre les deux dépôts.
--
-- MOT DE PASSE DE DÉMONSTRATION : « MotDePasse123! » pour tous les comptes.
-- mot_de_passe_hash contient son haché bcrypt (coût 12).
--
-- PARTICULARITÉ POSTGRESQL :
--   - les colonnes « id » étant GENERATED ALWAYS AS IDENTITY, on force les valeurs
--     explicites (pour maîtriser les clés étrangères du seed) avec la clause
--     « OVERRIDING SYSTEM VALUE » ;
--   - « uuid » n'est pas fourni : il est généré par le DEFAULT gen_random_uuid() ;
--   - à la fin, on RÉALIGNE les séquences d'identité avec setval().
-- =============================================================================

-- Nettoyage rejouable. TRUNCATE ... RESTART IDENTITY réinitialise les séquences ;
-- CASCADE gère les dépendances de clés étrangères. TRUNCATE ne déclenche pas les
-- triggers ligne à ligne (dont l'interdiction de suppression d'emprunt actif).
TRUNCATE emprunts_archive, statistiques_quotidiennes, journal_audit,
         jetons_rafraichissement, emprunts, livres, auteurs, categories, utilisateurs
    RESTART IDENTITY CASCADE;

-- -----------------------------------------------------------------------------
-- CATÉGORIES
-- -----------------------------------------------------------------------------
INSERT INTO categories (id, nom, description) OVERRIDING SYSTEM VALUE VALUES
    (1, 'Roman',           'Œuvres de fiction narrative en prose'),
    (2, 'Science-fiction', 'Anticipation, mondes imaginaires et technologies'),
    (3, 'Policier',        'Enquêtes, thrillers et romans à suspense'),
    (4, 'Histoire',        'Essais et récits historiques'),
    (5, 'Informatique',    'Programmation, systèmes et réseaux'),
    (6, 'Jeunesse',        'Ouvrages destinés au jeune public'),
    (7, 'Poésie',          'Recueils de poèmes'),
    (8, 'Biographie',      'Récits de vie et mémoires');

-- -----------------------------------------------------------------------------
-- AUTEURS
-- -----------------------------------------------------------------------------
INSERT INTO auteurs (id, nom, prenom, nationalite, date_naissance, biographie) OVERRIDING SYSTEM VALUE VALUES
    (1,  'Hugo',          'Victor',     'française',   '1802-02-26', 'Écrivain majeur du romantisme français.'),
    (2,  'Verne',         'Jules',      'française',   '1828-02-08', 'Pionnier de la science-fiction et du roman d''aventures.'),
    (3,  'Christie',      'Agatha',     'britannique', '1890-09-15', 'Reine du roman policier.'),
    (4,  'Asimov',        'Isaac',      'américaine',  '1920-01-02', 'Auteur prolifique de science-fiction.'),
    (5,  'Camus',         'Albert',     'française',   '1913-11-07', 'Écrivain et philosophe, prix Nobel 1957.'),
    (6,  'Orwell',        'George',     'britannique', '1903-06-25', 'Auteur de récits politiques et dystopiques.'),
    (7,  'Saint-Exupéry', 'Antoine de', 'française',   '1900-06-29', 'Aviateur et écrivain.'),
    (8,  'Herbert',       'Frank',      'américaine',  '1920-10-08', 'Créateur du cycle de Dune.'),
    (9,  'Yourcenar',     'Marguerite', 'française',   '1903-06-08', 'Première femme élue à l''Académie française.'),
    (10, 'Dostoïevski',   'Fiodor',     'russe',       '1821-11-11', 'Romancier russe majeur du XIXe siècle.'),
    (11, 'Tolkien',       'J.R.R.',     'britannique', '1892-01-03', 'Père de la fantasy moderne.'),
    (12, 'de Beauvoir',   'Simone',     'française',   '1908-01-09', 'Philosophe et figure du féminisme.');

-- -----------------------------------------------------------------------------
-- LIVRES (ISBN-13 valides ; exemplaires_disponibles ajusté plus bas)
-- -----------------------------------------------------------------------------
INSERT INTO livres (id, titre, isbn, auteur_id, categorie_id, annee_publication, nombre_exemplaires, exemplaires_disponibles, prix, langue, resume) OVERRIDING SYSTEM VALUE VALUES
    (1,  'Les Misérables',                  '9782010000003', 1,  1, 1862, 4, 4, 12.90, 'français', 'Le destin de Jean Valjean dans la France du XIXe siècle.'),
    (2,  'Notre-Dame de Paris',             '9782010000010', 1,  1, 1831, 3, 3, 10.50, 'français', 'Quasimodo, Esmeralda et la cathédrale.'),
    (3,  'Vingt mille lieues sous les mers','9782010000027', 2,  2, 1870, 5, 5,  9.90, 'français', 'Le capitaine Nemo et le Nautilus.'),
    (4,  'Le Tour du monde en 80 jours',    '9782010000034', 2,  1, 1872, 4, 4,  8.90, 'français', 'Le pari de Phileas Fogg.'),
    (5,  'De la Terre à la Lune',           '9782010000041', 2,  2, 1865, 2, 2,  7.50, 'français', 'Un voyage spatial visionnaire.'),
    (6,  'Le Crime de l''Orient-Express',   '9782010000058', 3,  3, 1934, 3, 3, 11.00, 'français', 'Hercule Poirot mène l''enquête.'),
    (7,  'Dix petits nègres',               '9782010000065', 3,  3, 1939, 3, 3, 10.90, 'français', 'Dix inconnus sur une île.'),
    (8,  'Fondation',                       '9782010000072', 4,  2, 1951, 4, 4, 13.50, 'français', 'La psychohistoire et la chute d''un empire galactique.'),
    (9,  'Les Robots',                      '9782010000089', 4,  2, 1950, 3, 3, 12.00, 'français', 'Les trois lois de la robotique.'),
    (10, 'L''Étranger',                     '9782010000096', 5,  1, 1942, 5, 5,  7.90, 'français', 'Meursault face à l''absurde.'),
    (11, 'La Peste',                        '9782010000102', 5,  1, 1947, 4, 4,  9.20, 'français', 'Oran frappée par l''épidémie.'),
    (12, '1984',                            '9782010000119', 6,  2, 1949, 6, 6, 10.00, 'français', 'Big Brother vous regarde.'),
    (13, 'La Ferme des animaux',            '9782010000126', 6,  1, 1945, 4, 4,  8.50, 'français', 'Une fable politique.'),
    (14, 'Le Petit Prince',                 '9782010000133', 7,  6, 1943, 8, 8,  6.90, 'français', 'Un aviateur rencontre un petit prince.'),
    (15, 'Vol de nuit',                     '9782010000140', 7,  1, 1931, 2, 2,  7.20, 'français', 'L''aéropostale et le courage des pilotes.'),
    (16, 'Dune',                            '9782010000157', 8,  2, 1965, 5, 5, 15.90, 'français', 'Paul Atréides sur la planète Arrakis.'),
    (17, 'Le Messie de Dune',               '9782010000164', 8,  2, 1969, 3, 3, 14.50, 'français', 'La suite de l''épopée de Dune.'),
    (18, 'Mémoires d''Hadrien',             '9782010000171', 9,  1, 1951, 3, 3, 11.90, 'français', 'Les mémoires imaginaires de l''empereur romain.'),
    (19, 'Crime et Châtiment',              '9782010000188', 10, 1, 1866, 4, 4, 12.50, 'français', 'Raskolnikov et sa conscience.'),
    (20, 'Les Frères Karamazov',            '9782010000195', 10, 1, 1880, 3, 3, 14.00, 'français', 'Une fresque familiale et philosophique.'),
    (21, 'Le Seigneur des anneaux',         '9782010000201', 11, 2, 1954, 6, 6, 24.90, 'français', 'La quête de l''anneau unique.'),
    (22, 'Le Hobbit',                       '9782010000218', 11, 6, 1937, 5, 5, 13.90, 'français', 'Les aventures de Bilbo.'),
    (23, 'Le Deuxième Sexe',                '9782010000225', 12, 4, 1949, 2, 2, 16.50, 'français', 'Essai fondateur du féminisme.'),
    (24, 'Le Programmeur pragmatique',      '9782010000232', 4,  5, 1999, 3, 3, 39.90, 'français', 'Bonnes pratiques du développement logiciel.'),
    (25, 'Introduction aux algorithmes',    '9782010000249', 4,  5, 2009, 2, 2, 79.00, 'français', 'Référence sur les algorithmes.'),
    (26, 'Le Langage Go',                   '9782010000256', 4,  5, 2015, 4, 4, 42.00, 'français', 'Guide du langage Go.'),
    (27, 'Contes du soir',                  '9782010000263', 7,  6, 1998, 5, 5,  9.50, 'français', 'Recueil d''histoires pour enfants.'),
    (28, 'Recueil de poèmes',               '9782010000270', 1,  7, 1856, 3, 3,  8.00, 'français', 'Sélection de poèmes du XIXe siècle.');

-- -----------------------------------------------------------------------------
-- UTILISATEURS — 1 admin, 1 bibliothécaire, 6 membres. Mot de passe : MotDePasse123!
-- -----------------------------------------------------------------------------
INSERT INTO utilisateurs (id, email, mot_de_passe_hash, nom, prenom, role, actif) OVERRIDING SYSTEM VALUE VALUES
    (1, 'admin@bibliotheque.fr',          '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Martin',  'Alice',   'admin',          true),
    (2, 'bibliothecaire@bibliotheque.fr', '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Bernard', 'Bruno',   'bibliothecaire', true),
    (3, 'chloe.durand@exemple.fr',        '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Durand',  'Chloé',   'membre',         true),
    (4, 'david.petit@exemple.fr',         '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Petit',   'David',   'membre',         true),
    (5, 'emma.roux@exemple.fr',           '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Roux',    'Emma',    'membre',         true),
    (6, 'farid.benali@exemple.fr',        '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Benali',  'Farid',   'membre',         true),
    (7, 'gwen.leroy@exemple.fr',          '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Leroy',   'Gwendal', 'membre',         true),
    (8, 'hugo.moreau@exemple.fr',         '$2a$12$yT9vOdQVzElpmOmuxIuQVedExe66QZ6GroshqHR5gkP4cAYA9I.CG', 'Moreau',  'Hugo',    'membre',         false);

-- -----------------------------------------------------------------------------
-- EMPRUNTS — statuts variés (rendu / en_cours / en_retard). Dates relatives.
-- Certains laissent date_retour_prevue à NULL pour illustrer le trigger de calcul.
-- -----------------------------------------------------------------------------
INSERT INTO emprunts (id, utilisateur_id, livre_id, date_emprunt, date_retour_prevue, date_retour_effective, statut, penalite) OVERRIDING SYSTEM VALUE VALUES
    (1,  3, 1,  CURRENT_DATE - 40, CURRENT_DATE - 26, CURRENT_DATE - 28, 'rendu',     0.00),
    (2,  4, 10, CURRENT_DATE - 35, CURRENT_DATE - 21, CURRENT_DATE - 15, 'rendu',     3.00),
    (3,  5, 16, CURRENT_DATE - 50, CURRENT_DATE - 36, CURRENT_DATE - 40, 'rendu',     0.00),
    (4,  6, 12, CURRENT_DATE - 25, CURRENT_DATE - 11, CURRENT_DATE - 12, 'rendu',     0.00),
    (5,  3, 21, CURRENT_DATE - 5,  CURRENT_DATE + 9,  NULL,              'en_cours',  0.00),
    (6,  4, 8,  CURRENT_DATE - 3,  CURRENT_DATE + 11, NULL,              'en_cours',  0.00),
    (7,  5, 3,  CURRENT_DATE - 7,  CURRENT_DATE + 7,  NULL,              'en_cours',  0.00),
    (8,  7, 26, CURRENT_DATE - 2,  NULL,              NULL,              'en_cours',  0.00),
    (9,  6, 14, CURRENT_DATE - 1,  CURRENT_DATE + 13, NULL,              'en_cours',  0.00),
    (10, 4, 19, CURRENT_DATE - 30, CURRENT_DATE - 16, NULL,              'en_retard', 8.00),
    (11, 7, 6,  CURRENT_DATE - 28, CURRENT_DATE - 14, NULL,              'en_retard', 7.00),
    (12, 5, 1,  CURRENT_DATE - 20, CURRENT_DATE - 6,  NULL,              'en_retard', 3.00);

-- -----------------------------------------------------------------------------
-- SYNCHRONISATION DU STOCK : exemplaires_disponibles = total - emprunts actifs.
-- (Sous-requête corrélée.)
-- -----------------------------------------------------------------------------
UPDATE livres l
SET exemplaires_disponibles = l.nombre_exemplaires - COALESCE((
    SELECT count(*)
    FROM emprunts e
    WHERE e.livre_id = l.id AND e.statut IN ('en_cours', 'en_retard')
), 0);

-- -----------------------------------------------------------------------------
-- RÉALIGNEMENT DES SÉQUENCES D'IDENTITÉ
-- Après des insertions à id explicites, on repositionne chaque séquence au-dessus
-- du plus grand id, pour éviter tout conflit lors des prochains INSERT applicatifs.
-- pg_get_serial_sequence retrouve le nom de la séquence liée à la colonne.
-- -----------------------------------------------------------------------------
SELECT setval(pg_get_serial_sequence('categories',   'id'), (SELECT max(id) FROM categories));
SELECT setval(pg_get_serial_sequence('auteurs',      'id'), (SELECT max(id) FROM auteurs));
SELECT setval(pg_get_serial_sequence('livres',       'id'), (SELECT max(id) FROM livres));
SELECT setval(pg_get_serial_sequence('utilisateurs', 'id'), (SELECT max(id) FROM utilisateurs));
SELECT setval(pg_get_serial_sequence('emprunts',     'id'), (SELECT max(id) FROM emprunts));

-- -----------------------------------------------------------------------------
-- Premier calcul de la vue matérialisée de popularité (rafraîchie ensuite par pg_cron).
-- -----------------------------------------------------------------------------
REFRESH MATERIALIZED VIEW vue_statistiques_livres;
