#!/usr/bin/env bash
# =============================================================================
# rebuild.sh — Reconstruit l'image de l'API SANS cache et redémarre
# -----------------------------------------------------------------------------
# Utile après une modification du code Go ou des dépendances : force une
# reconstruction complète (sans réutiliser le cache de couches Docker), puis
# recrée les conteneurs. Les DONNÉES de la base sont CONSERVÉES.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
cd "$RACINE_PROJET"

echo "Reconstruction de l'image de l'API (sans cache)..."
$COMPOSE build --no-cache api

echo "Recréation des conteneurs..."
# --force-recreate : recrée les conteneurs même si leur configuration n'a pas changé.
$COMPOSE up -d --force-recreate

echo "Reconstruction terminée. Journaux : $COMPOSE logs -f api"
