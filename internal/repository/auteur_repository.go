package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// AuteurRepository gère les accès SQL à la table auteurs.
// Même structure et mêmes principes que UtilisateurRepository (voir ce dernier
// pour les explications détaillées sur les requêtes préparées et le scan).
type AuteurRepository struct {
	db *sql.DB
}

// NouveauAuteurRepository construit le repository avec sa dépendance.
func NouveauAuteurRepository(db *sql.DB) *AuteurRepository {
	return &AuteurRepository{db: db}
}

const colonnesAuteur = `id, uuid, nom, prenom, nationalite, date_naissance, biographie, cree_le, modifie_le`

// scannerAuteur lit une ligne auteur. La date de naissance (DATE nullable) passe
// par une variable sql.NullTime intermédiaire avant d'être formatée en *string.
func scannerAuteur(ligne ligneScannable) (*models.Auteur, error) {
	var a models.Auteur
	var dateNaissance sql.NullTime
	var biographie sql.NullString

	err := ligne.Scan(
		&a.ID, &a.UUID, &a.Nom, &a.Prenom, &a.Nationalite,
		&dateNaissance, &biographie, &a.CreeLe, &a.ModifieLe,
	)
	if err != nil {
		return nil, err
	}
	a.DateNaissance = dateVersChaine(dateNaissance)
	a.Biographie = biographie.String
	return &a, nil
}

// Creer insère un auteur (UUID généré par la couche service). « RETURNING id »
// récupère l'identifiant généré (PostgreSQL n'a pas de LastInsertId).
func (r *AuteurRepository) Creer(ctx context.Context, a *models.Auteur) error {
	const requete = `INSERT INTO auteurs (uuid, nom, prenom, nationalite, date_naissance, biographie)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err := r.db.QueryRowContext(ctx, requete,
		a.UUID, a.Nom, a.Prenom, a.Nationalite, argDate(a.DateNaissance), a.Biographie).Scan(&a.ID)
	if err != nil {
		return apperreur.Interne("création de l'auteur").AvecCause(err)
	}
	return nil
}

// ParUUID récupère un auteur par son identifiant public.
func (r *AuteurRepository) ParUUID(ctx context.Context, uuid string) (*models.Auteur, error) {
	const requete = `SELECT ` + colonnesAuteur + ` FROM auteurs WHERE uuid = $1`
	a, err := scannerAuteur(r.db.QueryRowContext(ctx, requete, uuid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperreur.NonTrouve("Auteur introuvable.")
		}
		return nil, apperreur.Interne("lecture de l'auteur").AvecCause(err)
	}
	return a, nil
}

// Lister renvoie une page d'auteurs et le total, avec recherche par nom/prénom.
func (r *AuteurRepository) Lister(ctx context.Context, params models.ParametresListe) ([]models.Auteur, int, error) {
	var conditions constructeurConditions
	if params.Recherche != "" {
		motif := "%" + params.Recherche + "%"
		conditions.ajouter("(nom ILIKE ? OR prenom ILIKE ?)", motif, motif)
	}
	if nat := params.Filtres["nationalite"]; nat != "" {
		conditions.ajouter("nationalite = ?", nat)
	}
	where := conditions.clauseWHERE()

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auteurs `+where, conditions.args...).Scan(&total); err != nil {
		return nil, 0, apperreur.Interne("comptage des auteurs").AvecCause(err)
	}
	if total == 0 {
		return []models.Auteur{}, 0, nil
	}

	triPagination, argsPagination := clauseTriEtPagination(params, "nom", &conditions)
	//nolint:gosec // G202 : concaténation sûre — 'where' n'utilise que des paramètres '$N' et 'triPagination' une colonne validée par liste blanche.
	requete := `SELECT ` + colonnesAuteur + ` FROM auteurs ` + where + triPagination
	lignes, err := r.db.QueryContext(ctx, requete, append(conditions.args, argsPagination...)...)
	if err != nil {
		return nil, 0, apperreur.Interne("liste des auteurs").AvecCause(err)
	}
	defer lignes.Close()

	auteurs := make([]models.Auteur, 0, params.Taille)
	for lignes.Next() {
		a, err := scannerAuteur(lignes)
		if err != nil {
			return nil, 0, apperreur.Interne("lecture d'une ligne auteur").AvecCause(err)
		}
		auteurs = append(auteurs, *a)
	}
	if err := lignes.Err(); err != nil {
		return nil, 0, apperreur.Interne("parcours des auteurs").AvecCause(err)
	}
	return auteurs, total, nil
}

// MettreAJour modifie un auteur identifié par UUID.
func (r *AuteurRepository) MettreAJour(ctx context.Context, a *models.Auteur) error {
	const requete = `UPDATE auteurs SET nom = $1, prenom = $2, nationalite = $3, date_naissance = $4, biographie = $5
		WHERE uuid = $6`
	resultat, err := r.db.ExecContext(ctx, requete,
		a.Nom, a.Prenom, a.Nationalite, argDate(a.DateNaissance), a.Biographie, a.UUID)
	if err != nil {
		return apperreur.Interne("mise à jour de l'auteur").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Auteur introuvable.")
}

// Supprimer efface un auteur. La contrainte FK (ON DELETE RESTRICT) empêche la
// suppression d'un auteur encore lié à des livres : on traduit alors l'erreur SQL
// en 409 Conflit explicite.
func (r *AuteurRepository) Supprimer(ctx context.Context, uuid string) error {
	const requete = `DELETE FROM auteurs WHERE uuid = $1`
	resultat, err := r.db.ExecContext(ctx, requete, uuid)
	if err != nil {
		if database.EstErreurCleEtrangere(err) {
			return apperreur.Conflit("Impossible de supprimer cet auteur : des livres y sont rattachés.")
		}
		return apperreur.Interne("suppression de l'auteur").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Auteur introuvable.")
}
