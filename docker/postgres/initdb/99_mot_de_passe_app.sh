#!/bin/bash
# =============================================================================
# 99_mot_de_passe_app.sh — Applique le mot de passe RÉEL du rôle applicatif
# -----------------------------------------------------------------------------
# CONTEXTE : le script SQL 01_roles.sql crée le rôle « app_bibliotheque » avec un
# mot de passe de développement par défaut. Ce script d'initialisation (exécuté en
# DERNIER, en tant que superutilisateur) remplace ce mot de passe par le SECRET
# fourni via la variable d'environnement APP_DB_PASSWORD.
#
# Pourquoi un script shell ? Parce que seuls les scripts .sh de l'image PostgreSQL
# ont accès aux variables d'environnement du conteneur. On évite ainsi d'écrire le
# secret en dur dans un fichier SQL versionné.
#
# « set -e » : on s'arrête à la première erreur. Les variables POSTGRES_USER et
# POSTGRES_DB sont fournies par l'image officielle.
# =============================================================================
set -e

# Valeur de repli identique au défaut du script SQL, au cas où la variable ne
# serait pas fournie (démarrage sans .env).
MOT_DE_PASSE="${APP_DB_PASSWORD:-changez_moi_app}"

# psql lit la commande sur son entrée standard. On passe le mot de passe via une
# variable psql (-v) et on l'insère avec quote_literal pour un échappement correct.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
     -v motdepasse="$MOT_DE_PASSE" <<-'EOSQL'
    -- format('...%s...') + quote_literal : échappement sûr du mot de passe.
    SELECT format('ALTER ROLE app_bibliotheque PASSWORD %L', :'motdepasse') \gexec
EOSQL

echo "Mot de passe du rôle app_bibliotheque appliqué depuis l'environnement."
