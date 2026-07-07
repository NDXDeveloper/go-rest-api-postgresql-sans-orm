package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/reponse"
)

// SanteHandler expose les sondes d'OBSERVABILITÉ : « liveness » et « readiness ».
//
// # Deux sondes, deux questions différentes
//
//   - /health (liveness) : « le processus est-il vivant ? » Répond toujours 200
//     tant que le serveur tourne. Un orchestrateur (Kubernetes, Docker) qui reçoit
//     un échec ici REDÉMARRE le conteneur.
//
//   - /ready (readiness) : « le service peut-il traiter des requêtes ? » Vérifie
//     les dépendances (ici, la base de données). En cas d'échec (503), l'orchestrateur
//     RETIRE l'instance du service sans la redémarrer, le temps qu'elle se rétablisse.
//
// Distinguer les deux évite des redémarrages inutiles quand seule une dépendance
// est momentanément indisponible.
type SanteHandler struct {
	db        *sql.DB
	logger    *slog.Logger
	version   string
	demarrage time.Time
}

// NouveauSanteHandler assemble le handler avec ses dépendances.
func NouveauSanteHandler(db *sql.DB, logger *slog.Logger, version string) *SanteHandler {
	return &SanteHandler{
		db:        db,
		logger:    logger,
		version:   version,
		demarrage: time.Now(),
	}
}

// Vivant — GET /health (liveness). Ne dépend d'AUCUNE ressource externe.
func (h *SanteHandler) Vivant(w http.ResponseWriter, r *http.Request) {
	reponse.Succes(w, http.StatusOK, map[string]any{
		"statut":               "ok",
		"version":              h.version,
		"duree_fonctionnement": time.Since(h.demarrage).String(),
	})
}

// Pret — GET /ready (readiness). Vérifie que la base de données répond.
func (h *SanteHandler) Pret(w http.ResponseWriter, r *http.Request) {
	// Ping borné : on ne veut pas que la sonde elle-même reste bloquée.
	ctx, annuler := context.WithTimeout(r.Context(), 2*time.Second)
	defer annuler()

	if err := database.Verifier(ctx, h.db); err != nil {
		// 503 : le service n'est pas prêt (base injoignable). La cause technique
		// est journalisée, pas exposée.
		reponse.Erreur(w, r, h.logger, apperreur.ServiceIndisponible("Base de données indisponible.").AvecCause(err))
		return
	}

	reponse.Succes(w, http.StatusOK, map[string]any{
		"statut":          "pret",
		"base_de_donnees": "ok",
	})
}
