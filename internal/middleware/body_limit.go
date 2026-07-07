package middleware

import "net/http"

// LimiteCorps borne la taille du CORPS d'une requête. Au-delà, la lecture du
// corps échoue avec une erreur *http.MaxBytesError (traduite en 413 par le
// décodeur JSON).
//
// Pourquoi c'est important (anti-DoS) : sans limite, un client pourrait envoyer
// un corps de plusieurs gigaoctets et saturer la mémoire du serveur. On fixe donc
// un plafond raisonnable (par défaut 1 Mio, configurable via l'environnement).
//
// http.MaxBytesReader a un avantage subtil : il interrompt aussi la CONNEXION si
// le client continue d'envoyer des données après la limite, évitant qu'un client
// malveillant ne monopolise la connexion.
func LimiteCorps(tailleMaxOctets int64) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, tailleMaxOctets)
			suivant.ServeHTTP(w, r)
		})
	}
}
