package service

import (
	"context"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/validation"
	"github.com/google/uuid"
)

// UtilisateurService porte la logique métier de gestion des comptes (hors
// authentification, gérée par AuthService).
type UtilisateurService struct {
	repo UtilisateurRepo
}

// NouveauUtilisateurService assemble le service avec sa dépendance.
func NouveauUtilisateurService(repo UtilisateurRepo) *UtilisateurService {
	return &UtilisateurService{repo: repo}
}

// Lister renvoie une page d'utilisateurs (réservé aux administrateurs côté route).
func (s *UtilisateurService) Lister(ctx context.Context, params models.ParametresListe) ([]models.Utilisateur, int, error) {
	return s.repo.Lister(ctx, params)
}

// Obtenir renvoie un utilisateur par son identifiant public.
func (s *UtilisateurService) Obtenir(ctx context.Context, uuidCible string) (*models.Utilisateur, error) {
	if !validation.EstUUIDValide(uuidCible) {
		return nil, apperreur.RequeteInvalide("Identifiant d'utilisateur invalide.")
	}
	return s.repo.ParUUID(ctx, uuidCible)
}

// Creer crée un utilisateur avec un rôle explicite (usage administrateur).
func (s *UtilisateurService) Creer(ctx context.Context, entree models.CreerUtilisateurEntree) (*models.Utilisateur, error) {
	email := strings.ToLower(strings.TrimSpace(entree.Email))

	v := validation.Nouveau()
	v.ChampRequis("email", email)
	v.Email("email", email)
	v.LongueurMax("email", email, 254)
	v.ChampRequis("mot_de_passe", entree.MotDePasse)
	v.LongueurMin("mot_de_passe", entree.MotDePasse, 8)
	v.LongueurMax("mot_de_passe", entree.MotDePasse, 72)
	v.ChampRequis("nom", entree.Nom)
	v.LongueurMax("nom", entree.Nom, 100)
	v.ChampRequis("prenom", entree.Prenom)
	v.LongueurMax("prenom", entree.Prenom, 100)
	v.DansEnsemble("role", string(entree.Role), string(models.RoleAdmin), string(models.RoleBibliothecaire), string(models.RoleMembre))
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	hache, err := auth.HacherMotDePasse(entree.MotDePasse)
	if err != nil {
		return nil, apperreur.Interne("hachage du mot de passe").AvecCause(err)
	}

	u := &models.Utilisateur{
		UUID:           uuid.NewString(),
		Email:          email,
		MotDePasseHash: hache,
		Nom:            strings.TrimSpace(entree.Nom),
		Prenom:         strings.TrimSpace(entree.Prenom),
		Role:           entree.Role,
		Actif:          true,
	}
	if err := s.repo.Creer(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// MettreAJour remplace les champs de profil (PUT) : nom et prénom uniquement.
// Le rôle et l'état actif ne sont pas modifiables par cette voie.
func (s *UtilisateurService) MettreAJour(ctx context.Context, uuidCible string, entree models.MettreAJourUtilisateurEntree) (*models.Utilisateur, error) {
	u, err := s.repo.ParUUID(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	v.ChampRequis("nom", entree.Nom)
	v.LongueurMax("nom", entree.Nom, 100)
	v.ChampRequis("prenom", entree.Prenom)
	v.LongueurMax("prenom", entree.Prenom, 100)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	u.Nom = strings.TrimSpace(entree.Nom)
	u.Prenom = strings.TrimSpace(entree.Prenom)
	if err := s.repo.MettreAJour(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Modifier applique une mise à jour PARTIELLE (PATCH). Les champs sensibles
// (role, actif) ne sont pris en compte QUE si l'appelant est administrateur :
// c'est une protection contre l'escalade de privilèges (un membre ne peut pas
// se promouvoir « admin » lui-même).
func (s *UtilisateurService) Modifier(ctx context.Context, uuidCible string, entree models.ModifierUtilisateurEntree, appelantEstAdmin bool) (*models.Utilisateur, error) {
	u, err := s.repo.ParUUID(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	// Un pointeur nil signifie « champ absent » : on ne le touche pas.
	if entree.Nom != nil {
		v.ChampRequis("nom", *entree.Nom)
		v.LongueurMax("nom", *entree.Nom, 100)
		u.Nom = strings.TrimSpace(*entree.Nom)
	}
	if entree.Prenom != nil {
		v.ChampRequis("prenom", *entree.Prenom)
		v.LongueurMax("prenom", *entree.Prenom, 100)
		u.Prenom = strings.TrimSpace(*entree.Prenom)
	}
	if entree.Role != nil {
		if !appelantEstAdmin {
			return nil, apperreur.Interdit("Seul un administrateur peut modifier le rôle.")
		}
		v.DansEnsemble("role", string(*entree.Role), string(models.RoleAdmin), string(models.RoleBibliothecaire), string(models.RoleMembre))
		u.Role = *entree.Role
	}
	if entree.Actif != nil {
		if !appelantEstAdmin {
			return nil, apperreur.Interdit("Seul un administrateur peut activer/désactiver un compte.")
		}
		u.Actif = *entree.Actif
	}
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	if err := s.repo.MettreAJour(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// SupprimerLogique désactive et masque un utilisateur (suppression réversible).
func (s *UtilisateurService) SupprimerLogique(ctx context.Context, uuidCible string) error {
	return s.repo.SupprimerLogique(ctx, uuidCible)
}

// SupprimerPhysique efface définitivement un utilisateur (réservé aux admins).
func (s *UtilisateurService) SupprimerPhysique(ctx context.Context, uuidCible string) error {
	return s.repo.SupprimerPhysique(ctx, uuidCible)
}
