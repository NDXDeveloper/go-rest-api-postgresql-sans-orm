# Migrations SQL

Ce dossier illustre la notion de **migrations** : des évolutions **incrémentales** et
**versionnées** du schéma, à appliquer dans l'ordre sur une base **existante** contenant
déjà des données de production.

## Init (développement) vs Migrations (production)

Ce projet utilise deux approches complémentaires :

| Aspect            | Scripts d'init (`sql/schema`, etc.)              | Migrations (`sql/migrations`)                 |
|-------------------|-------------------------------------------------|-----------------------------------------------|
| Quand             | Au **premier** démarrage d'une base **vierge**  | Sur une base **déjà en service**              |
| Contenu           | `CREATE DATABASE`, `CREATE TABLE`, seed…         | `ALTER TABLE`, ajout d'index, backfill…       |
| Destructif ?      | Peut l'être (`DROP` en dev)                      | **Jamais** (on ne perd pas les données prod)  |
| Rejouable ?       | Sur base vierge                                  | **Une seule fois**, dans l'ordre des versions |
| Où, dans Docker   | `/docker-entrypoint-initdb.d/` (1er boot)        | Appliquées manuellement ou via un outil       |

> En clair : les scripts de `sql/schema` servent à **repartir de zéro** (idéal en  
> formation/développement). Les migrations servent à **faire évoluer** une base qui  
> tourne déjà, sans jamais détruire l'existant.

## Convention de nommage

`Vxxx__description.sql`, où `xxx` est un numéro de version croissant :

```
V001__schema_initial.sql
V002__ajout_colonne_langue_par_defaut.sql
```

On applique les fichiers **dans l'ordre croissant**, une seule fois chacun. En  
production réelle, on s'appuierait sur un outil dédié (Flyway, golang-migrate,  
Liquibase…) qui mémorise les versions déjà appliquées dans une table de suivi.  
Ici, on reste volontairement manuel et transparent, pour bien comprendre le principe.

## Appliquer une migration manuellement

```bash
docker compose exec -T mariadb \
  mariadb -u root -p"$MARIADB_ROOT_PASSWORD" bibliotheque \
  < sql/migrations/V002__ajout_colonne_langue_par_defaut.sql
```
