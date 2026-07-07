package observabilite

import (
	"log/slog"
	"os"

	"github.com/exemple/api-bibliotheque/internal/config"
)

// NouveauLogger construit un journaliseur STRUCTURÉ à partir de la configuration.
//
// # Pourquoi slog (bibliothèque standard) ?
//
// slog produit des logs STRUCTURÉS (clé=valeur) plutôt que du texte libre. Chaque
// champ (methode, statut, duree...) est exploitable par une machine : on peut
// filtrer, agréger et alerter dessus. C'est indispensable en production.
//
// # Deux formats
//
//   - « json »  : une ligne JSON par log, idéal pour la production (collecte par
//     un agent type Loki/ELK).
//   - « texte » : plus lisible à l'œil, pratique en développement.
//
// # Niveaux
//
// Le niveau configuré filtre les logs : en « info », les logs « debug » sont
// ignorés. On règle donc « debug » en développement, « info » en production.
func NouveauLogger(cfg config.Log) *slog.Logger {
	options := &slog.HandlerOptions{
		Level: niveauDepuisChaine(cfg.Niveau),
	}

	var gestionnaire slog.Handler
	if cfg.Format == "json" {
		gestionnaire = slog.NewJSONHandler(os.Stdout, options)
	} else {
		gestionnaire = slog.NewTextHandler(os.Stdout, options)
	}

	return slog.New(gestionnaire)
}

// niveauDepuisChaine convertit le niveau textuel de configuration en slog.Level.
func niveauDepuisChaine(niveau string) slog.Level {
	switch niveau {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
