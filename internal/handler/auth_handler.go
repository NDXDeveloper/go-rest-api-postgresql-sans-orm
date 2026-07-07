package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// AuthHandler expose les points d'entrée d'authentification (non protégés pour
// la plupart : inscription, connexion, rafraîchissement, déconnexion).
type AuthHandler struct {
	service *service.AuthService
	logger  *slog.Logger
}

// NouveauAuthHandler assemble le handler avec ses dépendances.
func NouveauAuthHandler(s *service.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{service: s, logger: logger}
}

// Inscription — POST /api/v1/auth/inscription
// Crée un compte « membre » et renvoie le profil (201 Created).
func (h *AuthHandler) Inscription(w http.ResponseWriter, r *http.Request) {
	var entree models.InscriptionEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	utilisateur, err := h.service.Inscription(r.Context(), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusCreated, utilisateur)
}

// Connexion — POST /api/v1/auth/connexion
// Vérifie les identifiants et renvoie une paire de jetons + le profil (200).
func (h *AuthHandler) Connexion(w http.ResponseWriter, r *http.Request) {
	var entree models.ConnexionEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	resultat, err := h.service.Connexion(r.Context(), entree)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, resultat)
}

// Rafraichir — POST /api/v1/auth/rafraichir
// Échange un refresh token valide contre une nouvelle paire de jetons (200).
func (h *AuthHandler) Rafraichir(w http.ResponseWriter, r *http.Request) {
	var entree models.RafraichissementEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	paire, err := h.service.Rafraichir(r.Context(), entree.JetonRafraichissement)
	if err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.Succes(w, http.StatusOK, paire)
}

// Deconnexion — POST /api/v1/auth/deconnexion
// Révoque le refresh token fourni (204 No Content).
func (h *AuthHandler) Deconnexion(w http.ResponseWriter, r *http.Request) {
	var entree models.RafraichissementEntree
	if err := decoderJSON(r, &entree); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	if err := h.service.Deconnexion(r.Context(), entree.JetonRafraichissement); err != nil {
		reponse.Erreur(w, r, h.logger, err)
		return
	}
	reponse.SansContenu(w)
}
