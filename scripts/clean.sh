#!/usr/bin/env bash
# =============================================================================
# clean.sh — Nettoyage COMPLET (conteneurs, volumes, images, réseau du projet)
# -----------------------------------------------------------------------------
# Supprime TOUT ce que le projet a créé dans Docker : conteneurs, volume de
# données (DONNÉES PERDUES), images construites et réseau. À utiliser pour
# repartir totalement à zéro.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
cd "$RACINE_PROJET"

echo "ATTENTION : suppression complète (conteneurs, volumes, images) du projet."
read -r -p "Confirmer le nettoyage complet ? [o/N] " reponse
case "$reponse" in
    [oO][uU][iI] | [oO]) ;;
    *)
        echo "Annulé."
        exit 0
        ;;
esac

# --volumes : supprime les volumes nommés (données).
# --rmi local : supprime les images construites localement par ce projet.
# --remove-orphans : supprime d'éventuels conteneurs orphelins.
echo "Arrêt et suppression des ressources Docker du projet..."
$COMPOSE down --volumes --rmi local --remove-orphans

echo "Nettoyage terminé. La prochaine commande « start.sh » repartira de zéro."
