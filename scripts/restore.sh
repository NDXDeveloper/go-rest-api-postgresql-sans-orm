#!/usr/bin/env bash
# =============================================================================
# restore.sh — Restaure la base depuis un fichier de sauvegarde
# -----------------------------------------------------------------------------
# Usage : ./scripts/restore.sh backups/bibliotheque_AAAAMMJJ_HHMMSS.dump
#
# ATTENTION : écrase le contenu actuel de la base par celui de la sauvegarde.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
charger_env
cd "$RACINE_PROJET"

# Vérifie qu'un fichier a bien été fourni en argument.
if [ "$#" -ne 1 ]; then
    echo "Usage : $0 <fichier_de_sauvegarde.dump>" >&2
    echo "Exemple : $0 backups/bibliotheque_20260101_120000.dump" >&2
    exit 1
fi

FICHIER="$1"
if [ ! -f "$FICHIER" ]; then
    echo "Erreur : fichier introuvable : $FICHIER" >&2
    exit 1
fi

echo "ATTENTION : la base « $BDD_NOM » va être ÉCRASÉE par : $FICHIER"
read -r -p "Confirmer la restauration ? [o/N] " reponse
case "$reponse" in
    [oO][uU][iI] | [oO]) ;;
    *)
        echo "Annulé."
        exit 0
        ;;
esac

echo "Restauration en cours..."
# pg_restore restaure un dump au format « custom » DANS le conteneur (superutilisateur
# postgres, connexion locale). Options :
#   --clean --if-exists : supprime les objets existants avant de les recréer
#                         (--if-exists évite les erreurs si un objet manque) ;
#   -d "$BDD_NOM"       : base cible.
$COMPOSE exec -T postgres pg_restore -U postgres -d "$BDD_NOM" --clean --if-exists <"$FICHIER"

echo "Restauration terminée."
