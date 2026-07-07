// Commande api — point d'entrée du serveur de l'API Bibliothèque.
//
// Ce fichier est le SEUL endroit où l'on assemble concrètement toutes les pièces
// de l'application (« composition root »). C'est ici que se fait l'INJECTION DE
// DÉPENDANCES MANUELLE : on construit chaque couche et on lui passe explicitement
// ce dont elle a besoin, sans framework ni conteneur de dépendances, sans aucune
// variable globale.
//
// On y gère aussi le DÉMARRAGE et l'ARRÊT GRACIEUX du serveur : à la réception
// d'un signal (Ctrl+C / SIGTERM), on laisse les requêtes en cours se terminer
// avant de fermer proprement les ressources.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/config"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/handler"
	"github.com/exemple/api-bibliotheque/internal/middleware"
	"github.com/exemple/api-bibliotheque/internal/observabilite"
	"github.com/exemple/api-bibliotheque/internal/repository"
	"github.com/exemple/api-bibliotheque/internal/scheduler"
	"github.com/exemple/api-bibliotheque/internal/service"
)

// version identifie la version du binaire. Elle peut être injectée à la
// compilation via : go build -ldflags "-X main.version=1.2.3".
var version = "1.0.0"

func main() {
	// On délègue à executer() pour pouvoir utiliser « defer » proprement et
	// renvoyer une erreur, tout en centralisant la sortie du process ici.
	if err := executer(); err != nil {
		fmt.Fprintf(os.Stderr, "erreur fatale au démarrage : %v\n", err)
		os.Exit(1)
	}
}

