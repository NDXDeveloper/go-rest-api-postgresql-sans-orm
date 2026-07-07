---
name: "Rapport de bogue"  
about: "Signaler un comportement inattendu ou un dysfonctionnement de l'API"  
title: "[BOGUE] "  
labels: [bogue]  
assignees: ''
---

## Description

Une description claire et concise du bogue constaté.

## Étapes de reproduction

Étapes permettant de reproduire le comportement :

1. Aller à '...'
2. Exécuter la commande ou envoyer la requête '...'
3. Observer '...'

```bash
# Exemple de requête ayant déclenché le problème (adaptez ou supprimez)
curl -i -X GET http://localhost:8080/...
```

## Comportement attendu

Une description claire de ce qui aurait dû se produire.

## Comportement observé

Ce qui s'est réellement produit (code HTTP, message d'erreur, corps de la  
réponse JSON...).

## Environnement

- Système d'exploitation :
- Version de Go (`go version`) :
- Version de Docker (`docker --version`) :
- Version de Docker Compose (`docker compose version`) :
- Version ou commit du projet concerné :

## Journaux

Collez ici les journaux pertinents (par exemple la sortie de
`docker compose logs -f api`), entre les balises ci-dessous :

```
(journaux ici)
```

## Contexte additionnel

Toute autre information utile : capture d'écran, configuration particulière,  
piste de correction déjà envisagée...
