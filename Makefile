# =============================================================================
# Makefile — Raccourcis pour les tâches courantes de développement
# -----------------------------------------------------------------------------
# Utilisation : « make <cible> », par exemple « make tester » ou « make demarrer ».
# « make aide » (ou simplement « make ») affiche la liste des cibles.
# =============================================================================

# Nom du binaire produit.
BINAIRE := api
# Chemin du paquet principal.
CHEMIN_MAIN := ./cmd/api

# .DEFAULT_GOAL fixe la cible exécutée quand on tape « make » sans argument.
.DEFAULT_GOAL := aide

# .PHONY déclare les cibles qui ne correspondent pas à un fichier (évite les
# conflits si un fichier du même nom existe).
.PHONY: aide compiler executer tester tester-court couverture banc \
        formater vet lint verifier proprifier \
        demarrer arreter reconstruire journaux nettoyer

aide: ## Affiche cette aide
	@echo "Cibles disponibles :"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# --- Go ----------------------------------------------------------------------
compiler: ## Compile le binaire dans ./bin
	@mkdir -p bin
	go build -ldflags="-s -w" -o bin/$(BINAIRE) $(CHEMIN_MAIN)

executer: ## Lance l'application en local (nécessite un .env)
	go run $(CHEMIN_MAIN)

tester: ## Lance tous les tests avec la détection de data races
	go test -race -count=1 ./...

tester-court: ## Lance uniquement les tests rapides (sans intégration)
	go test -short -count=1 ./...

couverture: ## Génère un rapport de couverture HTML (coverage.html)
	go test -covermode=atomic -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Rapport : coverage.html"

banc: ## Exécute les benchmarks
	go test -bench=. -benchmem -run=^$$ ./...

# --- Qualité -----------------------------------------------------------------
formater: ## Formate le code (gofmt)
	gofmt -w .

vet: ## Analyse statique du compilateur (go vet)
	go vet ./...

lint: ## Lance golangci-lint (doit être installé)
	golangci-lint run ./...

verifier: formater vet ## Formate, vet et compile (contrôle rapide avant commit)
	go build ./...

proprifier: ## Nettoie et met à jour go.mod / go.sum
	go mod tidy

# --- Docker ------------------------------------------------------------------
demarrer: ## Démarre la pile (API + MariaDB) en arrière-plan
	docker compose up -d --build

arreter: ## Arrête la pile sans supprimer les données
	docker compose down

reconstruire: ## Reconstruit les images et redémarre
	docker compose up -d --build --force-recreate

journaux: ## Affiche les journaux de l'API en continu
	docker compose logs -f api

nettoyer: ## Arrête la pile ET supprime les volumes (DONNÉES PERDUES)
	docker compose down -v
