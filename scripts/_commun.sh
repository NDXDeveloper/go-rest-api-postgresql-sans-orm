#!/usr/bin/env bash
# =============================================================================
# _commun.sh — Fonctions et réglages partagés par tous les scripts
# -----------------------------------------------------------------------------
# Ce fichier n'est pas exécuté directement : il est « sourcé » par les autres
# scripts (start.sh, stop.sh...) via « source "$(dirname "$0")/_commun.sh" ».
# =============================================================================

# « set -euo pipefail » est la ceinture de sécurité des scripts Bash :
#   -e : arrête le script à la première commande qui échoue ;
#   -u : erreur si on utilise une variable non définie ;
#   -o pipefail : un échec dans un pipe « a | b » fait échouer toute la chaîne.
set -euo pipefail

# Racine du projet (dossier parent de scripts/), quel que soit l'endroit d'appel.
RACINE_PROJET="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Détecte la bonne commande Compose : « docker compose » (v2, recommandé) ou
# « docker-compose » (v1, ancien). On expose la variable COMPOSE.
detecter_compose() {
    if docker compose version >/dev/null 2>&1; then
        COMPOSE="docker compose"
    elif command -v docker-compose >/dev/null 2>&1; then
        COMPOSE="docker-compose"
    else
        echo "Erreur : ni « docker compose » (v2) ni « docker-compose » (v1) n'est disponible." >&2
        echo "Installez Docker Compose : https://docs.docker.com/compose/install/" >&2
        exit 1
    fi
}

# Charge les variables du fichier .env s'il existe (pour BDD_NOM, etc.).
charger_env() {
    if [ -f "$RACINE_PROJET/.env" ]; then
        # set -a exporte automatiquement toute variable définie ensuite.
        set -a
        # shellcheck disable=SC1091
        . "$RACINE_PROJET/.env"
        set +a
    fi
}

# Nom de la base (valeur par défaut si non défini dans .env).
BDD_NOM="${BDD_NOM:-bibliotheque}"
