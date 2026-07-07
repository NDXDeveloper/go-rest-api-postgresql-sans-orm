package service

import (
	"context"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/validation"
	"github.com/google/uuid"
)

// AuteurService porte la logique métier des auteurs.
type AuteurService struct {
	repo AuteurRepo
}

// NouveauAuteurService assemble le service avec sa dépendance.
func NouveauAuteurService(repo AuteurRepo) *AuteurService {
	return &AuteurService{repo: repo}
}

// Lister renvoie une page d'auteurs.
func (s *AuteurService) Lister(ctx context.Context, params models.ParametresListe) ([]models.Auteur, int, error) {
	return s.repo.Lister(ctx, params)
}

// Obtenir renvoie un auteur par identifiant public.
func (s *AuteurService) Obtenir(ctx context.Context, uuidCible string) (*models.Auteur, error) {
	if !validation.EstUUIDValide(uuidCible) {
		return nil, apperreur.RequeteInvalide("Identifiant d'auteur invalide.")
	}
	return s.repo.ParUUID(ctx, uuidCible)
}

// validerChampsAuteur factorise les règles de validation communes (nom, prénom,
// nationalité, date de naissance facultative). Le paramètre « prefixeVide »
// permet aux appels PATCH de ne pas exiger la présence.
func validerChampsAuteur(v *validation.Validateur, nom, prenom, nationalite string, dateNaissance *string) {
	v.LongueurMax("nom", nom, 100)
	v.LongueurMax("prenom", prenom, 100)
	v.LongueurMax("nationalite", nationalite, 100)
	if dateNaissance != nil && *dateNaissance != "" {
		v.DateISO("date_naissance", *dateNaissance)
	}
}

// Creer crée un auteur.
func (s *AuteurService) Creer(ctx context.Context, entree models.CreerAuteurEntree) (*models.Auteur, error) {
	v := validation.Nouveau()
	v.ChampRequis("nom", entree.Nom)
	validerChampsAuteur(v, entree.Nom, entree.Prenom, entree.Nationalite, entree.DateNaissance)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	a := &models.Auteur{
		UUID:          uuid.NewString(),
		Nom:           strings.TrimSpace(entree.Nom),
		Prenom:        strings.TrimSpace(entree.Prenom),
		Nationalite:   strings.TrimSpace(entree.Nationalite),
		DateNaissance: entree.DateNaissance,
		Biographie:    entree.Biographie,
	}
	if err := s.repo.Creer(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// MettreAJour remplace tous les champs modifiables d'un auteur (PUT).
func (s *AuteurService) MettreAJour(ctx context.Context, uuidCible string, entree models.MettreAJourAuteurEntree) (*models.Auteur, error) {
	a, err := s.repo.ParUUID(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	v.ChampRequis("nom", entree.Nom)
	validerChampsAuteur(v, entree.Nom, entree.Prenom, entree.Nationalite, entree.DateNaissance)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	a.Nom = strings.TrimSpace(entree.Nom)
	a.Prenom = strings.TrimSpace(entree.Prenom)
	a.Nationalite = strings.TrimSpace(entree.Nationalite)
	a.DateNaissance = entree.DateNaissance
	a.Biographie = entree.Biographie
	if err := s.repo.MettreAJour(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Modifier applique une mise à jour partielle (PATCH).
func (s *AuteurService) Modifier(ctx context.Context, uuidCible string, entree models.ModifierAuteurEntree) (*models.Auteur, error) {
	a, err := s.repo.ParUUID(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	if entree.Nom != nil {
		v.ChampRequis("nom", *entree.Nom)
		v.LongueurMax("nom", *entree.Nom, 100)
		a.Nom = strings.TrimSpace(*entree.Nom)
	}
	if entree.Prenom != nil {
		v.LongueurMax("prenom", *entree.Prenom, 100)
		a.Prenom = strings.TrimSpace(*entree.Prenom)
	}
	if entree.Nationalite != nil {
		v.LongueurMax("nationalite", *entree.Nationalite, 100)
		a.Nationalite = strings.TrimSpace(*entree.Nationalite)
	}
	if entree.DateNaissance != nil {
		if *entree.DateNaissance != "" {
			v.DateISO("date_naissance", *entree.DateNaissance)
		}
		a.DateNaissance = entree.DateNaissance
	}
	if entree.Biographie != nil {
		a.Biographie = *entree.Biographie
	}
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	if err := s.repo.MettreAJour(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Supprimer efface un auteur (bloqué si des livres y sont rattachés).
func (s *AuteurService) Supprimer(ctx context.Context, uuidCible string) error {
	return s.repo.Supprimer(ctx, uuidCible)
}
