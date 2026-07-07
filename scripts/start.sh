#!/usr/bin/env bash
# =============================================================================
# start.sh — Démarre la pile complète (API + PostgreSQL)
# -----------------------------------------------------------------------------
# Construit les images si nécessaire et lance les conteneurs en arrière-plan.
# La base est initialisée automatiquement au tout premier démarrage.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
cd "$RACINE_PROJET"

# Avertit si aucun .env n'est présent (les valeurs par défaut du compose seront utilisées).
if [ ! -f .env ]; then
    echo "Attention : aucun fichier .env trouvé. Copiez .env.example en .env et"
    echo "définissez vos secrets. Démarrage avec les valeurs par défaut..."
fi

echo "Démarrage de la pile (construction des images si besoin)..."
# -d : mode détaché (arrière-plan). --build : (re)construit l'image de l'API.
$COMPOSE up -d --build

echo ""
echo "Pile démarrée. Quelques secondes peuvent être nécessaires à l'initialisation."
echo "  - API    : http://localhost:${SERVEUR_PORT_HOTE:-8080}"
echo "  - Santé  : http://localhost:${SERVEUR_PORT_HOTE:-8080}/health"
echo "  - Suivre les journaux : $COMPOSE logs -f api"
