package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// UtilisateurRepository encapsule tous les accès SQL à la table utilisateurs.
//
// Il ne connaît que le pool *sql.DB (injecté à la construction) : aucune variable
// globale, tout est passé explicitement. C'est l'injection de dépendances « à la
// main », sans framework.
type UtilisateurRepository struct {
	db *sql.DB
}

// NouveauUtilisateurRepository construit le repository avec sa dépendance.
func NouveauUtilisateurRepository(db *sql.DB) *UtilisateurRepository {
	return &UtilisateurRepository{db: db}
}

// colonnesUtilisateur liste, dans un ordre stable, les colonnes lues. On la
// réutilise pour toutes les requêtes SELECT afin que l'ordre corresponde
// exactement à celui attendu par scannerUtilisateur.
const colonnesUtilisateur = `id, uuid, email, mot_de_passe_hash, nom, prenom, role, actif, cree_le, modifie_le, supprime_le`

// scannerUtilisateur lit une ligne (issue d'un Row ou de Rows) dans un modèle.
// L'ordre des &pointeurs DOIT suivre celui de colonnesUtilisateur.
func scannerUtilisateur(ligne ligneScannable) (*models.Utilisateur, error) {
	var u models.Utilisateur
	err := ligne.Scan(
		&u.ID, &u.UUID, &u.Email, &u.MotDePasseHash, &u.Nom, &u.Prenom,
		&u.Role, &u.Actif, &u.CreeLe, &u.ModifieLe, &u.SupprimeLe,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Creer insère un nouvel utilisateur. L'UUID et le hash du mot de passe sont
// fournis par la couche service (qui génère l'UUID et hache le mot de passe).
//
// DIFFÉRENCE AVEC MariaDB : PostgreSQL ne fournit pas de LastInsertId(). On
// utilise la clause « RETURNING id » (SQL standard PostgreSQL) pour récupérer la
// valeur générée par la colonne d'identité, directement dans la même requête.
func (r *UtilisateurRepository) Creer(ctx context.Context, u *models.Utilisateur) error {
	const requete = `
		INSERT INTO utilisateurs (uuid, email, mot_de_passe_hash, nom, prenom, role, actif)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	err := r.db.QueryRowContext(ctx, requete,
		u.UUID, u.Email, u.MotDePasseHash, u.Nom, u.Prenom, u.Role, u.Actif).Scan(&u.ID)
	if err != nil {
		// Un e-mail déjà pris viole la contrainte UNIQUE : on renvoie un 409 clair
		// plutôt que l'erreur SQL brute.
		if database.EstErreurDoublon(err) {
			return apperreur.Conflit("Cette adresse e-mail est déjà utilisée.")
		}
		return apperreur.Interne("échec de création de l'utilisateur").AvecCause(err)
	}
	return nil
}

// ParUUID récupère un utilisateur actif (non supprimé) par son identifiant public.
func (r *UtilisateurRepository) ParUUID(ctx context.Context, uuid string) (*models.Utilisateur, error) {
	const requete = `SELECT ` + colonnesUtilisateur + `
		FROM utilisateurs WHERE uuid = $1 AND supprime_le IS NULL`

	u, err := scannerUtilisateur(r.db.QueryRowContext(ctx, requete, uuid))
	if err != nil {
		// sql.ErrNoRows n'est PAS une erreur serveur : c'est un « 404 » métier.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperreur.NonTrouve("Utilisateur introuvable.")
		}
		return nil, apperreur.Interne("lecture de l'utilisateur").AvecCause(err)
	}
	return u, nil
}

// ParEmail récupère un utilisateur par e-mail. Utilisé pour l'authentification.
// On inclut le hash du mot de passe (nécessaire à la vérification par le service).
func (r *UtilisateurRepository) ParEmail(ctx context.Context, email string) (*models.Utilisateur, error) {
	const requete = `SELECT ` + colonnesUtilisateur + `
		FROM utilisateurs WHERE email = $1 AND supprime_le IS NULL`

	u, err := scannerUtilisateur(r.db.QueryRowContext(ctx, requete, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Message volontairement identique à « mauvais mot de passe » côté
			// service, pour ne pas révéler si un e-mail existe (anti-énumération).
			return nil, apperreur.NonTrouve("Utilisateur introuvable.")
		}
		return nil, apperreur.Interne("lecture de l'utilisateur par e-mail").AvecCause(err)
	}
	return u, nil
}

// ParID récupère un utilisateur par sa clé technique interne. Utilisé notamment
// après avoir retrouvé un refresh token (qui référence l'utilisateur par son id).
func (r *UtilisateurRepository) ParID(ctx context.Context, id int64) (*models.Utilisateur, error) {
	const requete = `SELECT ` + colonnesUtilisateur + `
		FROM utilisateurs WHERE id = $1 AND supprime_le IS NULL`
	u, err := scannerUtilisateur(r.db.QueryRowContext(ctx, requete, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperreur.NonTrouve("Utilisateur introuvable.")
		}
		return nil, apperreur.Interne("lecture de l'utilisateur par identifiant").AvecCause(err)
	}
	return u, nil
}

// Lister renvoie une page d'utilisateurs et le nombre TOTAL (toutes pages) pour
// permettre au client de calculer la pagination.
//
// La requête est construite dynamiquement : un filtre de recherche optionnel
// (nom/prénom/e-mail) et un filtre par rôle. On exécute deux requêtes partageant
// les mêmes conditions : un COUNT (total) et un SELECT paginé.
//
// On utilise ILIKE (recherche INSENSIBLE à la casse propre à PostgreSQL) là où
// MariaDB s'appuyait sur LIKE + une collation insensible à la casse : le
// comportement observable reste identique.
func (r *UtilisateurRepository) Lister(ctx context.Context, params models.ParametresListe) ([]models.Utilisateur, int, error) {
	var conditions constructeurConditions
	// On ne liste jamais les comptes supprimés logiquement.
	conditions.ajouter("supprime_le IS NULL")

	if params.Recherche != "" {
		// Le motif ILIKE est passé en PARAMÈTRE : aucune injection possible.
		motif := "%" + params.Recherche + "%"
		conditions.ajouter("(nom ILIKE ? OR prenom ILIKE ? OR email ILIKE ?)", motif, motif, motif)
	}
	if role := params.Filtres["role"]; role != "" {
		// role est un type ENUM : on caste la colonne en text pour la comparer à
		// un paramètre texte (sinon PostgreSQL réclamerait un cast explicite).
		conditions.ajouter("role::text = ?", role)
	}
	where := conditions.clauseWHERE()

	// 1) Total (pour la pagination).
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM utilisateurs `+where, conditions.args...).Scan(&total); err != nil {
		return nil, 0, apperreur.Interne("comptage des utilisateurs").AvecCause(err)
	}
	if total == 0 {
		return []models.Utilisateur{}, 0, nil
	}

	// 2) Page de résultats.
	triPagination, argsPagination := clauseTriEtPagination(params, "cree_le", &conditions)
	//nolint:gosec // G202 : concaténation sûre — 'where' n'utilise que des paramètres '$N' et 'triPagination' une colonne validée par liste blanche.
	requete := `SELECT ` + colonnesUtilisateur + ` FROM utilisateurs ` + where + triPagination
	args := append(conditions.args, argsPagination...)

	lignes, err := r.db.QueryContext(ctx, requete, args...)
	if err != nil {
		return nil, 0, apperreur.Interne("liste des utilisateurs").AvecCause(err)
	}
	defer lignes.Close()

	utilisateurs := make([]models.Utilisateur, 0, params.Taille)
	for lignes.Next() {
		u, err := scannerUtilisateur(lignes)
		if err != nil {
			return nil, 0, apperreur.Interne("lecture d'une ligne utilisateur").AvecCause(err)
		}
		utilisateurs = append(utilisateurs, *u)
	}
	// Toujours vérifier lignes.Err() après la boucle : une erreur d'itération
	// (réseau coupé...) ne se manifeste pas dans lignes.Next().
	if err := lignes.Err(); err != nil {
		return nil, 0, apperreur.Interne("parcours des utilisateurs").AvecCause(err)
	}
	return utilisateurs, total, nil
}

// MettreAJour modifie les champs modifiables d'un utilisateur, identifié par UUID.
// Renvoie NonTrouve si aucune ligne n'a été affectée (UUID inexistant/supprimé).
func (r *UtilisateurRepository) MettreAJour(ctx context.Context, u *models.Utilisateur) error {
	const requete = `
		UPDATE utilisateurs
		   SET nom = $1, prenom = $2, role = $3, actif = $4
		 WHERE uuid = $5 AND supprime_le IS NULL`

	resultat, err := r.db.ExecContext(ctx, requete, u.Nom, u.Prenom, u.Role, u.Actif, u.UUID)
	if err != nil {
		return apperreur.Interne("mise à jour de l'utilisateur").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Utilisateur introuvable.")
}

// SupprimerLogique effectue une suppression LOGIQUE : on horodate supprime_le.
// Les données restent en base (historique, audit) mais l'utilisateur disparaît
// des listes et ne peut plus se connecter.
func (r *UtilisateurRepository) SupprimerLogique(ctx context.Context, uuid string) error {
	const requete = `UPDATE utilisateurs SET supprime_le = NOW(), actif = FALSE
		WHERE uuid = $1 AND supprime_le IS NULL`
	resultat, err := r.db.ExecContext(ctx, requete, uuid)
	if err != nil {
		return apperreur.Interne("suppression logique de l'utilisateur").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Utilisateur introuvable.")
}

// SupprimerPhysique efface DÉFINITIVEMENT la ligne (DELETE). Réservé aux
// administrateurs. Les emprunts liés sont supprimés en cascade (voir la FK).
func (r *UtilisateurRepository) SupprimerPhysique(ctx context.Context, uuid string) error {
	const requete = `DELETE FROM utilisateurs WHERE uuid = $1`
	resultat, err := r.db.ExecContext(ctx, requete, uuid)
	if err != nil {
		return apperreur.Interne("suppression physique de l'utilisateur").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Utilisateur introuvable.")
}

// verifierLigneAffectee renvoie NonTrouve si l'UPDATE/DELETE n'a touché aucune
// ligne (identifiant inexistant). C'est une vérification récurrente, factorisée ici.
func verifierLigneAffectee(resultat sql.Result, messageSiAbsent string) error {
	n, err := resultat.RowsAffected()
	if err != nil {
		return apperreur.Interne("vérification des lignes affectées").AvecCause(err)
	}
	if n == 0 {
		return apperreur.NonTrouve(messageSiAbsent)
	}
	return nil
}
