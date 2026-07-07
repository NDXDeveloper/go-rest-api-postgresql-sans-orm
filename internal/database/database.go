// Package database gère la connexion à PostgreSQL via le package standard
// database/sql et le pilote pgx (github.com/jackc/pgx/v5) en mode « stdlib ».
//
// # Aucun ORM ici
//
// On utilise UNIQUEMENT database/sql : c'est la bibliothèque standard de Go pour
// dialoguer avec une base SQL. Le « pilote » pgx traduit les appels database/sql
// en protocole réseau PostgreSQL. Toutes les requêtes SQL sont écrites à la main
// dans les repositories : aucune génération automatique.
//
// # Pourquoi pgx plutôt que lib/pq ?
//
// pgx est le pilote PostgreSQL le plus complet et le plus performant de
// l'écosystème Go, activement maintenu. Son sous-package « stdlib » l'expose
// comme un pilote database/sql classique (on garde donc *sql.DB, *sql.Tx...),
// tout en profitant de ses performances. lib/pq, l'ancien standard, est
// aujourd'hui en maintenance minimale.
//
// # Le pool de connexions
//
// *sql.DB N'EST PAS une connexion unique : c'est un POOL de connexions géré
// automatiquement. On l'ouvre une fois au démarrage et on le partage (par
// injection) dans toute l'application.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"

	"github.com/exemple/api-bibliotheque/internal/config"
	// Import « anonyme » : il enregistre le pilote « pgx » auprès de database/sql
	// (via sa fonction init), sans qu'on l'utilise directement dans le code.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connecter ouvre le pool de connexions vers PostgreSQL, le configure et vérifie
// qu'il répond (ping). Elle renvoie un *sql.DB prêt à l'emploi ou une erreur.
//
// IMPORTANT : sql.Open n'établit PAS réellement de connexion (il prépare juste
// le pool). C'est PingContext qui force une vraie connexion et permet de détecter
// tôt un problème (mauvais mot de passe, base injoignable...).
func Connecter(cfg config.BaseDeDonnees) (*sql.DB, error) {
	dsn := construireDSN(cfg)

	// On ouvre avec le pilote « pgx » (enregistré par l'import stdlib ci-dessus).
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("ouverture du pool PostgreSQL : %w", err)
	}

	configurerPool(db, cfg)

	// Vérification effective de la connexion, bornée par un délai : si la base
	// ne répond pas à temps, on échoue proprement plutôt que d'attendre indéfiniment.
	ctx, annuler := context.WithTimeout(context.Background(), cfg.DelaiConnexion)
	defer annuler()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connexion à PostgreSQL impossible (%s) : %w", cfg.Hote, err)
	}

	return db, nil
}

// construireDSN assemble la chaîne de connexion (« Data Source Name ») à partir
// de la configuration, sous forme d'URL « postgres://... ».
//
// On construit l'URL avec net/url : url.UserPassword échappe correctement le nom
// d'utilisateur et le mot de passe (qui peuvent contenir des caractères spéciaux),
// ce qui est plus sûr que de concaténer une chaîne à la main.
func construireDSN(cfg config.BaseDeDonnees) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Utilisateur, cfg.MotDePasse),
		Host:   fmt.Sprintf("%s:%d", cfg.Hote, cfg.Port),
		Path:   cfg.Nom,
	}

	parametres := u.Query()
	// sslmode=disable : pas de TLS entre l'application et la base. Acceptable
	// quand les deux communiquent sur un réseau privé/isolé (réseau Docker). En
	// production sur un réseau non maîtrisé, on passerait à « require » ou
	// « verify-full » et on fournirait les certificats.
	parametres.Set("sslmode", "disable")
	// Délai d'établissement de la connexion, en secondes.
	parametres.Set("connect_timeout", strconv.Itoa(int(cfg.DelaiConnexion.Seconds())))
	// On raisonne en UTC côté serveur ET côté client : cohérence des horodatages
	// (PostgreSQL stocke les TIMESTAMPTZ en UTC ; on force aussi la session en UTC).
	parametres.Set("timezone", "UTC")
	u.RawQuery = parametres.Encode()

	return u.String()
}

// configurerPool applique les réglages du pool de connexions.
//
// Ces réglages sont cruciaux en production :
//   - MaxOpenConns limite le nombre de connexions simultanées vers la base.
//     Trop haut : on sature PostgreSQL (max_connections, défaut 100). Trop bas :
//     on crée un goulet d'étranglement.
//   - MaxIdleConns garde quelques connexions ouvertes « au repos » pour éviter
//     le coût de réouverture.
//   - ConnMaxLifetime recycle les connexions périodiquement : utile derrière un
//     équilibreur de charge ou face à un pare-feu qui coupe les connexions longues.
func configurerPool(db *sql.DB, cfg config.BaseDeDonnees) {
	db.SetMaxOpenConns(cfg.MaxConnexionsOuvertes)
	db.SetMaxIdleConns(cfg.MaxConnexionsInactives)
	db.SetConnMaxLifetime(cfg.DureeVieMaxConnexion)
	db.SetConnMaxIdleTime(cfg.DureeVieMaxConnexion)
}

// Verifier effectue un ping borné dans le temps. On l'utilise dans la sonde de
// disponibilité (/ready) pour savoir si la base est joignable à l'instant T.
func Verifier(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}
