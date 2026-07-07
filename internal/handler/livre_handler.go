package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// LivreHandler expose les points d'entrée CRUD des livres.
type LivreHandler struct {
	service *service.LivreService
	logger  *slog.Logger
}

// NouveauLivreHandler assemble le handler avec ses dépendances.
func NouveauLivreHandler(s *service.LivreService, logger *slog.Logger) *LivreHandler {
	return &LivreHandler{service: s, logger: logger}
}

var colonnesTriLivre = map[string]string{
	"titre": "titre",
	"annee": "annee_publication",
	"prix":  "prix",
	"date":  "cree_le",
}

// Lister — GET /api/v1/livres
// Prend en charge : ?page= &taille= &tri= &ordre= &recherche= &categorie= &auteur= &disponible=
func (h *LivreHandler) Lister(w http.ResponseWriter, r *http.Request) {
	params := analyserParametresListe(r, colonnesTriLivre, "titre", "categorie", "auteur", "disponible")
	livres, total, err := h.service.Lister(r.Context(), params)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SuccesPagine(w, http.StatusOK, livres, params.Page, params.Taille, total)
}

// Obtenir — GET /api/v1/livres/{id}
func (h *LivreHandler) Obtenir(w http.ResponseWriter, r *http.Request) {
	livre, err := h.service.Obtenir(r.Context(), r.PathValue("id"))
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, livre)
}

// Creer — POST /api/v1/livres
func (h *LivreHandler) Creer(w http.ResponseWriter, r *http.Request) {
	var entree models.CreerLivreEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	livre, err := h.service.Creer(r.Context(), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusCreated, livre)
}

// Remplacer — PUT /api/v1/livres/{id}
func (h *LivreHandler) Remplacer(w http.ResponseWriter, r *http.Request) {
	var entree models.MettreAJourLivreEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	livre, err := h.service.MettreAJour(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, livre)
}

// Modifier — PATCH /api/v1/livres/{id}
func (h *LivreHandler) Modifier(w http.ResponseWriter, r *http.Request) {
	var entree models.ModifierLivreEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	livre, err := h.service.Modifier(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, livre)
}

// Supprimer — DELETE /api/v1/livres/{id}[?definitif=true]
//
// Par défaut, suppression LOGIQUE (le livre est masqué mais conservé). Avec
// « ?definitif=true », suppression PHYSIQUE définitive, réservée aux ADMINS.
func (h *LivreHandler) Supprimer(w http.ResponseWriter, r *http.Request) {
	uuidCible := r.PathValue("id")

	if r.URL.Query().Get("definitif") == "true" {
		utilisateur, err := utilisateurCourant(r)
		if err != nil {
			reponse.Erreur(w, r, h.logger, err)
			return
		}
		if utilisateur.Role != models.RoleAdmin {
			reponse.Erreur(w, r, h.logger, interditAdmin())
			return
		}
		if err := h.service.SupprimerPhysique(r.Context(), uuidCible); err != nil {
			reponse.Erreur(w, r, h.logger, err)
			return
		}
		reponse.SansContenu(w)
		return
	}

	if err := h.service.SupprimerLogique(r.Context(), uuidCible); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SansContenu(w)
}
