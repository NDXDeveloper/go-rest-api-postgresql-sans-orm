// Module Go du projet « API Bibliothèque ».
//
// Le chemin de module (la première ligne « module ... ») sert de préfixe à tous
// les imports internes du projet. Par exemple, le package de configuration
// s'importe via « github.com/exemple/api-bibliotheque/internal/config ».
//
// Choix pédagogique : on cible Go 1.25 (version stable récente). Grâce au
// mécanisme des « toolchains » de Go (GOTOOLCHAIN=auto), la commande `go`
// télécharge automatiquement la bonne version du compilateur si elle n'est pas
// déjà installée. On profite ainsi du routeur `net/http.ServeMux` amélioré
// (Go 1.22+) qui gère les patrons de route par méthode et les jokers, ce qui
// nous évite d'ajouter une dépendance de routeur externe.
module github.com/exemple/api-bibliotheque

go 1.25.0

// Les dépendances directes sont ajoutées automatiquement par `go mod tidy`
// au fur et à mesure que le code les importe. On reste volontairement minimal :
//   - go-sql-driver/mysql : pilote MySQL/MariaDB pour database/sql (PAS un ORM).
//   - golang-jwt/jwt       : génération et vérification des jetons JWT.
//   - google/uuid          : génération d'identifiants publics UUID v4.
//   - golang.org/x/crypto  : hachage bcrypt des mots de passe.
//   - golang.org/x/time    : limiteur de débit (rate limiting) par « token bucket ».
//   - prometheus/client_golang : exposition des métriques au format Prometheus.

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/prometheus/client_golang v1.24.0
	golang.org/x/crypto v0.54.0
	golang.org/x/time v0.15.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.0 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
