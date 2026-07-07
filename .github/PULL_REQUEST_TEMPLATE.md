## Description

Décrivez clairement les changements apportés par cette Pull Request et leur  
motivation.

## Issue liée

Closes #

## Type de changement

- [ ] Correctif (correction d'un bogue, sans rupture de compatibilité)
- [ ] Fonctionnalité (ajout de fonctionnalité, sans rupture de compatibilité)
- [ ] Rupture (« *breaking change* » : modifie un comportement existant)
- [ ] Documentation (changement concernant uniquement la documentation)

## Checklist

- [ ] J'ai ajouté des tests couvrant mes changements, et l'ensemble des tests
      passe (`make tester`)
- [ ] `gofmt`, `go vet` et `golangci-lint` passent sans erreur
      (`make formater vet lint`)
- [ ] La documentation (README, `docs/`, commentaires godoc, `CHANGELOG.md`)
      a été mise à jour si nécessaire
- [ ] Aucun secret, mot de passe ou fichier `.env` n'a été committé
- [ ] Mes messages de commit suivent la convention décrite dans
      [CONTRIBUTING.md](../CONTRIBUTING.md)

## Contexte additionnel

Toute information utile à la relecture : choix d'implémentation, points  
d'attention particuliers, captures d'écran...
