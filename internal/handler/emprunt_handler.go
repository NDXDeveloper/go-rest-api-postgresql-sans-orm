package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// EmpruntHandler expose la gestion des emprunts et retours.
type EmpruntHandler struct {
	service *service.EmpruntService
	logger  *slog.Logger
}

// NouveauEmpruntHandler assemble le handler avec ses dépendances.
func NouveauEmpruntHandler(s *service.EmpruntService, logger *slog.Logger) *EmpruntHandler {
	return &EmpruntHandler{service: s, logger: logger}
}

var colonnesTriEmprunt = map[string]string{
	"date_emprunt": "e.date_emprunt",
	"date_retour":  "e.date_retour_prevue",
	"statut":       "e.statut",
}

// Emprunter — POST /api/v1/emprunts
//
// Par défaut, un membre emprunte pour LUI-MÊME (identité issue du jeton). Un
// bibliothécaire ou un admin peut renseigner « utilisateur_id » pour enregistrer
// un emprunt au nom d'un membre. Un membre qui tenterait cela reçoit un 403.
func (h *EmpruntHandler) Emprunter(w http.ResponseWriter, r *http.Request) {
	appelant, err := utilisateurCourant(r)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	var entree models.EmprunterEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}

	utilisateurUUID := appelant.UUID
	if entree.UtilisateurID != "" && entree.UtilisateurID != appelant.UUID {
		if appelant.Role != models.RoleAdmin && appelant.Role != models.RoleBibliothecaire {
			reponse.Erreur(w, r, h.logger, apperreur.Interdit("Vous ne pouvez emprunter que pour vous-même."))
			return
		}
		utilisateurUUID = entree.UtilisateurID
	}

	emprunt, err := h.service.Emprunter(r.Context(), utilisateurUUID, entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusCreated, emprunt)
}

// Rendre — POST /api/v1/emprunts/{id}/retour
// Enregistre le retour et renvoie l'emprunt clôturé (avec sa pénalité éventuelle).
func (h *EmpruntHandler) Rendre(w http.ResponseWriter, r *http.Request) {
	emprunt, err := h.service.Rendre(r.Context(), r.PathValue("id"))
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, emprunt)
}

// Obtenir — GET /api/v1/emprunts/{id}
func (h *EmpruntHandler) Obtenir(w http.ResponseWriter, r *http.Request) {
	emprunt, err := h.service.Obtenir(r.Context(), r.PathValue("id"))
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, emprunt)
}

// Lister — GET /api/v1/emprunts (réservé bibliothécaire/admin par la route)
// Liste TOUS les emprunts, avec filtre optionnel ?statut= et pagination.
func (h *EmpruntHandler) Lister(w http.ResponseWriter, r *http.Request) {
	params := analyserParametresListe(r, colonnesTriEmprunt, "e.date_emprunt", "statut")
	emprunts, total, err := h.service.Lister(r.Context(), "", params)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SuccesPagine(w, http.StatusOK, emprunts, params.Page, params.Taille, total)
}

// MesEmprunts — GET /api/v1/moi/emprunts
// Liste les emprunts de l'utilisateur authentifié uniquement.
func (h *EmpruntHandler) MesEmprunts(w http.ResponseWriter, r *http.Request) {
	appelant, err := utilisateurCourant(r)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	params := analyserParametresListe(r, colonnesTriEmprunt, "e.date_emprunt", "statut")
	emprunts, total, err := h.service.Lister(r.Context(), appelant.UUID, params)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SuccesPagine(w, http.StatusOK, emprunts, params.Page, params.Taille, total)
}

// MesStatistiques — GET /api/v1/moi/statistiques
// Renvoie les indicateurs d'emprunt de l'utilisateur authentifié (via procédure OUT).
func (h *EmpruntHandler) MesStatistiques(w http.ResponseWriter, r *http.Request) {
	appelant, err := utilisateurCourant(r)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	stats, err := h.service.Statistiques(r.Context(), appelant.UUID)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, stats)
}
