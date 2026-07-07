package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/reponse"
	"golang.org/x/time/rate"
)

// LimiteurDebit limite le nombre de requêtes par client (identifié par son IP),
// pour se protéger des abus : attaques par force brute sur la connexion,
// scraping massif, déni de service applicatif (DoS).
//
// # Algorithme : le « seau à jetons » (token bucket)
//
// Chaque client dispose d'un seau qui se remplit de « jetons » à un débit constant
// (rps = requêtes par seconde). Chaque requête consomme un jeton. Le seau a une
// capacité maximale (« rafale » / burst) qui autorise de courtes pointes. Quand le
// seau est vide, les requêtes sont refusées (429 Too Many Requests). C'est le rôle
// de rate.Limiter (bibliothèque golang.org/x/time/rate).
type LimiteurDebit struct {
	mu      sync.Mutex
	clients map[string]*clientLimite
	debit   rate.Limit
	rafale  int
	logger  *slog.Logger
}

// clientLimite associe à un client son limiteur et la date de sa dernière requête
// (pour purger les clients inactifs et éviter une fuite mémoire).
type clientLimite struct {
	limiteur    *rate.Limiter
	derniereVue time.Time
}

// NouveauLimiteurDebit crée le limiteur et lance une goroutine de nettoyage des
// entrées inactives. Cette goroutine s'arrête proprement quand `ctx` est annulé
// (à l'arrêt du serveur).
func NouveauLimiteurDebit(ctx context.Context, requetesParSeconde float64, rafale int, logger *slog.Logger) *LimiteurDebit {
	l := &LimiteurDebit{
		clients: make(map[string]*clientLimite),
		debit:   rate.Limit(requetesParSeconde),
		rafale:  rafale,
		logger:  logger,
	}
	go l.nettoyerPeriodiquement(ctx)
	return l
}

// limiteurPour renvoie le limiteur associé à une IP, en le créant au besoin.
// L'accès à la map est protégé par un mutex (plusieurs requêtes concurrentes).
func (l *LimiteurDebit) limiteurPour(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	client, existe := l.clients[ip]
	if !existe {
		client = &clientLimite{limiteur: rate.NewLimiter(l.debit, l.rafale)}
		l.clients[ip] = client
	}
	client.derniereVue = time.Now()
	return client.limiteur
}

// nettoyerPeriodiquement supprime les clients inactifs depuis plus de 3 minutes,
// toutes les minutes, pour que la map ne grossisse pas indéfiniment.
func (l *LimiteurDebit) nettoyerPeriodiquement(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return // arrêt propre du serveur
		case <-ticker.C:
			l.mu.Lock()
			for ip, client := range l.clients {
				if time.Since(client.derniereVue) > 3*time.Minute {
					delete(l.clients, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

// Middleware renvoie le middleware qui applique la limite. On utilise Allow() :
// si aucun jeton n'est disponible, on refuse immédiatement avec un 429.
func (l *LimiteurDebit) Middleware() Middleware {
	return func(suivant http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.limiteurPour(ipClient(r)).Allow() {
				// En-tête indicatif : combien de temps attendre avant de réessayer.
				w.Header().Set("Retry-After", "1")
				reponse.Erreur(w, r, l.logger, apperreur.TropDeRequetes("Trop de requêtes. Veuillez réessayer dans quelques instants."))
				return
			}
			suivant.ServeHTTP(w, r)
		})
	}
}
