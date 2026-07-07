package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/service"
	"github.com/google/uuid"
)

// nouveauLivreHandlerTest assemble un LivreHandler branché sur de vrais services et
// des repositories mockés.
func nouveauLivreHandlerTest(l service.LivreRepo, a service.AuteurRepo, c service.CategorieRepo) *LivreHandler {
	svc := service.NouveauLivreService(l, a, c)
	return NouveauLivreHandler(svc, loggerMuet())
}

// TestListerLivresHandler exerce GET /livres et vérifie la présence des
// métadonnées de pagination.
func TestListerLivresHandler(t *testing.T) {
	lRepo := &mockLivreRepo{
		lister: func(_ context.Context, _ models.ParametresListe) ([]models.Livre, int, error) {
			return []models.Livre{
				{UUID: uuid.NewString(), Titre: "Les Misérables"},
				{UUID: uuid.NewString(), Titre: "1984"},
			}, 2, nil
		},
	}
	h := nouveauLivreHandlerTest(lRepo, &mockAuteurRepo{}, &mockCategorieRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/livres?page=1&taille=20", nil)
	rec := httptest.NewRecorder()

	h.Lister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("statut = %d ; attendu 200 (corps : %s)", rec.Code, rec.Body.String())
	}
	env := decoderEnveloppe(t, rec)
	if !env.Succes {
		t.Error("succes = false ; attendu true")
	}
	if env.Meta == nil {
		t.Fatal("les métadonnées de pagination sont absentes")
	}
	if env.Meta.TotalElements != 2 {
		t.Errorf("total_elements = %d ; attendu 2", env.Meta.TotalElements)
	}
	if !strings.Contains(string(env.Donnees), "Les Misérables") {
		t.Errorf("la liste devrait contenir les livres : %s", env.Donnees)
	}
}

// TestCreerLivreHandler exerce POST /livres : corps invalide, champ inconnu, et
// référence d'auteur inexistante.
func TestCreerLivreHandler(t *testing.T) {
	t.Run("corps JSON invalide → 400", func(t *testing.T) {
		h := nouveauLivreHandlerTest(&mockLivreRepo{}, &mockAuteurRepo{}, &mockCategorieRepo{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/livres", strings.NewReader(`{pas du JSON`))
		rec := httptest.NewRecorder()

		h.Creer(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("statut = %d ; attendu 400", rec.Code)
		}
	})

	t.Run("champ inconnu → 400 (protection Mass Assignment)", func(t *testing.T) {
		h := nouveauLivreHandlerTest(&mockLivreRepo{}, &mockAuteurRepo{}, &mockCategorieRepo{})
		// « exemplaires_disponibles » n'est pas un champ d'entrée autorisé.
		corps := `{"titre":"X","isbn":"9782010000003","auteur_id":"` + uuid.NewString() +
			`","categorie_id":"` + uuid.NewString() + `","exemplaires_disponibles":9999}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/livres", strings.NewReader(corps))
		rec := httptest.NewRecorder()

		h.Creer(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("statut = %d ; attendu 400 (corps : %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("auteur inexistant → 422 VALIDATION", func(t *testing.T) {
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
		h := nouveauLivreHandlerTest(&mockLivreRepo{}, aRepo, cRepo)

		corps := `{"titre":"Les Misérables","isbn":"9782010000003","auteur_id":"` + uuid.NewString() +
			`","categorie_id":"` + uuid.NewString() + `","annee_publication":1862,"nombre_exemplaires":4,"prix":12.9}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/livres", strings.NewReader(corps))
		rec := httptest.NewRecorder()

		h.Creer(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("statut = %d ; attendu 422 (corps : %s)", rec.Code, rec.Body.String())
		}
		env := decoderEnveloppe(t, rec)
		if env.Erreur == nil || env.Erreur.Code != string(apperreur.CodeValidation) {
			t.Errorf("code attendu VALIDATION : %+v", env.Erreur)
		}
	})
}
