package middleware

import (
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/contexte"
	"github.com/exemple/api-bibliotheque/internal/validation"
	"github.com/google/uuid"
)

// IdentifiantRequete attribue à chaque requête un identifiant unique, qu'il place
// dans le contexte ET renvoie dans l'en-tête « X-Request-ID ».
//
// À quoi ça sert ?
//   - CORRÉLATION : tous les logs d'une même requête portent le même identifiant,
//     ce qui permet de reconstituer son parcours (indispensable en production).
//   - SUPPORT : en cas d'erreur, on peut renvoyer cet identifiant au client et
//     retrouver instantanément les logs correspondants.
//
// Si le client fournit déjà un « X-Request-ID » AU FORMAT UUID valide, on le
// réutilise (traçage de bout en bout). Sinon, on en génère un. On refuse un
// identifiant client au format libre pour éviter l'injection de contenu dans les logs.
func IdentifiantRequete() Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if !validation.EstUUIDValide(id) {
				id = uuid.NewString()
			}

			w.Header().Set("X-Request-ID", id)
			ctx := contexte.AvecIdentifiantRequete(r.Context(), id)
			suivant.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
