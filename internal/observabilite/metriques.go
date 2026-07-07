// Package observabilite expose les métriques Prometheus de l'application.
//
// # À quoi servent les métriques ?
//
// Là où les LOGS racontent des événements ponctuels, les MÉTRIQUES agrègent des
// tendances : nombre de requêtes par seconde, latence, taux d'erreurs... Un
// serveur Prometheus les collecte périodiquement sur l'endpoint /metrics, et un
// tableau de bord (Grafana) les visualise.
//
// # Pas de variable globale
//
// La bibliothèque Prometheus propose un registre GLOBAL par défaut, mais on crée
// ici notre PROPRE registre. C'est plus propre (pas d'état global partagé), plus
// testable, et cela évite les doubles enregistrements.
package observabilite

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metriques regroupe les indicateurs applicatifs et leur registre.
type Metriques struct {
	registre *prometheus.Registry

	// RequetesTotal : compteur du nombre de requêtes, ventilé par méthode, route
	// et code de statut. Un COMPTEUR ne fait qu'augmenter.
	RequetesTotal *prometheus.CounterVec

	// DureeRequete : histogramme des durées de traitement, par méthode et route.
	// Un HISTOGRAMME répartit les observations dans des tranches (buckets) et
	// permet de calculer des quantiles (p50, p95, p99).
	DureeRequete *prometheus.HistogramVec

	// RequetesEnCours : jauge du nombre de requêtes en cours. Une JAUGE peut
	// monter et descendre.
	RequetesEnCours prometheus.Gauge
}

// NouvellesMetriques crée les métriques et les enregistre dans un registre dédié,
// avec en prime les métriques standard du runtime Go et du processus.
func NouvellesMetriques() *Metriques {
	registre := prometheus.NewRegistry()

	m := &Metriques{
		registre: registre,
		RequetesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bibliotheque_http_requetes_total",
				Help: "Nombre total de requêtes HTTP traitées, par méthode, route et statut.",
			},
			[]string{"methode", "route", "statut"},
		),
		DureeRequete: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bibliotheque_http_duree_requete_secondes",
				Help:    "Durée de traitement des requêtes HTTP, en secondes.",
				Buckets: prometheus.DefBuckets, // 0,005s à 10s
			},
			[]string{"methode", "route"},
		),
		RequetesEnCours: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "bibliotheque_http_requetes_en_cours",
				Help: "Nombre de requêtes HTTP actuellement en cours de traitement.",
			},
		),
	}

	// Enregistrement : nos métriques + celles du runtime Go (GC, goroutines...) et
	// du processus (CPU, mémoire, descripteurs de fichiers).
	registre.MustRegister(
		m.RequetesTotal,
		m.DureeRequete,
		m.RequetesEnCours,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Handler renvoie le handler HTTP à monter sur /metrics pour exposer les données
// au format texte attendu par Prometheus.
func (m *Metriques) Handler() http.Handler {
	return promhttp.HandlerFor(m.registre, promhttp.HandlerOpts{})
}