func executer() error {
	// --- 1. Configuration -------------------------------------------------
	cfg, err := config.Charger()
	if err != nil {
		return err
	}

	// --- 2. Journalisation ------------------------------------------------
	logger := observabilite.NouveauLogger(cfg.Log)
	logger.Info("démarrage de l'API Bibliothèque",
		slog.String("version", version),
		slog.String("environnement", cfg.Environnement),
	)

	// --- 3. Contexte de cycle de vie --------------------------------------
	// Ce contexte est annulé à la réception de SIGINT (Ctrl+C) ou SIGTERM
	// (arrêt par Docker/l'orchestrateur). Il pilote l'arrêt des composants en
	// tâche de fond (ordonnanceur, limiteur de débit).
	ctx, arreterSignaux := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer arreterSignaux()

	// --- 4. Base de données -----------------------------------------------
	db, err := database.Connecter(cfg.BaseDeDonnees)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("connexion à PostgreSQL établie",
		slog.String("hote", cfg.BaseDeDonnees.Hote),
		slog.String("base", cfg.BaseDeDonnees.Nom),
	)

	// --- 5. Repositories (couche d'accès aux données) ---------------------
	utilisateurRepo := repository.NouveauUtilisateurRepository(db)
	jetonRepo := repository.NouveauJetonRepository(db)
	auteurRepo := repository.NouveauAuteurRepository(db)
	categorieRepo := repository.NouveauCategorieRepository(db)
	livreRepo := repository.NouveauLivreRepository(db)
	empruntRepo := repository.NouveauEmpruntRepository(db)

	// --- 6. Sécurité (JWT) et services (logique métier) -------------------
	gestionnaireJWT := auth.NouveauGestionnaireJWT(cfg.JWT)
	authService := service.NouveauAuthService(utilisateurRepo, jetonRepo, gestionnaireJWT)
	utilisateurService := service.NouveauUtilisateurService(utilisateurRepo)
	auteurService := service.NouveauAuteurService(auteurRepo)
	categorieService := service.NouveauCategorieService(categorieRepo)
	livreService := service.NouveauLivreService(livreRepo, auteurRepo, categorieRepo)
	empruntService := service.NouveauEmpruntService(empruntRepo)

	// --- 7. Observabilité --------------------------------------------------
	metriques := observabilite.NouvellesMetriques()

	// --- 8. Handlers (couche HTTP) ----------------------------------------
	deps := handler.Dependances{
		Auth:            handler.NouveauAuthHandler(authService, logger),
		Utilisateur:     handler.NouveauUtilisateurHandler(utilisateurService, logger),
		Auteur:          handler.NouveauAuteurHandler(auteurService, logger),
		Categorie:       handler.NouveauCategorieHandler(categorieService, logger),
		Livre:           handler.NouveauLivreHandler(livreService, logger),
		Emprunt:         handler.NouveauEmpruntHandler(empruntService, logger),
		Sante:           handler.NouveauSanteHandler(db, logger, version),
		GestionnaireJWT: gestionnaireJWT,
		Metriques:       metriques,
		Logger:          logger,
	}
	routeur := handler.ConstruireRouteur(deps)

	// --- 9. Ordonnanceur de tâches (goroutines) ---------------------------
	ordonnanceur := scheduler.NouvelOrdonnanceur(logger)
	// Tâche d'exemple : journaliser périodiquement l'état du pool de connexions,
	// utile pour surveiller la santé de la base (connexions ouvertes, en attente...).
	ordonnanceur.Enregistrer(scheduler.Tache{
		Nom:        "statistiques_pool_bdd",
		Intervalle: 5 * time.Minute,
		Executer: func(context.Context) error {
			stats := db.Stats()
			logger.Info("état du pool de connexions",
				slog.Int("connexions_ouvertes", stats.OpenConnections),
				slog.Int("en_utilisation", stats.InUse),
				slog.Int("au_repos", stats.Idle),
				slog.Int64("en_attente", stats.WaitCount),
			)
			return nil
		},
	})
	ordonnanceur.Demarrer(ctx)

	// --- 10. Pile de middlewares GLOBAUX ----------------------------------
	// L'ORDRE est important : du plus externe (premier) au plus interne (dernier).
	limiteur := middleware.NouveauLimiteurDebit(ctx, cfg.Securite.LimiteDebitParSeconde, cfg.Securite.LimiteDebitRafale, logger)
	pileGlobale := middleware.Chainer(
		middleware.IdentifiantRequete(),                           // 1. attribue un ID à la requête
		middleware.Journalisation(logger),                         // 2. journalise la requête/réponse
		middleware.Recuperation(logger),                           // 3. capture les paniques
		middleware.EntetesSecurite(),                              // 4. en-têtes de sécurité
		middleware.CORS(cfg.Securite.OriginesAutorisees),          // 5. CORS
		limiteur.Middleware(),                                     // 6. limite de débit
		middleware.LimiteCorps(cfg.Securite.TailleMaxCorpsOctets), // 7. limite la taille du corps
		middleware.Timeout(cfg.Serveur.DelaiTraitement),           // 8. délai de traitement
		middleware.Metriques(metriques),                           // 9. métriques Prometheus
	)
	handlerFinal := pileGlobale(routeur)

	// --- 11. Serveur HTTP --------------------------------------------------
	serveur := &http.Server{
		Addr:    cfg.Serveur.AdresseComplete(),
		Handler: handlerFinal,
		// Délais protégeant contre les clients lents (attaque Slowloris) :
		ReadHeaderTimeout: cfg.Serveur.DelaiLecture,
		ReadTimeout:       cfg.Serveur.DelaiLecture,
		WriteTimeout:      cfg.Serveur.DelaiEcriture,
		IdleTimeout:       cfg.Serveur.DelaiInactif,
		// Le logger d'erreurs du serveur HTTP est branché sur notre slog.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// --- 12. Démarrage du serveur (dans une goroutine) --------------------
	erreursServeur := make(chan error, 1)
	go func() {
		logger.Info("serveur HTTP en écoute", slog.String("adresse", serveur.Addr))
		// ListenAndServe bloque jusqu'à Shutdown/Close. ErrServerClosed est
		// l'arrêt NORMAL, on ne le traite pas comme une erreur.
		if err := serveur.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			erreursServeur <- err
		}
	}()

	// --- 13. Attente d'un signal d'arrêt ou d'une erreur fatale -----------
	select {
	case err := <-erreursServeur:
		return fmt.Errorf("le serveur s'est arrêté sur erreur : %w", err)
	case <-ctx.Done():
		logger.Info("signal d'arrêt reçu, arrêt gracieux en cours...")
	}

	// --- 14. Arrêt gracieux -----------------------------------------------
	// On laisse aux requêtes en cours le temps de se terminer avant de fermer.
	ctxArret, annulerArret := context.WithTimeout(context.Background(), cfg.Serveur.DelaiArretGracieux)
	defer annulerArret()

	if err := serveur.Shutdown(ctxArret); err != nil {
		logger.Error("arrêt gracieux impossible, fermeture forcée", slog.Any("erreur", err))
		_ = serveur.Close()
	}

	// Le contexte principal est déjà annulé (signal) : l'ordonnanceur et le
	// limiteur de débit s'arrêtent. On attend la fin des goroutines de tâches.
	ordonnanceur.Attendre()

	logger.Info("arrêt terminé proprement")
	return nil
}
