//go:build integration

// Package repository — tests d'INTÉGRATION.
//
// Ces tests dialoguent avec une VRAIE base PostgreSQL de test (par exemple un
// conteneur « postgres-test » exposé sur 127.0.0.1:5432, avec le jeu de données
// de démonstration). Ils ne sont compilés et exécutés QUE lorsque l'étiquette de
// build « integration » est active :
//
//	GOTOOLCHAIN=auto go test -tags=integration ./internal/repository/
//
// Si la base n'est pas joignable, chaque test s'auto-ignore proprement (t.Skip)
// plutôt que d'échouer : on n'impose pas une base à quiconque lance `go test ./...`.
package repository

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/exemple/api-bibliotheque/internal/config"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// envTest lit une variable d'environnement avec une valeur par défaut. Cela rend
// les tests d'intégration flexibles : par défaut ils visent une instance
// PostgreSQL locale sur 127.0.0.1:5432 (le port standard), mais on peut les
// rediriger vers n'importe quelle base (ex. celle de « docker compose », un
// conteneur de test dédié sur un autre port, ou le service PostgreSQL de la CI)
// en définissant BDD_HOTE, BDD_PORT, BDD_MOT_DE_PASSE, etc.
func envTest(cle, defaut string) string {
	if v := os.Getenv(cle); v != "" {
		return v
	}
	return defaut
}

