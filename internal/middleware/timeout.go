package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout borne la durée de TRAITEMENT d'une requête en attachant un délai au
// context.Context. Quand le délai expire, le contexte est annulé : toutes les
// opérations qui l'observent (requêtes SQL via ...Context, appels réseau...)
// s'interrompent d'elles-mêmes.
//
// C'est complémentaire des délais du serveur HTTP (ReadTimeout/WriteTimeout,
// configurés sur http.Server) qui protègent, eux, contre les clients lents
// (attaque « Slowloris »). Ici, on protège contre les TRAITEMENTS trop longs
// (une requête SQL qui s'éternise) en libérant les ressources.
//
// Bonne pratique : toujours propager r.Context() jusqu'aux appels base de
// données. Sans cela, l'annulation resterait sans effet.
func Timeout(duree time.Duration) Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, annuler := context.WithTimeout(r.Context(), duree)
			defer annuler() // libère les ressources associées au contexte

			suivant.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
