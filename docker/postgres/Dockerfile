# =============================================================================
# docker/postgres/Dockerfile — Image PostgreSQL 18 + extension pg_cron
# -----------------------------------------------------------------------------
# L'image officielle « postgres » n'embarque PAS pg_cron (contrairement à MariaDB
# dont l'Event Scheduler est intégré). On étend donc l'image pour installer le
# paquet de l'extension. pg_cron doit ensuite être PRÉCHARGÉE au démarrage
# (shared_preload_libraries) : c'est fait via le « command » du docker-compose.
# =============================================================================
FROM postgres:18

# Installation du paquet pg_cron correspondant à la version majeure (18) du serveur.
# On nettoie le cache apt dans la même couche pour garder l'image légère.
# DEBIAN_FRONTEND=noninteractive évite toute invite pendant le build.
RUN set -eux; \
    apt-get update; \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        postgresql-18-cron; \
    rm -rf /var/lib/apt/lists/*

# Aucune autre modification : l'image conserve le point d'entrée officiel de
# postgres (initialisation, exécution des scripts de /docker-entrypoint-initdb.d/,
# healthcheck via pg_isready...). La configuration (shared_preload_libraries,
# cron.database_name) est fournie au lancement par docker-compose.yml.