// connecterTest ouvre une connexion vers la base de test. En cas d'échec, le test
// appelant est ignoré (Skip) : indispensable pour ne pas casser une CI sans base.
func connecterTest(t *testing.T) *sql.DB {
	t.Helper()
	port, err := strconv.Atoi(envTest("BDD_PORT", "5432"))
	if err != nil {
		t.Fatalf("BDD_PORT invalide : %v", err)
	}
	cfg := config.BaseDeDonnees{
		Hote:                   envTest("BDD_HOTE", "127.0.0.1"),
		Port:                   port,
		Nom:                    envTest("BDD_NOM", "bibliotheque"),
		Utilisateur:            envTest("BDD_UTILISATEUR", "app_bibliotheque"),
		MotDePasse:             envTest("BDD_MOT_DE_PASSE", "apppwd"),
		MaxConnexionsOuvertes:  5,
		MaxConnexionsInactives: 5,
		DureeVieMaxConnexion:   5 * time.Minute,
		DelaiConnexion:         5 * time.Second,
	}
	db, err := database.Connecter(cfg)
	if err != nil {
		t.Skipf("PostgreSQL de test indisponible : %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// parametresListeTest renvoie des paramètres de liste normalisés (première page).
func parametresListeTest() models.ParametresListe {
	p := models.ParametresListe{Page: 1, Taille: 20, Ordre: models.OrdreAsc, Filtres: map[string]string{}}
	p.Normaliser()
	return p
}

// TestIntegrationUtilisateurParEmail vérifie que le compte administrateur du jeu de
// démonstration est bien lisible par e-mail.
func TestIntegrationUtilisateurParEmail(t *testing.T) {
	db := connecterTest(t)
	repo := NouveauUtilisateurRepository(db)
	ctx := context.Background()

	u, err := repo.ParEmail(ctx, "admin@bibliotheque.fr")
	if err != nil {
		t.Fatalf("ParEmail(admin) a échoué : %v", err)
	}
	if u.Email != "admin@bibliotheque.fr" {
		t.Errorf("Email = %q ; attendu admin@bibliotheque.fr", u.Email)
	}
	if u.Role != models.RoleAdmin {
		t.Errorf("Role = %q ; attendu admin", u.Role)
	}
	if u.UUID == "" {
		t.Error("l'UUID de l'administrateur ne devrait pas être vide")
	}
	if u.MotDePasseHash == "" {
		t.Error("le hash du mot de passe devrait être renseigné (nécessaire au login)")
	}
}

// TestIntegrationLivreLister vérifie que le catalogue de démonstration renvoie des
// livres.
func TestIntegrationLivreLister(t *testing.T) {
	db := connecterTest(t)
	repo := NouveauLivreRepository(db)
	ctx := context.Background()

	livres, total, err := repo.Lister(ctx, parametresListeTest())
	if err != nil {
		t.Fatalf("Lister a échoué : %v", err)
	}
	if total <= 0 {
		t.Errorf("total = %d ; attendu > 0 (le seed contient 28 livres)", total)
	}
	if len(livres) == 0 {
		t.Error("la première page devrait contenir des livres")
	}
	for _, l := range livres {
		if l.UUID == "" || l.Titre == "" {
			t.Errorf("livre mal formé : %+v", l)
		}
	}
}

// TestIntegrationEmprunterEtRendre effectue un cycle complet emprunt → retour sur
// un utilisateur et un livre du seed, puis nettoie l'emprunt créé.
func TestIntegrationEmprunterEtRendre(t *testing.T) {
	db := connecterTest(t)
	ctx := context.Background()

	utilisateurs := NouveauUtilisateurRepository(db)
	livres := NouveauLivreRepository(db)
	emprunts := NouveauEmpruntRepository(db)

	// 1) Emprunteur : l'administrateur (aucun emprunt actif dans le seed, quota OK).
	admin, err := utilisateurs.ParEmail(ctx, "admin@bibliotheque.fr")
	if err != nil {
		t.Fatalf("ParEmail(admin) : %v", err)
	}

	// 2) Un livre effectivement disponible (filtre disponible=true).
	params := parametresListeTest()
	params.Filtres["disponible"] = "true"
	listeDispo, _, err := livres.Lister(ctx, params)
	if err != nil {
		t.Fatalf("Lister(disponible) : %v", err)
	}
	if len(listeDispo) == 0 {
		t.Skip("aucun livre disponible dans le jeu de données : cycle d'emprunt ignoré")
	}
	livreCible := listeDispo[0]

	// 3) Emprunt.
	empruntUUID, err := emprunts.Emprunter(ctx, admin.UUID, livreCible.UUID, 14)
	if err != nil {
		t.Fatalf("Emprunter a échoué : %v", err)
	}
	if empruntUUID == "" {
		t.Fatal("l'UUID de l'emprunt créé ne devrait pas être vide")
	}
	// Nettoyage : une fois l'emprunt rendu (statut « rendu »), la ligne est
	// supprimable (le trigger n'interdit que la suppression d'un emprunt actif).
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), "DELETE FROM emprunts WHERE uuid = $1", empruntUUID); err != nil {
			t.Logf("nettoyage de l'emprunt %s impossible (sera réinitialisé au reseed) : %v", empruntUUID, err)
		}
	})

	// 4) Vérifie que l'emprunt est lisible et rattaché au bon livre.
	emprunt, err := emprunts.ParUUID(ctx, empruntUUID)
	if err != nil {
		t.Fatalf("ParUUID(emprunt) : %v", err)
	}
	if emprunt.LivreUUID != livreCible.UUID {
		t.Errorf("livre de l'emprunt = %q ; attendu %q", emprunt.LivreUUID, livreCible.UUID)
	}
	if emprunt.Statut != models.StatutEnCours {
		t.Errorf("statut = %q ; attendu en_cours", emprunt.Statut)
	}

	// 5) Retour : emprunté et rendu le même jour => aucune pénalité attendue.
	penalite, err := emprunts.Rendre(ctx, empruntUUID)
	if err != nil {
		t.Fatalf("Rendre a échoué : %v", err)
	}
	if penalite != 0 {
		t.Errorf("pénalité = %.2f ; attendu 0 (retour immédiat)", penalite)
	}

	// 6) Un second retour doit être refusé (déjà rendu) → CONFLIT.
	if _, err := emprunts.Rendre(ctx, empruntUUID); err == nil {
		t.Error("un emprunt déjà rendu ne devrait pas pouvoir être rendu une seconde fois")
	}
}
