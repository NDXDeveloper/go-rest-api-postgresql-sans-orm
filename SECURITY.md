# Politique de sécurité

Ce document décrit les versions actuellement suivies en matière de sécurité  
et la procédure à suivre pour signaler une vulnérabilité de manière  
responsable.

## Versions supportées

| Version | Supportée          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

Seule la dernière version mineure de la branche `1.0.x` reçoit des correctifs  
de sécurité. Les utilisateurs sont invités à toujours se maintenir à jour sur  
la dernière version publiée (voir [CHANGELOG.md](CHANGELOG.md)).

## Signaler une vulnérabilité

La sécurité de ce projet est prise au sérieux, y compris dans un cadre  
pédagogique. Si vous découvrez une faille de sécurité, merci de nous aider à  
la corriger avant qu'elle ne soit rendue publique, en suivant une démarche de
**divulgation responsable**.

**Merci de ne PAS ouvrir d'issue publique, de discussion ou de Pull Request**
décrivant la vulnérabilité : cela l'exposerait avant qu'un correctif ne soit  
disponible.

Signalez plutôt la vulnérabilité par e-mail, directement à :

**securite@exemple.fr**

### Informations à fournir

Pour nous permettre d'évaluer et de reproduire le problème le plus rapidement  
possible, merci d'inclure autant que possible :

- une description claire du problème et de son impact potentiel ;
- les étapes de reproduction détaillées, idéalement une preuve de concept
  (requête HTTP, jeu de données, script...) ;
- la ou les version(s), tag(s) ou commit(s) concerné(s) ;
- l'environnement concerné (système d'exploitation, version de Go, version de
  Docker / Docker Compose, version de PostgreSQL) ;
- toute suggestion de correctif ou d'atténuation, si vous en avez une.

### Délais de réponse indicatifs

- **Accusé de réception** : sous 48 heures ouvrées.
- **Évaluation initiale** (confirmation, sévérité, périmètre) : sous 7 jours.
- **Correctif ou plan d'action** : selon la sévérité, généralement sous 30 à
  90 jours.

Ces délais sont indicatifs et peuvent varier selon la complexité du problème  
signalé ; nous nous engageons dans tous les cas à vous tenir informé·e de  
l'avancement.

### Engagement de divulgation coordonnée

En retour d'un signalement effectué de bonne foi et dans le respect de cette  
politique, nous nous engageons à :

- accuser réception de votre signalement rapidement ;
- vous tenir informé·e de l'avancement du traitement ;
- travailler avec vous pour comprendre et confirmer le problème ;
- vous créditer pour la découverte (sauf si vous préférez rester anonyme),
  une fois le correctif publié ;
- ne pas engager de poursuites à l'encontre des personnes ayant signalé une
  vulnérabilité de bonne foi, en respectant cette procédure.

En contrepartie, nous vous demandons de nous accorder un délai raisonnable
(idéalement 90 jours à compter du signalement, ou tout autre délai convenu
d'un commun accord) avant toute divulgation publique, le temps de développer,  
tester et déployer un correctif.

Merci de contribuer à la sécurité de ce projet.
