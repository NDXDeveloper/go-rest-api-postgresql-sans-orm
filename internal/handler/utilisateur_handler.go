package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// UtilisateurHandler expose la gestion des comptes et le profil personnel.
type UtilisateurHandler struct {
	service *service.UtilisateurService
	logger  *slog.Logger
}

// NouveauUtilisateurHandler assemble le handler avec ses dépendances.
func NouveauUtilisateurHandler(s *service.UtilisateurService, logger *slog.Logger) *UtilisateurHandler {
	return &UtilisateurHandler{service: s, logger: logger}
}

var colonnesTriUtilisateur = map[string]string{
	"nom":   "nom",
	"email": "email",
	"role":  "role",
	"date":  "cree_le",
}

// MonProfil — GET /api/v1/moi
// Renvoie le profil de l'utilisateur authentifié (identité lue dans le jeton).
func (h *UtilisateurHandler) MonProfil(w http.ResponseWriter, r *http.Request) {
	utilisateur, err := utilisateurCourant(r)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	profil, err := h.service.Obtenir(r.Context(), utilisateur.UUID)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, profil)
}

// ModifierMonProfil — PATCH /api/v1/moi
// Permet à l'utilisateur authentifié de modifier SON propre nom/prénom. Les
// champs role et actif sont neutralisés : un membre ne peut pas s'auto-promouvoir.
func (h *UtilisateurHandler) ModifierMonProfil(w http.ResponseWriter, r *http.Request) {
	appelant, err := utilisateurCourant(r)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	var entree models.ModifierUtilisateurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	// Sécurité : on ignore toute tentative de modifier son rôle ou son état.
	entree.Role = nil
	entree.Actif = nil

	utilisateur, err := h.service.Modifier(r.Context(), appelant.UUID, entree, false)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, utilisateur)
}

// Lister — GET /api/v1/utilisateurs (réservé aux administrateurs par la route)
func (h *UtilisateurHandler) Lister(w http.ResponseWriter, r *http.Request) {
	params := analyserParametresListe(r, colonnesTriUtilisateur, "cree_le", "role")
	utilisateurs, total, err := h.service.Lister(r.Context(), params)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SuccesPagine(w, http.StatusOK, utilisateurs, params.Page, params.Taille, total)
}

// Obtenir — GET /api/v1/utilisateurs/{id}
func (h *UtilisateurHandler) Obtenir(w http.ResponseWriter, r *http.Request) {
	utilisateur, err := h.service.Obtenir(r.Context(), r.PathValue("id"))
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, utilisateur)
}

// Creer — POST /api/v1/utilisateurs (administrateur : crée avec un rôle explicite)
func (h *UtilisateurHandler) Creer(w http.ResponseWriter, r *http.Request) {
	var entree models.CreerUtilisateurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	utilisateur, err := h.service.Creer(r.Context(), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusCreated, utilisateur)
}

// Remplacer — PUT /api/v1/utilisateurs/{id} (met à jour nom/prénom)
func (h *UtilisateurHandler) Remplacer(w http.ResponseWriter, r *http.Request) {
	var entree models.MettreAJourUtilisateurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	utilisateur, err := h.service.MettreAJour(r.Context(), r.PathValue("id"), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, utilisateur)
}

// Modifier — PATCH /api/v1/utilisateurs/{id}
// Les champs role/actif ne sont appliqués que si l'appelant est administrateur.
func (h *UtilisateurHandler) Modifier(w http.ResponseWriter, r *http.Request) {
	appelant, err := utilisateurCourant(r)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	var entree models.ModifierUtilisateurEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	appelantEstAdmin := appelant.Role == models.RoleAdmin
	utilisateur, err := h.service.Modifier(r.Context(), r.PathValue("id"), entree, appelantEstAdmin)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, utilisateur)
}

// Supprimer — DELETE /api/v1/utilisateurs/{id}[?definitif=true]
// Suppression logique par défaut ; suppression physique (admin) avec ?definitif=true.
func (h *UtilisateurHandler) Supprimer(w http.ResponseWriter, r *http.Request) {
	uuidCible := r.PathValue("id")

	if r.URL.Query().Get("definitif") == "true" {
		appelant, err := utilisateurCourant(r)
		if err != nil {
			reponse.Erreur(w, r, h.logger, err)
			return
		}
		if appelant.Role != models.RoleAdmin {
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
