package service

import (
	"context"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/validation"
	"github.com/google/uuid"
)

// CategorieService porte la logique métier des catégories.
type CategorieService struct {
	repo CategorieRepo
}

// NouveauCategorieService assemble le service avec sa dépendance.
func NouveauCategorieService(repo CategorieRepo) *CategorieService {
	return &CategorieService{repo: repo}
}

// Lister renvoie une page de catégories.
func (s *CategorieService) Lister(ctx context.Context, params models.ParametresListe) ([]models.Categorie, int, error) {
	return s.repo.Lister(ctx, params)
}

// Obtenir renvoie une catégorie par identifiant public.
func (s *CategorieService) Obtenir(ctx context.Context, uuidCible string) (*models.Categorie, error) {
	if !validation.EstUUIDValide(uuidCible) {
		return nil, apperreur.RequeteInvalide("Identifiant de catégorie invalide.")
	}
	return s.repo.ParUUID(ctx, uuidCible)
}

// Creer crée une catégorie.
func (s *CategorieService) Creer(ctx context.Context, entree models.CreerCategorieEntree) (*models.Categorie, error) {
	v := validation.Nouveau()
	v.ChampRequis("nom", entree.Nom)
	v.LongueurMax("nom", entree.Nom, 100)
	v.LongueurMax("description", entree.Description, 500)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	c := &models.Categorie{
		UUID:        uuid.NewString(),
		Nom:         strings.TrimSpace(entree.Nom),
		Description: strings.TrimSpace(entree.Description),
	}
	if err := s.repo.Creer(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// MettreAJour remplace les champs d'une catégorie (PUT).
func (s *CategorieService) MettreAJour(ctx context.Context, uuidCible string, entree models.MettreAJourCategorieEntree) (*models.Categorie, error) {
	c, err := s.repo.ParUUID(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	v.ChampRequis("nom", entree.Nom)
	v.LongueurMax("nom", entree.Nom, 100)
	v.LongueurMax("description", entree.Description, 500)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	c.Nom = strings.TrimSpace(entree.Nom)
	c.Description = strings.TrimSpace(entree.Description)
	if err := s.repo.MettreAJour(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Modifier applique une mise à jour partielle (PATCH).
func (s *CategorieService) Modifier(ctx context.Context, uuidCible string, entree models.ModifierCategorieEntree) (*models.Categorie, error) {
	c, err := s.repo.ParUUID(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	if entree.Nom != nil {
		v.ChampRequis("nom", *entree.Nom)
		v.LongueurMax("nom", *entree.Nom, 100)
		c.Nom = strings.TrimSpace(*entree.Nom)
	}
	if entree.Description != nil {
		v.LongueurMax("description", *entree.Description, 500)
		c.Description = strings.TrimSpace(*entree.Description)
	}
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	if err := s.repo.MettreAJour(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Supprimer efface une catégorie (bloquée si des livres l'utilisent).
func (s *CategorieService) Supprimer(ctx context.Context, uuidCible string) error {
	return s.repo.Supprimer(ctx, uuidCible)
}
