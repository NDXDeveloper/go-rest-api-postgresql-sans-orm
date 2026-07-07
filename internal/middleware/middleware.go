// Package middleware fournit les intercepteurs HTTP (« middlewares ») qui
// enveloppent les handlers pour ajouter des comportements transversaux :
// journalisation, sécurité, limitation de débit, authentification...
//
// # Le motif « middleware » en Go
//
// Un middleware est une fonction qui prend un http.Handler et en renvoie un
// autre, l'enveloppant :
//
//	func MonMiddleware(suivant http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        // ... avant ...
//	        suivant.ServeHTTP(w, r)
//	        // ... après ...
//	    })
//	}
//
// En chaînant plusieurs middlewares, on construit un « oignon » de couches
// autour du handler final. L'ORDRE compte : le premier middleware de la chaîne
// est le plus EXTERNE (il voit la requête en premier et la réponse en dernier).
package middleware

import (
	"net"
	"net/http"
)

// ipClient extrait l'adresse IP du client à partir de RemoteAddr (« hôte:port »).
//
// SÉCURITÉ : on N'utilise PAS l'en-tête X-Forwarded-For par défaut car il est
// FALSIFIABLE par le client. On ne s'y fierait que derrière un proxy de confiance
// qui le réécrit. Ici, RemoteAddr (l'IP de la connexion TCP) est la source sûre.
func ipClient(r *http.Request) string {
	hote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return hote
}

// Middleware est le type d'un intercepteur : il enveloppe un handler.
type Middleware func(http.Handler) http.Handler

// Chainer compose plusieurs middlewares en un seul. Les middlewares sont
// appliqués de gauche à droite en ordre d'ENVELOPPEMENT : Chainer(A, B, C)
// exécute A en premier (couche la plus externe), puis B, puis C, puis le handler.
//
//	handler = Chainer(RequestID, Logger, Recovery)(monHandler)
//	// À l'exécution : RequestID -> Logger -> Recovery -> monHandler
func Chainer(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		// On enveloppe en partant de la fin pour que le premier de la liste soit
		// bien la couche la plus externe.
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// enregistreurReponse enveloppe http.ResponseWriter pour MÉMORISER le code de
// statut et le nombre d'octets écrits. Les middlewares Logger et Métriques en ont
// besoin : l'interface http.ResponseWriter standard ne permet pas de relire le
// statut une fois écrit.
type enregistreurReponse struct {
	http.ResponseWriter
	statut int
	octets int
	ecrit  bool
}

// WriteHeader mémorise le statut la première fois qu'il est défini.
func (e *enregistreurReponse) WriteHeader(code int) {
	if !e.ecrit {
		e.statut = code
		e.ecrit = true
	}
	e.ResponseWriter.WriteHeader(code)
}

// Write mémorise implicitement un statut 200 si WriteHeader n'a pas été appelé
// (comportement standard de net/http) et cumule le nombre d'octets écrits.
func (e *enregistreurReponse) Write(donnees []byte) (int, error) {
	if !e.ecrit {
		e.WriteHeader(http.StatusOK)
	}
	n, err := e.ResponseWriter.Write(donnees)
	e.octets += n
	return n, err
}

// statutOuDefaut renvoie le statut mémorisé, ou 200 si aucune écriture n'a eu
// lieu (cas d'un handler qui ne fait rien, rare).
func (e *enregistreurReponse) statutOuDefaut() int {
	if e.statut == 0 {
		return http.StatusOK
	}
	return e.statut
}
