package middleware

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/contexte"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/reponse"
)

// Authentification vérifie le jeton d'accès JWT présent dans l'en-tête
// « Authorization: Bearer <jeton> ». En cas de succès, l'identité de
// l'utilisateur est placée dans le contexte pour les handlers suivants.
//
// Ce middleware protège les routes qui EXIGENT une identité. Les routes publiques
// (consultation du catalogue, connexion...) ne le montent pas.
func Authentification(gestionnaireJWT *auth.GestionnaireJWT, logger *slog.Logger) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jetonSigne, ok := extraireJetonBearer(r.Header.Get("Authorization"))
			if !ok {
				reponse.Erreur(w, r, logger, apperreur.NonAuthentifie("Jeton d'accès manquant ou mal formé."))
				return
			}

			revendications, err := gestionnaireJWT.VerifierJetonAcces(jetonSigne)
			if err != nil {
				reponse.Erreur(w, r, logger, err)
				return
			}

			utilisateur := &contexte.UtilisateurAuthentifie{
				UUID:  revendications.Subject,
				Email: revendications.Email,
				Role:  models.Role(revendications.Role),
			}
			ctx := contexte.AvecUtilisateur(r.Context(), utilisateur)
			suivant.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ExigerRole autorise la requête uniquement si l'utilisateur authentifié possède
// l'un des rôles indiqués. À monter APRÈS Authentification (qui pose l'identité).
//
// Exemple : ExigerRole(logger, models.RoleAdmin, models.RoleBibliothecaire)
// laisse passer un admin OU un bibliothécaire, refuse un simple membre (403).
func ExigerRole(logger *slog.Logger, rolesAutorises ...models.Role) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			utilisateur, ok := contexte.Utilisateur(r.Context())
			if !ok {
				// Ne devrait pas arriver si Authentification précède ce middleware.
				reponse.Erreur(w, r, logger, apperreur.NonAuthentifie("Authentification requise."))
				return
			}
			if !slices.Contains(rolesAutorises, utilisateur.Role) {
				reponse.Erreur(w, r, logger, apperreur.Interdit("Vous n'avez pas les droits nécessaires pour cette action."))
				return
			}
			suivant.ServeHTTP(w, r)
		})
	}
}

// extraireJetonBearer isole le jeton d'un en-tête « Bearer <jeton> ».
// La comparaison du préfixe est insensible à la casse (tolérance aux clients).
func extraireJetonBearer(entete string) (string, bool) {
	const prefixe = "Bearer "
	if len(entete) <= len(prefixe) || !strings.EqualFold(entete[:len(prefixe)], prefixe) {
		return "", false
	}
	jeton := strings.TrimSpace(entete[len(prefixe):])
	if jeton == "" {
		return "", false
	}
	return jeton, true
}
