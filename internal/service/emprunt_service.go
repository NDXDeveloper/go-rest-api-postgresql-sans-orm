package service

import (
	"context"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/validation"
)

// EmpruntService porte la logique métier des emprunts. Le gros du travail
// (vérification de disponibilité, quota, transaction) vit dans la base
// (procédure stockée + transaction du repository) ; le service valide les
// entrées et orchestre.
type EmpruntService struct {
	repo EmpruntRepo
}

// NouveauEmpruntService assemble le service avec sa dépendance.
func NouveauEmpruntService(repo EmpruntRepo) *EmpruntService {
	return &EmpruntService{repo: repo}
}

// dureeParDefautJours est appliquée quand le client ne précise pas de durée.
const dureeParDefautJours = 14

// Emprunter enregistre un emprunt pour l'utilisateur donné (son UUID provient du
// jeton pour un membre, ou d'un membre désigné par un bibliothécaire).
func (s *EmpruntService) Emprunter(ctx context.Context, utilisateurUUID string, entree models.EmprunterEntree) (*models.Emprunt, error) {
	v := validation.Nouveau()
	v.ChampRequis("livre_id", entree.LivreID)
	v.UUID("livre_id", entree.LivreID)
	if entree.DureeJours != 0 {
		v.EntierDans("duree_jours", entree.DureeJours, 1, 90)
	}
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	duree := entree.DureeJours
	if duree == 0 {
		duree = dureeParDefautJours
	}

	// L'appel à la procédure stockée gère la transaction et les vérifications
	// (disponibilité, quota) côté base ; il renvoie l'UUID de l'emprunt créé.
	empruntUUID, err := s.repo.Emprunter(ctx, utilisateurUUID, entree.LivreID, duree)
	if err != nil {
		return nil, err
	}
	return s.repo.ParUUID(ctx, empruntUUID)
}

// Rendre enregistre le retour d'un emprunt et renvoie l'emprunt mis à jour
// (statut « rendu », pénalité éventuelle calculée).
func (s *EmpruntService) Rendre(ctx context.Context, empruntUUID string) (*models.Emprunt, error) {
	if !validation.EstUUIDValide(empruntUUID) {
		return nil, apperreur.RequeteInvalide("Identifiant d'emprunt invalide.")
	}
	if _, err := s.repo.Rendre(ctx, empruntUUID); err != nil {
		return nil, err
	}
	return s.repo.ParUUID(ctx, empruntUUID)
}

// Obtenir renvoie un emprunt par identifiant public.
func (s *EmpruntService) Obtenir(ctx context.Context, empruntUUID string) (*models.Emprunt, error) {
	if !validation.EstUUIDValide(empruntUUID) {
		return nil, apperreur.RequeteInvalide("Identifiant d'emprunt invalide.")
	}
	return s.repo.ParUUID(ctx, empruntUUID)
}

// Lister renvoie une page d'emprunts. Si utilisateurUUID est renseigné, la liste
// est restreinte aux emprunts de cet utilisateur (cas d'un membre consultant les
// siens). Vide => tous les emprunts (usage bibliothécaire/admin).
func (s *EmpruntService) Lister(ctx context.Context, utilisateurUUID string, params models.ParametresListe) ([]models.Emprunt, int, error) {
	return s.repo.Lister(ctx, utilisateurUUID, params)
}

// Statistiques renvoie les indicateurs d'emprunt d'un utilisateur (via la
// procédure stockée à paramètres OUT).
func (s *EmpruntService) Statistiques(ctx context.Context, utilisateurUUID string) (models.StatistiquesUtilisateur, error) {
	if !validation.EstUUIDValide(utilisateurUUID) {
		return models.StatistiquesUtilisateur{}, apperreur.RequeteInvalide("Identifiant d'utilisateur invalide.")
	}
	return s.repo.StatistiquesUtilisateur(ctx, utilisateurUUID)
}
