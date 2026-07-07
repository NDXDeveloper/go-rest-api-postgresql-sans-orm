package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/exemple/api-bibliotheque/internal/observabilite"
)

// Metriques instrumente chaque requête pour alimenter les compteurs Prometheus :
// nombre de requêtes (par méthode/route/statut), durée, et nombre en cours.
//
// Détail important — la CARDINALITÉ : on utilise comme label la ROUTE (le patron,
// ex. « GET /api/v1/livres/{id} ») et non le chemin réel (« .../abc-123 »). Sinon,
// chaque identifiant créerait une série de métriques distincte, faisant exploser
// la mémoire de Prometheus. Le patron est disponible via r.Pattern APRÈS le
// routage par le ServeMux.
func Metriques(metriques *observabilite.Metriques) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metriques.RequetesEnCours.Inc()
			defer metriques.RequetesEnCours.Dec()

			debut := time.Now()
			enregistreur := &enregistreurReponse{ResponseWriter: w}

			suivant.ServeHTTP(enregistreur, r)

			// r.Pattern est renseigné par le ServeMux une fois la route trouvée.
			route := r.Pattern
			if route == "" {
				route = "inconnue"
			}
			statut := strconv.Itoa(enregistreur.statutOuDefaut())

			metriques.RequetesTotal.WithLabelValues(r.Method, route, statut).Inc()
			metriques.DureeRequete.WithLabelValues(r.Method, route).Observe(time.Since(debut).Seconds())
		})
	}
}
