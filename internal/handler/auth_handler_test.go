package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/config"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// enveloppeTest permet de désérialiser l'enveloppe JSON commune des réponses.
type enveloppeTest struct {
	Succes  bool            `json:"succes"`
	Donnees json.RawMessage `json:"donnees"`
	Meta    *struct {
		Page          int `json:"page"`
		TailleParPage int `json:"taille_par_page"`
		TotalElements int `json:"total_elements"`
		TotalPages    int `json:"total_pages"`
	} `json:"meta"`
	Erreur *struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	} `json:"erreur"`
}

// decoderEnveloppe lit le corps d'une réponse de test dans une enveloppeTest.
func decoderEnveloppe(t *testing.T, rec *httptest.ResponseRecorder) enveloppeTest {
	t.Helper()
	var env enveloppeTest
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corps JSON illisible (%q) : %v", rec.Body.String(), err)
	}
	return env
}

// gestionnaireJWTTest fabrique un GestionnaireJWT valide pour les tests.
func gestionnaireJWTTest() *auth.GestionnaireJWT {
	return auth.NouveauGestionnaireJWT(config.JWT{
		Secret:                "secret-de-test-suffisamment-long-0123456789",
		Emetteur:              "api-bibliotheque-test",
		DureeAcces:            15 * time.Minute,
		DureeRafraichissement: 24 * time.Hour,
	})
}

// TestInscriptionHandler exerce le point d'entrée POST /auth/inscription.
func TestInscriptionHandler(t *testing.T) {
	construire := func(uRepo service.UtilisateurRepo) *AuthHandler {
		svc := service.NouveauAuthService(uRepo, &mockJetonRepo{}, gestionnaireJWTTest())
		return NouveauAuthHandler(svc, loggerMuet())
	}

	t.Run("inscription valide → 201 Created, rôle membre", func(t *testing.T) {
		h := construire(&mockUtilisateurRepo{})
		corps := `{"email":"chloe.durand@exemple.fr","mot_de_passe":"MotDePasse123!","nom":"Durand","prenom":"Chloé"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/inscription", strings.NewReader(corps))
		rec := httptest.NewRecorder()

		h.Inscription(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("statut = %d ; attendu 201 (corps : %s)", rec.Code, rec.Body.String())
		}
		env := decoderEnveloppe(t, rec)
		if !env.Succes {
			t.Error("succes = false ; attendu true")
		}
		if !strings.Contains(string(env.Donnees), `"role":"membre"`) {
			t.Errorf("le nouvel inscrit devrait avoir le rôle membre : %s", env.Donnees)
		}
	})

	t.Run("corps JSON invalide → 400", func(t *testing.T) {
		h := construire(&mockUtilisateurRepo{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/inscription", strings.NewReader(`{ceci n'est pas du JSON`))
		rec := httptest.NewRecorder()

		h.Inscription(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("statut = %d ; attendu 400", rec.Code)
		}
	})

	t.Run("champ inconnu → 400 (protection Mass Assignment)", func(t *testing.T) {
		h := construire(&mockUtilisateurRepo{})
		// « role » n'existe pas dans InscriptionEntree : DisallowUnknownFields rejette.
		corps := `{"email":"x@exemple.fr","mot_de_passe":"MotDePasse123!","nom":"X","prenom":"Y","role":"admin"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/inscription", strings.NewReader(corps))
		rec := httptest.NewRecorder()

		h.Inscription(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("statut = %d ; attendu 400", rec.Code)
		}
		env := decoderEnveloppe(t, rec)
		if env.Erreur == nil || !strings.Contains(env.Erreur.Message, "role") {
			t.Errorf("le message devrait signaler le champ non autorisé : %+v", env.Erreur)
		}
	})
}

// TestConnexionHandler exerce le point d'entrée POST /auth/connexion.
func TestConnexionHandler(t *testing.T) {
	t.Run("mauvais mot de passe → 401", func(t *testing.T) {
		hache, err := auth.HacherMotDePasse("MotDePasse123!")
		if err != nil {
			t.Fatalf("HacherMotDePasse : %v", err)
		}
		uRepo := &mockUtilisateurRepo{
			parEmail: func(_ context.Context, _ string) (*models.Utilisateur, error) {
				return &models.Utilisateur{
					ID: 1, UUID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
					Email: "membre@exemple.fr", MotDePasseHash: hache,
					Role: models.RoleMembre, Actif: true,
				}, nil
			},
		}
		svc := service.NouveauAuthService(uRepo, &mockJetonRepo{}, gestionnaireJWTTest())
		h := NouveauAuthHandler(svc, loggerMuet())

		corps := `{"email":"membre@exemple.fr","mot_de_passe":"MauvaisMotDePasse"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/connexion", strings.NewReader(corps))
		rec := httptest.NewRecorder()

		h.Connexion(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("statut = %d ; attendu 401", rec.Code)
		}
		env := decoderEnveloppe(t, rec)
		if env.Succes {
			t.Error("succes = true ; attendu false")
		}
	})
}
