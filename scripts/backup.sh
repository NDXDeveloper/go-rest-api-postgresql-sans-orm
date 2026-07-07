#!/usr/bin/env bash
# =============================================================================
# backup.sh — Sauvegarde la base de données dans un fichier compressé
# -----------------------------------------------------------------------------
# Produit un dump PostgreSQL au format « custom » (-Fc) : compressé, portable et
# restaurable avec pg_restore (voir restore.sh). Le fichier est horodaté et rangé
# dans le dossier backups/.
# =============================================================================
source "$(dirname "$0")/_commun.sh"
detecter_compose
charger_env
cd "$RACINE_PROJET"

DOSSIER_SAUVEGARDE="$RACINE_PROJET/backups"
mkdir -p "$DOSSIER_SAUVEGARDE"

HORODATAGE="$(date +%Y%m%d_%H%M%S)"
FICHIER="$DOSSIER_SAUVEGARDE/bibliotheque_${HORODATAGE}.dump"

echo "Sauvegarde de la base « $BDD_NOM » en cours..."

# pg_dump s'exécute DANS le conteneur, en tant que superutilisateur « postgres »
# (connexion locale par socket, donc sans mot de passe). Options importantes :
#   -Fc  : format « custom » (compressé, restaurable sélectivement avec pg_restore) ;
#   --no-owner / --no-privileges pourraient être ajoutés pour un dump plus portable.
# Le dump inclut par défaut le schéma, les données, les fonctions, procédures,
# triggers et vues.
$COMPOSE exec -T postgres pg_dump -U postgres -Fc "$BDD_NOM" >"$FICHIER"

echo "Sauvegarde terminée : $FICHIER"
echo "Taille : $(du -h "$FICHIER" | cut -f1)"
