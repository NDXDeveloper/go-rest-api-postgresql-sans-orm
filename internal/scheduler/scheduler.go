// Package scheduler fournit un ordonnanceur de tâches périodiques CÔTÉ
// APPLICATION (à distinguer de pg_cron, qui tourne côté base).
//
// # Quand utiliser l'un ou l'autre ?
//
//   - pg_cron : idéal pour la maintenance des DONNÉES (purge, archivage,
//     agrégats) — au plus près des tables, sans dépendre de l'application.
//   - Ordonnanceur Go : idéal pour des tâches APPLICATIVES (rafraîchir un cache,
//     journaliser des métriques internes, appeler un service externe...).
//
// Ce package illustre les fondamentaux de la concurrence en Go : une goroutine
// par tâche, un time.Ticker pour la périodicité, un context.Context pour l'arrêt
// propre, et un sync.WaitGroup pour attendre la fin de toutes les goroutines.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Tache décrit un travail à exécuter périodiquement.
type Tache struct {
	Nom        string
	Intervalle time.Duration
	// Executer reçoit un contexte : la tâche DOIT l'observer pour s'interrompre
	// rapidement lors de l'arrêt du serveur (ex. le passer aux requêtes SQL).
	Executer func(ctx context.Context) error
}

// Ordonnanceur exécute des tâches enregistrées, chacune dans sa propre goroutine.
type Ordonnanceur struct {
	logger *slog.Logger
	taches []Tache
	wg     sync.WaitGroup
}

// NouvelOrdonnanceur crée un ordonnanceur vide.
func NouvelOrdonnanceur(logger *slog.Logger) *Ordonnanceur {
	return &Ordonnanceur{logger: logger}
}

// Enregistrer ajoute une tâche. À appeler AVANT Demarrer.
func (o *Ordonnanceur) Enregistrer(tache Tache) {
	o.taches = append(o.taches, tache)
}

// Demarrer lance une goroutine par tâche. Les tâches s'exécutent jusqu'à ce que
// `ctx` soit annulé (arrêt du serveur). Retour immédiat (non bloquant).
func (o *Ordonnanceur) Demarrer(ctx context.Context) {
	for _, tache := range o.taches {
		o.wg.Add(1)
		go o.boucleTache(ctx, tache)
	}
}

// Attendre bloque jusqu'à ce que toutes les goroutines de tâches soient terminées.
// À appeler après l'annulation du contexte, pour un arrêt propre.
func (o *Ordonnanceur) Attendre() {
	o.wg.Wait()
}

// boucleTache exécute une tâche à intervalle régulier jusqu'à annulation.
func (o *Ordonnanceur) boucleTache(ctx context.Context, tache Tache) {
	defer o.wg.Done()

	ticker := time.NewTicker(tache.Intervalle)
	defer ticker.Stop()

	o.logger.Info("tâche planifiée démarrée",
		slog.String("tache", tache.Nom),
		slog.Duration("intervalle", tache.Intervalle),
	)

	for {
		select {
		case <-ctx.Done():
			// Arrêt demandé : on sort proprement de la boucle.
			o.logger.Info("tâche planifiée arrêtée", slog.String("tache", tache.Nom))
			return

		case <-ticker.C:
			debut := time.Now()
			if err := tache.Executer(ctx); err != nil {
				o.logger.Error("échec d'une tâche planifiée",
					slog.String("tache", tache.Nom),
					slog.Any("erreur", err),
				)
				continue
			}
			o.logger.Debug("tâche planifiée exécutée",
				slog.String("tache", tache.Nom),
				slog.Duration("duree", time.Since(debut)),
			)
		}
	}
}
