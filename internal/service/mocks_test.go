package service

import (
	"context"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// Ce fichier fournit des doublures (« mocks ») EN MÉMOIRE des interfaces de
// repository déclarées dans interfaces.go. Chaque mock expose des champs FONCTION
// configurables : un test ne renseigne que les comportements dont il a besoin.
// Lorsqu'un champ n'est pas renseigné, un comportement par défaut raisonnable
// s'applique (souvent « NonTrouve » pour les lectures, succès pour les écritures).
//
// Les assertions ci-dessous garantissent, dès la compilation, que chaque mock
// implémente bien l'interface correspondante.
var (
	_ UtilisateurRepo = (*mockUtilisateurRepo)(nil)
	_ JetonRepo       = (*mockJetonRepo)(nil)
	_ AuteurRepo      = (*mockAuteurRepo)(nil)
	_ CategorieRepo   = (*mockCategorieRepo)(nil)
	_ LivreRepo       = (*mockLivreRepo)(nil)
	_ EmpruntRepo     = (*mockEmpruntRepo)(nil)
)

// --- Utilisateurs ----------------------------------------------------------

type mockUtilisateurRepo struct {
	creer             func(ctx context.Context, u *models.Utilisateur) error
	parUUID           func(ctx context.Context, uuid string) (*models.Utilisateur, error)
	parEmail          func(ctx context.Context, email string) (*models.Utilisateur, error)
	parID             func(ctx context.Context, id int64) (*models.Utilisateur, error)
	lister            func(ctx context.Context, p models.ParametresListe) ([]models.Utilisateur, int, error)
	mettreAJour       func(ctx context.Context, u *models.Utilisateur) error
	supprimerLogique  func(ctx context.Context, uuid string) error
	supprimerPhysique func(ctx context.Context, uuid string) error
}

func (m *mockUtilisateurRepo) Creer(ctx context.Context, u *models.Utilisateur) error {
	if m.creer != nil {
		return m.creer(ctx, u)
	}
	if u.ID == 0 {
		u.ID = 1 // simule l'auto-incrément
	}
	return nil
}

func (m *mockUtilisateurRepo) ParUUID(ctx context.Context, uuid string) (*models.Utilisateur, error) {
	if m.parUUID != nil {
		return m.parUUID(ctx, uuid)
	}
	return nil, apperreur.NonTrouve("Utilisateur introuvable.")
}

func (m *mockUtilisateurRepo) ParEmail(ctx context.Context, email string) (*models.Utilisateur, error) {
	if m.parEmail != nil {
		return m.parEmail(ctx, email)
	}
	return nil, apperreur.NonTrouve("Utilisateur introuvable.")
}

func (m *mockUtilisateurRepo) ParID(ctx context.Context, id int64) (*models.Utilisateur, error) {
	if m.parID != nil {
		return m.parID(ctx, id)
	}
	return nil, apperreur.NonTrouve("Utilisateur introuvable.")
}

func (m *mockUtilisateurRepo) Lister(ctx context.Context, p models.ParametresListe) ([]models.Utilisateur, int, error) {
	if m.lister != nil {
		return m.lister(ctx, p)
	}
	return []models.Utilisateur{}, 0, nil
}

func (m *mockUtilisateurRepo) MettreAJour(ctx context.Context, u *models.Utilisateur) error {
	if m.mettreAJour != nil {
		return m.mettreAJour(ctx, u)
	}
	return nil
}

func (m *mockUtilisateurRepo) SupprimerLogique(ctx context.Context, uuid string) error {
	if m.supprimerLogique != nil {
		return m.supprimerLogique(ctx, uuid)
	}
	return nil
}

func (m *mockUtilisateurRepo) SupprimerPhysique(ctx context.Context, uuid string) error {
	if m.supprimerPhysique != nil {
		return m.supprimerPhysique(ctx, uuid)
	}
	return nil
}

// --- Jetons de rafraîchissement --------------------------------------------

type mockJetonRepo struct {
	enregistrer  func(ctx context.Context, utilisateurID int64, jetonHache string, expireLe time.Time) error
	trouver      func(ctx context.Context, jetonHache string) (int64, error)
	revoquer     func(ctx context.Context, jetonHache string) error
	revoquerTous func(ctx context.Context, utilisateurID int64) error
}

func (m *mockJetonRepo) Enregistrer(ctx context.Context, utilisateurID int64, jetonHache string, expireLe time.Time) error {
	if m.enregistrer != nil {
		return m.enregistrer(ctx, utilisateurID, jetonHache, expireLe)
	}
	return nil
}

func (m *mockJetonRepo) TrouverUtilisateurValide(ctx context.Context, jetonHache string) (int64, error) {
	if m.trouver != nil {
		return m.trouver(ctx, jetonHache)
	}
	return 0, apperreur.NonAuthentifie("Jeton de rafraîchissement invalide ou expiré.")
}

func (m *mockJetonRepo) Revoquer(ctx context.Context, jetonHache string) error {
	if m.revoquer != nil {
		return m.revoquer(ctx, jetonHache)
	}
	return nil
}

func (m *mockJetonRepo) RevoquerTousPourUtilisateur(ctx context.Context, utilisateurID int64) error {
	if m.revoquerTous != nil {
		return m.revoquerTous(ctx, utilisateurID)
	}
	return nil
}

// --- Auteurs ---------------------------------------------------------------

type mockAuteurRepo struct {
	creer       func(ctx context.Context, a *models.Auteur) error
	parUUID     func(ctx context.Context, uuid string) (*models.Auteur, error)
	lister      func(ctx context.Context, p models.ParametresListe) ([]models.Auteur, int, error)
	mettreAJour func(ctx context.Context, a *models.Auteur) error
	supprimer   func(ctx context.Context, uuid string) error
}

func (m *mockAuteurRepo) Creer(ctx context.Context, a *models.Auteur) error {
	if m.creer != nil {
		return m.creer(ctx, a)
	}
	if a.ID == 0 {
		a.ID = 1
	}
	return nil
}

func (m *mockAuteurRepo) ParUUID(ctx context.Context, uuid string) (*models.Auteur, error) {
	if m.parUUID != nil {
		return m.parUUID(ctx, uuid)
	}
	return nil, apperreur.NonTrouve("Auteur introuvable.")
}

func (m *mockAuteurRepo) Lister(ctx context.Context, p models.ParametresListe) ([]models.Auteur, int, error) {
	if m.lister != nil {
		return m.lister(ctx, p)
	}
	return []models.Auteur{}, 0, nil
}

func (m *mockAuteurRepo) MettreAJour(ctx context.Context, a *models.Auteur) error {
	if m.mettreAJour != nil {
		return m.mettreAJour(ctx, a)
	}
	return nil
}

func (m *mockAuteurRepo) Supprimer(ctx context.Context, uuid string) error {
	if m.supprimer != nil {
		return m.supprimer(ctx, uuid)
	}
	return nil
}

// --- Catégories ------------------------------------------------------------

type mockCategorieRepo struct {
	creer       func(ctx context.Context, c *models.Categorie) error
	parUUID     func(ctx context.Context, uuid string) (*models.Categorie, error)
	lister      func(ctx context.Context, p models.ParametresListe) ([]models.Categorie, int, error)
	mettreAJour func(ctx context.Context, c *models.Categorie) error
	supprimer   func(ctx context.Context, uuid string) error
}

func (m *mockCategorieRepo) Creer(ctx context.Context, c *models.Categorie) error {
	if m.creer != nil {
		return m.creer(ctx, c)
	}
	if c.ID == 0 {
		c.ID = 1
	}
	return nil
}

func (m *mockCategorieRepo) ParUUID(ctx context.Context, uuid string) (*models.Categorie, error) {
	if m.parUUID != nil {
		return m.parUUID(ctx, uuid)
	}
	return nil, apperreur.NonTrouve("Catégorie introuvable.")
}

func (m *mockCategorieRepo) Lister(ctx context.Context, p models.ParametresListe) ([]models.Categorie, int, error) {
	if m.lister != nil {
		return m.lister(ctx, p)
	}
	return []models.Categorie{}, 0, nil
}

func (m *mockCategorieRepo) MettreAJour(ctx context.Context, c *models.Categorie) error {
	if m.mettreAJour != nil {
		return m.mettreAJour(ctx, c)
	}
	return nil
}

func (m *mockCategorieRepo) Supprimer(ctx context.Context, uuid string) error {
	if m.supprimer != nil {
		return m.supprimer(ctx, uuid)
	}
	return nil
}

// --- Livres ----------------------------------------------------------------

type mockLivreRepo struct {
	creer             func(ctx context.Context, l *models.Livre) error
	parUUID           func(ctx context.Context, uuid string) (*models.Livre, error)
	parUUIDInterne    func(ctx context.Context, uuid string) (*models.Livre, error)
	lister            func(ctx context.Context, p models.ParametresListe) ([]models.Livre, int, error)
	mettreAJour       func(ctx context.Context, l *models.Livre) error
	supprimerLogique  func(ctx context.Context, uuid string) error
	supprimerPhysique func(ctx context.Context, uuid string) error
}

func (m *mockLivreRepo) Creer(ctx context.Context, l *models.Livre) error {
	if m.creer != nil {
		return m.creer(ctx, l)
	}
	if l.ID == 0 {
		l.ID = 1
	}
	return nil
}

func (m *mockLivreRepo) ParUUID(ctx context.Context, uuid string) (*models.Livre, error) {
	if m.parUUID != nil {
		return m.parUUID(ctx, uuid)
	}
	return nil, apperreur.NonTrouve("Livre introuvable.")
}

func (m *mockLivreRepo) ParUUIDInterne(ctx context.Context, uuid string) (*models.Livre, error) {
	if m.parUUIDInterne != nil {
		return m.parUUIDInterne(ctx, uuid)
	}
	return nil, apperreur.NonTrouve("Livre introuvable.")
}

func (m *mockLivreRepo) Lister(ctx context.Context, p models.ParametresListe) ([]models.Livre, int, error) {
	if m.lister != nil {
		return m.lister(ctx, p)
	}
	return []models.Livre{}, 0, nil
}

func (m *mockLivreRepo) MettreAJour(ctx context.Context, l *models.Livre) error {
	if m.mettreAJour != nil {
		return m.mettreAJour(ctx, l)
	}
	return nil
}

func (m *mockLivreRepo) SupprimerLogique(ctx context.Context, uuid string) error {
	if m.supprimerLogique != nil {
		return m.supprimerLogique(ctx, uuid)
	}
	return nil
}

func (m *mockLivreRepo) SupprimerPhysique(ctx context.Context, uuid string) error {
	if m.supprimerPhysique != nil {
		return m.supprimerPhysique(ctx, uuid)
	}
	return nil
}

// --- Emprunts --------------------------------------------------------------

type mockEmpruntRepo struct {
	emprunter    func(ctx context.Context, utilisateurUUID, livreUUID string, dureeJours int) (string, error)
	rendre       func(ctx context.Context, empruntUUID string) (float64, error)
	parUUID      func(ctx context.Context, uuid string) (*models.Emprunt, error)
	lister       func(ctx context.Context, utilisateurUUID string, p models.ParametresListe) ([]models.Emprunt, int, error)
	statistiques func(ctx context.Context, utilisateurUUID string) (models.StatistiquesUtilisateur, error)
}

func (m *mockEmpruntRepo) Emprunter(ctx context.Context, utilisateurUUID, livreUUID string, dureeJours int) (string, error) {
	if m.emprunter != nil {
		return m.emprunter(ctx, utilisateurUUID, livreUUID, dureeJours)
	}
	return "", apperreur.Interne("non configuré")
}

func (m *mockEmpruntRepo) Rendre(ctx context.Context, empruntUUID string) (float64, error) {
	if m.rendre != nil {
		return m.rendre(ctx, empruntUUID)
	}
	return 0, nil
}

func (m *mockEmpruntRepo) ParUUID(ctx context.Context, uuid string) (*models.Emprunt, error) {
	if m.parUUID != nil {
		return m.parUUID(ctx, uuid)
	}
	return nil, apperreur.NonTrouve("Emprunt introuvable.")
}

func (m *mockEmpruntRepo) Lister(ctx context.Context, utilisateurUUID string, p models.ParametresListe) ([]models.Emprunt, int, error) {
	if m.lister != nil {
		return m.lister(ctx, utilisateurUUID, p)
	}
	return []models.Emprunt{}, 0, nil
}

func (m *mockEmpruntRepo) StatistiquesUtilisateur(ctx context.Context, utilisateurUUID string) (models.StatistiquesUtilisateur, error) {
	if m.statistiques != nil {
		return m.statistiques(ctx, utilisateurUUID)
	}
	return models.StatistiquesUtilisateur{}, nil
}
