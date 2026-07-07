package middleware

import "net/http"

// EntetesSecurite ajoute à chaque réponse des en-têtes HTTP de sécurité qui
// durcissent le comportement des navigateurs. Chacun contre une menace précise :
//
//   - X-Content-Type-Options: nosniff
//     Empêche le navigateur de « deviner » le type d'un contenu (MIME sniffing),
//     ce qui bloque certaines attaques XSS via des fichiers mal typés.
//
//   - X-Frame-Options: DENY
//     Interdit d'afficher nos réponses dans une <iframe> : protège du CLICKJACKING
//     (un site malveillant superpose notre page pour piéger les clics).
//
//   - Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
//     Pour une API JSON, on n'autorise aucune ressource active : renforce la
//     protection contre le XSS et l'inclusion dans une iframe.
//
//   - Referrer-Policy: no-referrer
//     N'envoie pas l'URL d'origine lors des navigations sortantes (moins de fuite).
//
//   - Strict-Transport-Security (HSTS)
//     Demande au navigateur de toujours utiliser HTTPS. Sans effet en HTTP simple
//     (utile derrière un terminaison TLS/reverse proxy en production).
//
//   - Cache-Control: no-store
//     Évite la mise en cache de réponses potentiellement sensibles.
func EntetesSecurite() Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entetes := w.Header()
			entetes.Set("X-Content-Type-Options", "nosniff")
			entetes.Set("X-Frame-Options", "DENY")
			entetes.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			entetes.Set("Referrer-Policy", "no-referrer")
			entetes.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			entetes.Set("Cache-Control", "no-store")

			suivant.ServeHTTP(w, r)
		})
	}
}
