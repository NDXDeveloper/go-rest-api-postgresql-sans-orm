package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/contexte"
	"github.com/exemple/api-bibliotheque/internal/reponse"
)

// Recuperation intercepte les PANIQUES (panics) qui remonteraient d'un handler,
// pour éviter que le serveur ne plante ou ne laisse la connexion pendante.
//
// Sans ce middleware, une panique dans un handler (accès à un pointeur nil,
// index hors limites...) ferait CRASHER la goroutine de la requête et, selon les
// cas, couperait la connexion sans réponse propre. Ici, on récupère la panique,
// on la journalise AVEC la pile d'appels (pour le diagnostic), et on renvoie une
// réponse 500 générique — SANS jamais divulguer la trace technique au client.
func Recuperation(logger *slog.Logger) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if incident := recover(); incident != nil {
					// La pile d'appels est journalisée côté serveur uniquement.
					logger.ErrorContext(r.Context(), "panique récupérée dans un handler",
						slog.Any("panique", incident),
						slog.String("identifiant_requete", contexte.IdentifiantRequete(r.Context())),
						slog.String("methode", r.Method),
						slog.String("chemin", r.URL.Path),
						slog.String("pile_appels", string(debug.Stack())),
					)
					// Réponse neutre pour le client (aucune fuite technique).
					reponse.Erreur(w, r, logger, apperreur.Interne(""))
				}
			}()
			suivant.ServeHTTP(w, r)
		})
	}
}
