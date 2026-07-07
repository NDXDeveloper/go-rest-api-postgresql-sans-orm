package middleware

import (
	"net/http"
	"slices"
	"strings"
)

// CORS (« Cross-Origin Resource Sharing ») contrôle quels sites web (origines)
// ont le droit d'appeler notre API depuis un navigateur.
//
// # Le problème que CORS résout
//
// Par défaut, un navigateur INTERDIT à une page servie par https://site-a.fr
// d'appeler une API sur https://api-b.fr (politique « same-origin »). CORS est le
// mécanisme par lequel l'API déclare explicitement « j'autorise telle origine ».
// Ce n'est PAS une protection du serveur (un client hors navigateur ignore CORS),
// mais une protection des UTILISATEURS de navigateurs contre certaines attaques.
//
// # Requête de pré-vol (preflight)
//
// Avant une requête « non simple » (PUT, DELETE, en-têtes personnalisés...), le
// navigateur envoie d'abord une requête OPTIONS pour demander la permission. On y
// répond avec les en-têtes autorisés, puis un 204 sans corps.
//
// SÉCURITÉ : on n'autorise « * » (toutes origines) qu'en développement. En
// production, la configuration impose une liste explicite (voir config.valider()).
func CORS(originesAutorisees []string) Middleware {
	autoriseTout := slices.Contains(originesAutorisees, "*")

	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origine := r.Header.Get("Origin")

			if origine != "" && (autoriseTout || slices.Contains(originesAutorisees, origine)) {
				if autoriseTout {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					// On renvoie l'origine exacte et on signale que la réponse
					// dépend de l'en-tête Origin (pour les caches).
					w.Header().Set("Access-Control-Allow-Origin", origine)
					w.Header().Add("Vary", "Origin")
					// Les cookies/identifiants ne sont autorisés qu'avec une
					// origine précise (jamais avec « * », règle du navigateur).
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "3600") // cache du pré-vol : 1 h
			}

			// Réponse immédiate à la requête de pré-vol.
			if strings.EqualFold(r.Method, http.MethodOptions) {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			suivant.ServeHTTP(w, r)
		})
	}
}
