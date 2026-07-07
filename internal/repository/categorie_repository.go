package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// CategorieRepository gère les accès SQL à la table categories.
type CategorieRepository struct {
	db *sql.DB
}

// NouveauCategorieRepository construit le repository avec sa dépendance.
func NouveauCategorieRepository(db *sql.DB) *CategorieRepository {
	return &CategorieRepository{db: db}
}

const colonnesCategorie = `id, uuid, nom, description, cree_le, modifie_le`

func scannerCategorie(ligne ligneScannable) (*models.Categorie, error) {
	var c models.Categorie
	if err := ligne.Scan(&c.ID, &c.UUID, &c.Nom, &c.Description, &c.CreeLe, &c.ModifieLe); err != nil {
		return nil, err
	}
	return &c, nil
}

// Creer insère une catégorie. Le nom étant UNIQUE, un doublon renvoie un 409.
func (r *CategorieRepository) Creer(ctx context.Context, c *models.Categorie) error {
	const requete = `INSERT INTO categories (uuid, nom, description) VALUES ($1, $2, $3) RETURNING id`
	err := r.db.QueryRowContext(ctx, requete, c.UUID, c.Nom, c.Description).Scan(&c.ID)
	if err != nil {
		if database.EstErreurDoublon(err) {
			return apperreur.Conflit("Une catégorie porte déjà ce nom.")
		}
		return apperreur.Interne("création de la catégorie").AvecCause(err)
	}
	return nil
}

// ParUUID récupère une catégorie par identifiant public.
func (r *CategorieRepository) ParUUID(ctx context.Context, uuid string) (*models.Categorie, error) {
	const requete = `SELECT ` + colonnesCategorie + ` FROM categories WHERE uuid = $1`
	c, err := scannerCategorie(r.db.QueryRowContext(ctx, requete, uuid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperreur.NonTrouve("Catégorie introuvable.")
		}
		return nil, apperreur.Interne("lecture de la catégorie").AvecCause(err)
	}
	return c, nil
}

// Lister renvoie toutes les catégories paginées (avec recherche par nom).
func (r *CategorieRepository) Lister(ctx context.Context, params models.ParametresListe) ([]models.Categorie, int, error) {
	var conditions constructeurConditions
	if params.Recherche != "" {
		conditions.ajouter("nom ILIKE ?", "%"+params.Recherche+"%")
	}
	where := conditions.clauseWHERE()

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories `+where, conditions.args...).Scan(&total); err != nil {
		return nil, 0, apperreur.Interne("comptage des catégories").AvecCause(err)
	}
	if total == 0 {
		return []models.Categorie{}, 0, nil
	}

	triPagination, argsPagination := clauseTriEtPagination(params, "nom", &conditions)
	//nolint:gosec // G202 : concaténation sûre — 'where' n'utilise que des paramètres '$N' et 'triPagination' une colonne validée par liste blanche.
	requete := `SELECT ` + colonnesCategorie + ` FROM categories ` + where + triPagination
	lignes, err := r.db.QueryContext(ctx, requete, append(conditions.args, argsPagination...)...)
	if err != nil {
		return nil, 0, apperreur.Interne("liste des catégories").AvecCause(err)
	}
	defer lignes.Close()

	categories := make([]models.Categorie, 0, params.Taille)
	for lignes.Next() {
		c, err := scannerCategorie(lignes)
		if err != nil {
			return nil, 0, apperreur.Interne("lecture d'une ligne catégorie").AvecCause(err)
		}
		categories = append(categories, *c)
	}
	if err := lignes.Err(); err != nil {
		return nil, 0, apperreur.Interne("parcours des catégories").AvecCause(err)
	}
	return categories, total, nil
}

// MettreAJour modifie une catégorie identifiée par UUID.
func (r *CategorieRepository) MettreAJour(ctx context.Context, c *models.Categorie) error {
	const requete = `UPDATE categories SET nom = $1, description = $2 WHERE uuid = $3`
	resultat, err := r.db.ExecContext(ctx, requete, c.Nom, c.Description, c.UUID)
	if err != nil {
		if database.EstErreurDoublon(err) {
			return apperreur.Conflit("Une catégorie porte déjà ce nom.")
		}
		return apperreur.Interne("mise à jour de la catégorie").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Catégorie introuvable.")
}

// Supprimer efface une catégorie (bloquée si des livres l'utilisent).
func (r *CategorieRepository) Supprimer(ctx context.Context, uuid string) error {
	const requete = `DELETE FROM categories WHERE uuid = $1`
	resultat, err := r.db.ExecContext(ctx, requete, uuid)
	if err != nil {
		if database.EstErreurCleEtrangere(err) {
			return apperreur.Conflit("Impossible de supprimer cette catégorie : des livres y sont rattachés.")
		}
		return apperreur.Interne("suppression de la catégorie").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Catégorie introuvable.")
}
