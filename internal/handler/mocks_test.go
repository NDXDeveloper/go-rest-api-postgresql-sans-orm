package handler

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// Doublures en mémoire des interfaces de repository, pour brancher de VRAIS
// services sur des dépendances contrôlées dans les tests de handler.
//
// Remarque : en Go, les fichiers *_test.go ne sont pas importables d'un package à
// l'autre. On reprend donc ici le même motif de mocks que dans
// internal/service/mocks_test.go, restreint aux interfaces utiles aux handlers.
//
// Les assertions garantissent la conformité aux interfaces dès la compilation.
var (
	_ service.UtilisateurRepo = (*mockUtilisateurRepo)(nil)
	_ service.JetonRepo       = (*mockJetonRepo)(nil)
	_ service.AuteurRepo      = (*mockAuteurRepo)(nil)
	_ service.CategorieRepo   = (*mockCategorieRepo)(nil)
	_ service.LivreRepo       = (*mockLivreRepo)(nil)
)

// loggerMuet renvoie un logger qui n'écrit nulle part (tests concentrés sur les
// réponses HTTP).
func loggerMuet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- Utilisateurs ----------------------------------------------------------

type mockUtilisateurRepo struct {
	creer    func(ctx context.Context, u *models.Utilisateur) error
	parUUID  func(ctx context.Context, uuid string) (*models.Utilisateur, error)
	parEmail func(ctx context.Context, email string) (*models.Utilisateur, error)
	parID    func(ctx context.Context, id int64) (*models.Utilisateur, error)
	lister   func(ctx context.Context, p models.ParametresListe) ([]models.Utilisateur, int, error)
}

func (m *mockUtilisateurRepo) Creer(ctx context.Context, u *models.Utilisateur) error {
	if m.creer != nil {
		return m.creer(ctx, u)
	}
	if u.ID == 0 {
		u.ID = 1
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

func (m *mockUtilisateurRepo) MettreAJour(_ context.Context, _ *models.Utilisateur) error { return nil }
func (m *mockUtilisateurRepo) SupprimerLogique(_ context.Context, _ string) error         { return nil }
func (m *mockUtilisateurRepo) SupprimerPhysique(_ context.Context, _ string) error        { return nil }

// --- Jetons ----------------------------------------------------------------

type mockJetonRepo struct {
	enregistrer func(ctx context.Context, utilisateurID int64, jetonHache string, expireLe time.Time) error
	trouver     func(ctx context.Context, jetonHache string) (int64, error)
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

func (m *mockJetonRepo) Revoquer(_ context.Context, _ string) error                   { return nil }
func (m *mockJetonRepo) RevoquerTousPourUtilisateur(_ context.Context, _ int64) error { return nil }

// --- Auteurs ---------------------------------------------------------------

type mockAuteurRepo struct {
	parUUID func(ctx context.Context, uuid string) (*models.Auteur, error)
}

func (m *mockAuteurRepo) Creer(_ context.Context, a *models.Auteur) error {
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

func (m *mockAuteurRepo) Lister(_ context.Context, _ models.ParametresListe) ([]models.Auteur, int, error) {
	return []models.Auteur{}, 0, nil
}
func (m *mockAuteurRepo) MettreAJour(_ context.Context, _ *models.Auteur) error { return nil }
func (m *mockAuteurRepo) Supprimer(_ context.Context, _ string) error           { return nil }

// --- Catégories ------------------------------------------------------------

type mockCategorieRepo struct {
	parUUID func(ctx context.Context, uuid string) (*models.Categorie, error)
}

func (m *mockCategorieRepo) Creer(_ context.Context, c *models.Categorie) error {
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

func (m *mockCategorieRepo) Lister(_ context.Context, _ models.ParametresListe) ([]models.Categorie, int, error) {
	return []models.Categorie{}, 0, nil
}
func (m *mockCategorieRepo) MettreAJour(_ context.Context, _ *models.Categorie) error { return nil }
func (m *mockCategorieRepo) Supprimer(_ context.Context, _ string) error              { return nil }

// --- Livres ----------------------------------------------------------------

type mockLivreRepo struct {
	creer          func(ctx context.Context, l *models.Livre) error
	parUUID        func(ctx context.Context, uuid string) (*models.Livre, error)
	parUUIDInterne func(ctx context.Context, uuid string) (*models.Livre, error)
	lister         func(ctx context.Context, p models.ParametresListe) ([]models.Livre, int, error)
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

func (m *mockLivreRepo) MettreAJour(_ context.Context, _ *models.Livre) error { return nil }
func (m *mockLivreRepo) SupprimerLogique(_ context.Context, _ string) error   { return nil }
func (m *mockLivreRepo) SupprimerPhysique(_ context.Context, _ string) error  { return nil }
