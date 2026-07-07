package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// AuteurHandler expose les points d'entrée CRUD des auteurs.
type AuteurHandler struct {
	service *service.AuteurService
	logger  *slog.Logger
}

// NouveauAuteurHandler assemble le handler avec ses dépendances.
func NouveauAuteurHandler(s *service.AuteurService, logger *slog.Logger) *AuteurHandler {
	return &AuteurHandler{service: s, logger: logger}
}

// colonnesTriAuteur mappe les noms de tri exposés au client vers les colonnes SQL.
var colonnesTriAuteur = map[string]string{
	"nom":            "nom",
	"date_naissance": "date_naissance",
	"date":           "cree_le",
}

// Lister — GET /api/v1/auteurs
func (h *AuteurHandler) Lister(w http.ResponseWriter, r *http.Request) {
	params := analyserParametresListe(r, colonnesTriAuteur, "nom", "nationalite")
	auteurs, total, err := h.service.Lister(r.Context(), params)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SuccesPagine(w, http.StatusOK, auteurs, params.Page, params.Taille, total)
}

// Obtenir — GET /api/v1/auteurs/{id}
func (h *AuteurHandler) Obtenir(w http.ResponseWriter, r *http.Request) {
	auteur, err := h.service.Obtenir(r.Context(), r.PathValue("id"))
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, auteur)
}

// Creer — POST /api/v1/auteurs
func (h *AuteurHandler) Creer(w http.ResponseWriter, r *http.Request) {
	var entree models.CreerAuteurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	auteur, err := h.service.Creer(r.Context(), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusCreated, auteur)
}

// Remplacer — PUT /api/v1/auteurs/{id}
func (h *AuteurHandler) Remplacer(w http.ResponseWriter, r *http.Request) {
	var entree models.MettreAJourAuteurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	auteur, err := h.service.MettreAJour(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, auteur)
}

// Modifier — PATCH /api/v1/auteurs/{id}
func (h *AuteurHandler) Modifier(w http.ResponseWriter, r *http.Request) {
	var entree models.ModifierAuteurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	auteur, err := h.service.Modifier(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, auteur)
}

// Supprimer — DELETE /api/v1/auteurs/{id}
func (h *AuteurHandler) Supprimer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Supprimer(r.Context(), r.PathValue("id")); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SansContenu(w)
}
