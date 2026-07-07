package reponse

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
)

// enveloppeTest reflète la structure JSON produite par le package. On la
// redéclare ici (avec des champs exportés) pour pouvoir la désérialiser dans les
// tests, la structure interne « enveloppe » étant privée.
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

// loggerMuet renvoie un logger qui écrit dans le vide : les tests n'ont pas besoin
// d'inspecter les logs, seulement les réponses HTTP.
func loggerMuet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestSucces vérifie une réponse de succès : statut 200, en-tête JSON UTF-8,
// enveloppe { "succes": true, "donnees": ... }.
func TestSucces(t *testing.T) {
	rec := httptest.NewRecorder()
	Succes(rec, http.StatusOK, map[string]string{"titre": "Les Misérables"})

	if rec.Code != http.StatusOK {
		t.Fatalf("statut = %d ; attendu 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q ; attendu application/json", ct)
	}

	var env enveloppeTest
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corps JSON illisible : %v", err)
	}
	if !env.Succes {
		t.Error("succes = false ; attendu true")
	}
	if env.Erreur != nil {
		t.Error("le champ erreur aurait dû être absent")
	}
	if !strings.Contains(string(env.Donnees), "Les Misérables") {
		t.Errorf("donnees ne contient pas la valeur attendue : %s", env.Donnees)
	}
}

// TestSuccesPagine vérifie qu'une réponse paginée expose des métadonnées correctes,
// dont le nombre total de pages correctement calculé.
func TestSuccesPagine(t *testing.T) {
	rec := httptest.NewRecorder()
	// 25 éléments, 10 par page => 3 pages.
	SuccesPagine(rec, http.StatusOK, []int{1, 2, 3}, 1, 10, 25)

	if rec.Code != http.StatusOK {
		t.Fatalf("statut = %d ; attendu 200", rec.Code)
	}

	var env enveloppeTest
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corps JSON illisible : %v", err)
	}
	if env.Meta == nil {
		t.Fatal("les métadonnées de pagination sont absentes")
	}
	if env.Meta.Page != 1 || env.Meta.TailleParPage != 10 || env.Meta.TotalElements != 25 {
		t.Errorf("meta inattendue : %+v", *env.Meta)
	}
	if env.Meta.TotalPages != 3 {
		t.Errorf("TotalPages = %d ; attendu 3", env.Meta.TotalPages)
	}
}

// TestErreurApplicative vérifie qu'une *apperreur.Erreur produit le bon code HTTP et
// l'enveloppe { "succes": false, "erreur": { code, message, details } }.
func TestErreurApplicative(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/livres/xxx", nil)
	appErr := apperreur.Validation("Un ou plusieurs champs sont invalides.", map[string]string{"isbn": "ISBN-13 invalide"})

	Erreur(rec, r, loggerMuet(), appErr)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("statut = %d ; attendu 422", rec.Code)
	}

	var env enveloppeTest
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corps JSON illisible : %v", err)
	}
	if env.Succes {
		t.Error("succes = true ; attendu false")
	}
	if env.Erreur == nil {
		t.Fatal("le champ erreur est absent")
	}
	if env.Erreur.Code != string(apperreur.CodeValidation) {
		t.Errorf("code = %q ; attendu %q", env.Erreur.Code, apperreur.CodeValidation)
	}
	if env.Erreur.Details["isbn"] != "ISBN-13 invalide" {
		t.Errorf("détails absents ou incorrects : %v", env.Erreur.Details)
	}
}

// TestErreurInterneNeFuitPas vérifie qu'une erreur technique brute est transformée
// en 500 générique, SANS divulguer le détail technique au client.
func TestErreurInterneNeFuitPas(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	brute := errors.New("SELECT * FROM secrets -- fuite technique")

	Erreur(rec, r, loggerMuet(), brute)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("statut = %d ; attendu 500", rec.Code)
	}

	var env enveloppeTest
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("corps JSON illisible : %v", err)
	}
	if env.Erreur == nil {
		t.Fatal("le champ erreur est absent")
	}
	if env.Erreur.Code != string(apperreur.CodeInterne) {
		t.Errorf("code = %q ; attendu %q", env.Erreur.Code, apperreur.CodeInterne)
	}
	if strings.Contains(env.Erreur.Message, "SELECT") {
		t.Errorf("le message a divulgué une information technique : %q", env.Erreur.Message)
	}
}

// TestSansContenu vérifie une réponse 204 sans corps (typique après un DELETE).
func TestSansContenu(t *testing.T) {
	rec := httptest.NewRecorder()
	SansContenu(rec)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("statut = %d ; attendu 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("le corps aurait dû être vide, obtenu %q", rec.Body.String())
	}
}
