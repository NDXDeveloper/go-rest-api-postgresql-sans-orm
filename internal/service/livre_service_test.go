package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/google/uuid"
)

// entreeLivreValide renvoie une entrée de création de livre valide, réutilisée et
// adaptée par chaque sous-test.
func entreeLivreValide() models.CreerLivreEntree {
	return models.CreerLivreEntree{
		Titre:             "Les Misérables",
		ISBN:              "9782010000003", // ISBN-13 valide
		AuteurID:          uuid.NewString(),
		CategorieID:       uuid.NewString(),
		AnneePublication:  1862,
		NombreExemplaires: 4,
		Prix:              12.90,
		Langue:            "français",
	}
}

// TestCreerLivre couvre la création : références inexistantes, ISBN invalide et
// cas nominal.
func TestCreerLivre(t *testing.T) {
	t.Run("auteur inexistant → VALIDATION (422)", func(t *testing.T) {
		lRepo := &mockLivreRepo{}
		// L'auteur demandé n'existe pas.
		aRepo := &mockAuteurRepo{
			parUUID: func(_ context.Context, _ string) (*models.Auteur, error) {
				return nil, apperreur.NonTrouve("Auteur introuvable.")
			},
		}
		cRepo := &mockCategorieRepo{
			parUUID: func(_ context.Context, _ string) (*models.Categorie, error) {
				return &models.Categorie{ID: 3}, nil
			},
		}
		svc := NouveauLivreService(lRepo, aRepo, cRepo)

		_, err := svc.Creer(context.Background(), entreeLivreValide())
		appErr := codeErreur(t, err)
		if appErr.Code != apperreur.CodeValidation {
			t.Errorf("code = %q ; attendu VALIDATION", appErr.Code)
		}
		if appErr.StatutHTTP != http.StatusUnprocessableEntity {
			t.Errorf("StatutHTTP = %d ; attendu 422", appErr.StatutHTTP)
		}
		if appErr.Details["auteur_id"] == "" {
			t.Error("le détail devrait cibler le champ auteur_id")
		}
	})

	t.Run("ISBN invalide → VALIDATION", func(t *testing.T) {
		svc := NouveauLivreService(&mockLivreRepo{}, &mockAuteurRepo{}, &mockCategorieRepo{})

		entree := entreeLivreValide()
		entree.ISBN = "9782010000004" // clé de contrôle fausse

		_, err := svc.Creer(context.Background(), entree)
		appErr := codeErreur(t, err)
		if appErr.Code != apperreur.CodeValidation {
			t.Errorf("code = %q ; attendu VALIDATION", appErr.Code)
		}
		if appErr.Details["isbn"] == "" {
			t.Error("le détail devrait cibler le champ isbn")
		}
	})

	t.Run("cas nominal → livre créé", func(t *testing.T) {
		var livreCree *models.Livre
		lRepo := &mockLivreRepo{
			creer: func(_ context.Context, l *models.Livre) error {
				l.ID = 50
				livreCree = l
				return nil
			},
			parUUID: func(_ context.Context, uuidCible string) (*models.Livre, error) {
				// Simule la relecture via la vue (avec auteur/catégorie).
				return &models.Livre{
					UUID:             uuidCible,
					Titre:            "Les Misérables",
					ISBN:             "9782010000003",
					AuteurNomComplet: "Victor Hugo",
					CategorieNom:     "Roman",
				}, nil
			},
		}
		aRepo := &mockAuteurRepo{
			parUUID: func(_ context.Context, _ string) (*models.Auteur, error) {
				return &models.Auteur{ID: 5, Nom: "Hugo", Prenom: "Victor"}, nil
			},
		}
		cRepo := &mockCategorieRepo{
			parUUID: func(_ context.Context, _ string) (*models.Categorie, error) {
				return &models.Categorie{ID: 3, Nom: "Roman"}, nil
			},
		}
		svc := NouveauLivreService(lRepo, aRepo, cRepo)

		livre, err := svc.Creer(context.Background(), entreeLivreValide())
		if err != nil {
			t.Fatalf("Creer a échoué : %v", err)
		}
		if livre == nil || livre.Titre != "Les Misérables" {
			t.Fatalf("livre inattendu : %+v", livre)
		}
		// Vérifie que les clés internes résolues ont bien été posées avant l'INSERT.
		if livreCree == nil || livreCree.AuteurID != 5 || livreCree.CategorieID != 3 {
			t.Errorf("clés internes non résolues : %+v", livreCree)
		}
		// À la création, tous les exemplaires sont disponibles.
		if livreCree.ExemplairesDisponibles != livreCree.NombreExemplaires {
			t.Errorf("exemplaires_disponibles = %d ; attendu %d", livreCree.ExemplairesDisponibles, livreCree.NombreExemplaires)
		}
	})
}

// TestAjusterStock vérifie le recalcul du stock disponible lorsqu'on change le
// nombre total d'exemplaires, en préservant le nombre d'exemplaires empruntés.
func TestAjusterStock(t *testing.T) {
	cas := []struct {
		nom          string
		total        int // NombreExemplaires actuel
		dispo        int // ExemplairesDisponibles actuel
		nouveauTotal int
		attenduDispo int
		attenduErr   bool
	}{
		{"augmentation du stock", 5, 3, 7, 5, false},     // 2 empruntés préservés
		{"diminution compatible", 5, 3, 3, 1, false},     // 2 empruntés préservés
		{"nouveau total = empruntés", 5, 3, 2, 0, false}, // pile 2 empruntés, 0 dispo
		{"total sous les empruntés → conflit", 5, 3, 1, 0, true},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			livre := &models.Livre{NombreExemplaires: c.total, ExemplairesDisponibles: c.dispo}
			dispo, err := ajusterStock(livre, c.nouveauTotal)

			if c.attenduErr {
				appErr := codeErreur(t, err)
				if appErr.Code != apperreur.CodeConflit {
					t.Errorf("code = %q ; attendu CONFLIT", appErr.Code)
				}
				return
			}
			if err != nil {
				t.Fatalf("ajusterStock a échoué : %v", err)
			}
			if dispo != c.attenduDispo {
				t.Errorf("disponibles = %d ; attendu %d", dispo, c.attenduDispo)
			}
		})
	}
}
