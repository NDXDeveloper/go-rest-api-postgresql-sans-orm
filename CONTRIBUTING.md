# Contribuer au projet API Bibliothèque

Merci de l'intérêt que vous portez à ce projet ! Il a une vocation **pédagogique** :  
toute contribution qui améliore la clarté, la correction ou la couverture de tests  
du code est la bienvenue, qu'il s'agisse d'un correctif, d'une nouvelle  
fonctionnalité ou d'une amélioration de la documentation.

Ce guide décrit comment mettre en place l'environnement de développement, le  
workflow de contribution attendu, les conventions de code et de commit, ainsi  
que la checklist à vérifier avant d'ouvrir une Pull Request.

## Table des matières

- [Prérequis](#prérequis)
- [Mise en place locale](#mise-en-place-locale)
- [Workflow de contribution](#workflow-de-contribution)
- [Style de code Go](#style-de-code-go)
- [Convention des messages de commit](#convention-des-messages-de-commit)
- [Exécuter les tests](#exécuter-les-tests)
- [Checklist avant Pull Request](#checklist-avant-pull-request)

## Prérequis

- **Go 1.25** ou supérieur (voir `go.mod`). Avec `GOTOOLCHAIN=auto` (comportement
  par défaut de Go), la bonne version du compilateur est téléchargée
  automatiquement si nécessaire.
- **Docker** et **Docker Compose v2** (`docker compose version`) pour lancer la
  pile complète (API + PostgreSQL).
- **make** (optionnel mais recommandé) : le `Makefile` fourni regroupe toutes
  les commandes courantes (`make aide` pour la liste complète).
- Un client PostgreSQL (`psql`, optionnel), pratique pour inspecter la base
  pendant le développement.
- `golangci-lint` et `staticcheck` en local si vous voulez reproduire
  exactement les vérifications de la CI (voir [Style de code Go](#style-de-code-go)).

## Mise en place locale

```bash
# 1. Récupérez le dépôt (ou votre fork, voir le workflow ci-dessous)
git clone <url-de-votre-fork>
cd go-rest-api-postgresql-sans-orm

# 2. Copiez le modèle de configuration et adaptez les secrets locaux
cp .env.example .env

# 3. Démarrez la pile complète (API + PostgreSQL), schéma et données de départ
#    chargés automatiquement au premier démarrage
make demarrer

# 4. Vérifiez que l'API répond
curl http://localhost:8080/health

# 5. Suivez les journaux de l'API
make journaux

# 6. Arrêtez la pile (les données sont conservées)
make arreter
```

Vous pouvez aussi lancer l'API directement avec `go run ./cmd/api` (sans  
Docker) si vous disposez d'une instance PostgreSQL accessible : dans ce cas,  
adaptez les variables `BDD_HOTE` / `BDD_PORT` de votre `.env` pour qu'elles  
pointent vers cette instance.

## Workflow de contribution

1. Pour un changement conséquent, ouvrez d'abord une **issue** (bogue ou
   fonctionnalité) en utilisant les modèles fournis, afin de discuter de
   l'approche avant d'investir du temps dans le code.
2. **Forkez** le dépôt puis clonez votre fork.
3. Créez une **branche** depuis `main`, avec un nom explicite :
   `type/courte-description` (ex. `feat/recherche-isbn`,
   `fix/expiration-jeton`).
4. Développez votre changement, en respectant le [style de code](#style-de-code-go)
   et en ajoutant les tests correspondants.
5. Committez avec des messages suivant la [convention décrite plus bas](#convention-des-messages-de-commit).
6. Poussez votre branche sur votre fork et ouvrez une **Pull Request** vers la
   branche `main` du dépôt d'origine, en remplissant le modèle de PR fourni.
7. Assurez-vous que la CI (GitHub Actions) passe au vert et répondez aux
   retours de relecture.
8. Une fois la PR approuvée et la CI verte, elle est fusionnée par un
   mainteneur.

## Style de code Go

Le code doit passer, sans erreur, l'ensemble des vérifications suivantes
(identiques à celles exécutées par la CI, voir `.github/workflows/ci.yml`) :

- **`gofmt`** : formatage standard. Lancez `make formater` (ou `gofmt -w .`)
  avant de committer.
- **`go vet`** : analyse statique du compilateur. `make vet`.
- **`golangci-lint`** : agrège plusieurs analyseurs (`errcheck`, `govet`,
  `ineffassign`, `staticcheck`, `unused`, `revive`, `bodyclose`, `gosec`,
  `gofmt` — voir `.golangci.yml`). `make lint`.
- **`staticcheck`** : analyse avancée, exécutée aussi de façon autonome en CI.

Un contrôle rapide avant de committer :

```bash
make formater vet lint
```

### Nommage : vocabulaire métier en français

Ce projet fait un choix pédagogique assumé : le **vocabulaire du domaine  
métier** (types, champs, fonctions, noms de packages liés à la bibliothèque —
`livre`, `emprunt`, `auteur`, `EmprunterLivre`, `NombreExemplaires`...) est en
**français**, pour rester lisible et proche du langage métier pour un public
francophone.

En revanche, restent en anglais, comme dans tout code Go idiomatique :

- les mots-clés du langage et les identifiants de la bibliothèque standard
  (`context.Context`, `http.Request`, `error`, `interface`...) ;
- les identifiants exposés par les dépendances tierces ;
- les conventions Go universelles (`String()`, `Error()`, noms de méthodes
  d'interfaces standard...).

Merci de garder cette cohérence dans vos contributions plutôt que de mélanger  
les deux vocabulaires au sein d'un même paquet.

Tout élément **exporté** (commençant par une majuscule) doit porter un  
commentaire godoc (imposé par la règle `revive: exported` de `.golangci.yml`).

## Convention des messages de commit

Ce projet suit les **Commits Conventionnels** (*Conventional Commits*),  
rédigés **en français** :

```
<type>(<portée>) : <description au présent, sans majuscule ni point final>
```

Types courants :

| Type       | Usage                                                          |
|------------|-----------------------------------------------------------------|
| `feat`     | nouvelle fonctionnalité                                         |
| `fix`      | correction de bogue                                             |
| `docs`     | documentation uniquement                                        |
| `style`    | mise en forme (sans changement de comportement)                 |
| `refactor` | remaniement du code sans changement de comportement observable  |
| `perf`     | amélioration de performance                                     |
| `test`     | ajout ou correction de tests                                    |
| `chore`    | tâche d'entretien (dépendances, configuration...)                |
| `ci`       | changement dans les workflows d'intégration continue            |
| `build`    | changement affectant le système de build (Dockerfile, Makefile) |

Exemples :

```
feat(livres): ajouter la recherche par ISBN
fix(auth): corriger l'expiration du jeton
docs(readme): clarifier la procédure d'installation locale
refactor(repository): simplifier les requêtes préparées des emprunts
test(service): ajouter des cas limites pour l'emprunt de livres
chore(deps): mettre à jour golang-jwt vers v5.3.1
ci(workflow): ajouter l'étape staticcheck
```

Pour un changement de rupture, ajoutez un `!` après le type/portée (ex.
`feat(auth)!: modifier le format de réponse du jeton`) et détaillez la rupture
dans le corps du message (section `BREAKING CHANGE: ...`).

## Exécuter les tests

```bash
make tester          # tous les tests, avec détection des data races
make tester-court     # tests rapides uniquement (go test -short)
make couverture       # rapport de couverture HTML (coverage.html)
make banc             # benchmarks
```

### Tests d'intégration

Certains tests portent le tag de compilation `integration` et nécessitent une  
base PostgreSQL disponible. Démarrez d'abord la base (par exemple via
`make demarrer`, ou `docker compose up -d postgres`), puis lancez :

```bash
go test -tags=integration -race -count=1 ./...
```

Assurez-vous que les variables d'environnement de connexion (`BDD_HOTE`,
`BDD_PORT`, `BDD_NOM`, `BDD_UTILISATEUR`, `BDD_MOT_DE_PASSE`...) correspondent
à votre instance PostgreSQL locale (voir `.env.example`).

## Checklist avant Pull Request

- [ ] Le projet compile (`make compiler` ou `go build ./...`).
- [ ] `gofmt` est appliqué : `gofmt -l .` ne renvoie aucun fichier.
- [ ] `go vet ./...` ne remonte aucune erreur.
- [ ] `golangci-lint run ./...` ne remonte aucune erreur.
- [ ] Les tests passent (`make tester`) et, si le changement le justifie, les
      tests d'intégration également.
- [ ] Des tests ont été ajoutés ou mis à jour pour couvrir le changement.
- [ ] La documentation pertinente (README, `docs/`, commentaires godoc,
      `CHANGELOG.md`) a été mise à jour.
- [ ] Aucun secret, mot de passe ou fichier `.env` n'a été committé.
- [ ] Les messages de commit suivent la convention décrite ci-dessus.
- [ ] La Pull Request utilise le modèle fourni et référence l'issue liée, le
      cas échéant.

Merci encore pour votre contribution ! En participant à ce projet, vous  
acceptez de respecter notre [Code de conduite](CODE_OF_CONDUCT.md). Pour  
signaler une faille de sécurité, ne passez pas par une issue publique : suivez  
la procédure décrite dans [SECURITY.md](SECURITY.md). Vos contributions sont  
distribuées sous la licence MIT du projet (voir [LICENSE](LICENSE)).
