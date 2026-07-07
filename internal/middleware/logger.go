package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/exemple/api-bibliotheque/internal/contexte"
)

// Journalisation enregistre une ligne de log structurée pour CHAQUE requête, avec
// méthode, chemin, statut, durée et identifiant de requête.
//
// # Niveaux de log (rappel pédagogique)
//
//   - Debug : détail fin, utile en développement uniquement.
//   - Info  : événement normal attendu (une requête réussie).
//   - Warn  : anomalie non bloquante (erreur client 4xx : mauvaise saisie...).
//   - Error : problème sérveur (5xx) nécessitant une investigation.
//
// On choisit ici le niveau selon le code de statut : 2xx/3xx -> Info,
// 4xx -> Warn, 5xx -> Error.
//
// # Journalisation SÉCURISÉE
//
// On journalise le CHEMIN (r.URL.Path) mais PAS la chaîne de requête complète
// (r.URL.RawQuery) ni le corps : ils pourraient contenir des données sensibles
// (jetons, mots de passe). C'est une règle d'or de la journalisation.
func Journalisation(logger *slog.Logger) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			debut := time.Now()

			// On enveloppe le ResponseWriter pour connaître le statut final.
			enregistreur := &enregistreurReponse{ResponseWriter: w}
			suivant.ServeHTTP(enregistreur, r)

			duree := time.Since(debut)
			statut := enregistreur.statutOuDefaut()

			niveau := slog.LevelInfo
			switch {
			case statut >= 500:
				niveau = slog.LevelError
			case statut >= 400:
				niveau = slog.LevelWarn
			}

			logger.LogAttrs(r.Context(), niveau, "requête HTTP",
				slog.String("identifiant_requete", contexte.IdentifiantRequete(r.Context())),
				slog.String("methode", r.Method),
				slog.String("chemin", r.URL.Path),
				slog.Int("statut", statut),
				slog.Int("octets", enregistreur.octets),
				slog.Duration("duree", duree),
				slog.String("ip", ipClient(r)),
			)
		})
	}
}
