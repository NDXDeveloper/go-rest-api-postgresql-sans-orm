#!/usr/bin/env bash
# =============================================================================
# reset.sh — Réinitialise complètement la base (DONNÉES PERDUES)
# -----------------------------------------------------------------------------
# Supprime le volume de données puis redémarre : la base est recréée de zéro à
# partir des scripts d'initialisation (schéma + seed). Pratique en formation pour
# repartir d'un état propre.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
cd "$RACINE_PROJET"

echo "ATTENTION : cette opération SUPPRIME toutes les données de la base."
# On demande confirmation pour éviter une perte accidentelle.
read -r -p "Confirmer la réinitialisation ? [o/N] " reponse
case "$reponse" in
    [oO][uU][iI] | [oO]) ;;
    *)
        echo "Annulé."
        exit 0
        ;;
esac

echo "Suppression des conteneurs et du volume de données..."
# -v : supprime aussi les volumes nommés (donc les données).
$COMPOSE down -v

echo "Recréation de la pile (base réinitialisée + seed)..."
$COMPOSE up -d --build

echo "Base réinitialisée. Données de démonstration rechargées."
