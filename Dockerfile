# syntax=docker/dockerfile:1
# =============================================================================
# Dockerfile — Construction multi-étapes (« multi-stage build ») de l'API
# -----------------------------------------------------------------------------
# PRINCIPE DU MULTI-STAGE
#
# On sépare la COMPILATION (qui a besoin de tout le SDK Go, ~800 Mo) de
# l'EXÉCUTION (qui n'a besoin que du binaire final). L'image livrée ne contient
# donc PAS le compilateur ni le code source : elle est petite (~20 Mo), rapide à
# déployer et expose une surface d'attaque minimale.
# =============================================================================

# -----------------------------------------------------------------------------
# ÉTAPE 1 — Compilation
# -----------------------------------------------------------------------------
FROM golang:1.25-alpine AS constructeur

WORKDIR /app

# On copie d'abord UNIQUEMENT go.mod / go.sum, puis on télécharge les dépendances.
# Grâce au cache de couches Docker, cette étape n'est re-exécutée que si ces deux
# fichiers changent — pas à chaque modification du code. Compilations bien plus rapides.
COPY go.mod go.sum ./
RUN go mod download

# On copie ensuite le reste du code source.
COPY . .

# Compilation :
#   - CGO_ENABLED=0 : binaire STATIQUE (aucune dépendance à la libc), portable.
#   - -ldflags "-s -w" : retire la table des symboles et les infos de debug
#     (binaire plus petit).
#   - -X main.version : injecte le numéro de version dans le binaire.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=1.0.0" \
    -o /app/api ./cmd/api

# -----------------------------------------------------------------------------
# ÉTAPE 2 — Image d'exécution minimale
# -----------------------------------------------------------------------------
FROM alpine:3.20

# ca-certificates : pour d'éventuels appels HTTPS sortants.
# tzdata : bases de fuseaux horaires (au cas où).
# On crée un utilisateur NON PRIVILÉGIÉ : le conteneur ne tournera PAS en root,
# ce qui limite l'impact d'une éventuelle compromission (bonne pratique de sécurité).
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 appuser

WORKDIR /app

# On copie uniquement le binaire depuis l'étape de compilation.
COPY --from=constructeur /app/api /app/api

# À partir d'ici, tout s'exécute sous l'utilisateur non-root.
USER appuser

# Port d'écoute par défaut de l'application (informatif).
EXPOSE 8080

# HEALTHCHECK : Docker interroge périodiquement /health pour connaître l'état du
# conteneur. wget est fourni par busybox (présent dans alpine). Le « start-period »
# laisse le temps à l'application de démarrer avant de compter les échecs.
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8080/health >/dev/null 2>&1 || exit 1

# Point d'entrée : le binaire de l'API.
ENTRYPOINT ["/app/api"]
