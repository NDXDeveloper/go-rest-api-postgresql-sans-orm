# Organisation des tests

Les tests de ce projet sont répartis **au plus près du code qu'ils vérifient**
(convention Go : un fichier `xxx_test.go` dans le même paquet que `xxx.go`),
plutôt que regroupés dans un dossier séparé. Ce dossier documente cette  
organisation et sert de point d'entrée.

## Où se trouvent les tests ?

| Type de test | Emplacement | Dépendances |
|--------------|-------------|-------------|
| Unitaires — validation | `internal/validation/validation_test.go` | aucune |
| Unitaires — authentification | `internal/auth/*_test.go` | aucune |
| Unitaires — erreurs applicatives | `internal/apperreur/apperreur_test.go` | aucune |
| Unitaires — réponses HTTP | `internal/reponse/reponse_test.go` | aucune |
| Services (avec mocks) | `internal/service/*_test.go` | aucune (mocks en mémoire) |
| Handlers (httptest) | `internal/handler/*_test.go` | aucune (mocks en mémoire) |
| **Intégration — repositories** | `internal/repository/integration_test.go` | **PostgreSQL réelle** |
| Benchmarks | `internal/validation/*_bench_test.go`, `internal/auth/*_bench_test.go` | aucune |

## Lancer les tests

```bash
# Tous les tests unitaires (rapides, sans base de données), avec détection de data races
make tester              # équivaut à : go test -race -count=1 ./...

# Tests d'INTÉGRATION (nécessitent une base PostgreSQL accessible)
go test -tags=integration ./internal/repository/

# Benchmarks
make banc                # équivaut à : go test -bench=. -benchmem -run=^$ ./...
```

## Configurer la base pour les tests d'intégration

Les tests d'intégration se connectent à une base PostgreSQL **réelle** contenant le  
jeu de données de démonstration. Par défaut ils visent `127.0.0.1:5432` (le port  
standard PostgreSQL), mais tout est configurable par variables d'environnement :

```bash
# Exemple : cibler la base lancée par « docker compose up » (exposée sur 5432)
BDD_HOTE=127.0.0.1 BDD_PORT=5432 \
BDD_UTILISATEUR=app_bibliotheque BDD_MOT_DE_PASSE="<votre mot de passe>" \
go test -tags=integration ./internal/repository/
```

Si aucune base n'est joignable, ces tests s'**auto-ignorent** (`t.Skip`) au lieu  
d'échouer : `go test ./...` reste donc toujours vert, même sans base.

> Astuce : pour une base de test jetable, lancez un conteneur PostgreSQL dédié  
> (par exemple sur un port dédié comme 15432 pour ne pas entrer en conflit avec  
> une instance locale sur le port standard) avec les scripts d'initialisation de  
> `sql/`, ou réutilisez la pile Docker Compose du projet.
