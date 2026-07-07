#!/usr/bin/env bash
# =============================================================================
# stop.sh — Arrête la pile SANS supprimer les données
# -----------------------------------------------------------------------------
# Les conteneurs et le réseau sont supprimés, mais le VOLUME de données PostgreSQL
# est CONSERVÉ : au prochain « start.sh », vous retrouverez vos données.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
cd "$RACINE_PROJET"

echo "Arrêt de la pile (les données sont conservées)..."
$COMPOSE down

echo "Pile arrêtée. Vos données sont intactes (volume conservé)."
