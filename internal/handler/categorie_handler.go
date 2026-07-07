package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// CategorieHandler expose les points d'entrée CRUD des catégories.
type CategorieHandler struct {
	service *service.CategorieService
	logger  *slog.Logger
}

// NouveauCategorieHandler assemble le handler avec ses dépendances.
func NouveauCategorieHandler(s *service.CategorieService, logger *slog.Logger) *CategorieHandler {
	return &CategorieHandler{service: s, logger: logger}
}

var colonnesTriCategorie = map[string]string{
	"nom":  "nom",
	"date": "cree_le",
}

// Lister — GET /api/v1/categories
func (h *CategorieHandler) Lister(w http.ResponseWriter, r *http.Request) {
	params := analyserParametresListe(r, colonnesTriCategorie, "nom")
	categories, total, err := h.service.Lister(r.Context(), params)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SuccesPagine(w, http.StatusOK, categories, params.Page, params.Taille, total)
}

// Obtenir — GET /api/v1/categories/{id}
func (h *CategorieHandler) Obtenir(w http.ResponseWriter, r *http.Request) {
	categorie, err := h.service.Obtenir(r.Context(), r.PathValue("id"))
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, categorie)
}

// Creer — POST /api/v1/categories
func (h *CategorieHandler) Creer(w http.ResponseWriter, r *http.Request) {
	var entree models.CreerCategorieEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	categorie, err := h.service.Creer(r.Context(), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusCreated, categorie)
}

// Remplacer — PUT /api/v1/categories/{id}
func (h *CategorieHandler) Remplacer(w http.ResponseWriter, r *http.Request) {
	var entree models.MettreAJourCategorieEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	categorie, err := h.service.MettreAJour(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, categorie)
}

// Modifier — PATCH /api/v1/categories/{id}
func (h *CategorieHandler) Modifier(w http.ResponseWriter, r *http.Request) {
	var entree models.ModifierCategorieEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	categorie, err := h.service.Modifier(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, categorie)
}

// Supprimer — DELETE /api/v1/categories/{id}
func (h *CategorieHandler) Supprimer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Supprimer(r.Context(), r.PathValue("id")); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SansContenu(w)
}
