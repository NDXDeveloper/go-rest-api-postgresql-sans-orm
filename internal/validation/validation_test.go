package validation

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
)

// TestEstEmailValide vérifie l'analyseur d'adresses e-mail (basé sur net/mail),
// notamment le rejet des noms d'affichage (« Jean <j@ex.fr> ») et des adresses
// syntaxiquement incorrectes.
func TestEstEmailValide(t *testing.T) {
	cas := []struct {
		nom     string
		entree  string
		attendu bool
	}{
		{"adresse simple valide", "jean.dupont@exemple.fr", true},
		{"adresse avec sous-domaine", "a@mail.exemple.fr", true},
		{"espaces superflus tolérés (rognés)", "  jean@exemple.fr  ", true},
		{"chaîne vide", "", false},
		{"sans arobase", "jeanexemple.fr", false},
		{"partie locale absente", "@exemple.fr", false},
		{"domaine absent", "jean@", false},
		{"nom d'affichage refusé", "Jean <jean@exemple.fr>", false},
		{"double arobase", "jean@ex@emple.fr", false},
		{"espace interne", "jean dupont@exemple.fr", false},
		{"trop longue (> 254)", strings.Repeat("a", 250) + "@exemple.fr", false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := EstEmailValide(c.entree); got != c.attendu {
				t.Errorf("EstEmailValide(%q) = %v ; attendu %v", c.entree, got, c.attendu)
			}
		})
	}
}

// TestEstUUIDValide vérifie la reconnaissance du format UUID.
func TestEstUUIDValide(t *testing.T) {
	cas := []struct {
		nom     string
		entree  string
		attendu bool
	}{
		{"UUID v4 valide", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", true},
		{"UUID en majuscules", "3F2504E0-4F89-41D3-9A0C-0305E82C3301", true},
		{"chaîne vide", "", false},
		{"texte quelconque", "pas-un-uuid", false},
		{"sans tirets", "3f2504e04f8941d39a0c0305e82c3301", false},
		{"trop court", "3f2504e0-4f89-41d3-9a0c", false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := EstUUIDValide(c.entree); got != c.attendu {
				t.Errorf("EstUUIDValide(%q) = %v ; attendu %v", c.entree, got, c.attendu)
			}
		})
	}
}

// TestEstISBN13Valide vérifie la validation d'un ISBN-13 AVEC sa clé de contrôle.
// On inclut volontairement des ISBN dont la clé est fausse (donc invalides) et un
// ISBN parfaitement valide (9782010000003).
func TestEstISBN13Valide(t *testing.T) {
	cas := []struct {
		nom     string
		entree  string
		attendu bool
	}{
		{"ISBN valide (jeu de démonstration)", "9782010000003", true},
		{"ISBN valide avec tirets", "978-0-306-40615-7", true},
		{"ISBN valide avec espaces", "978 0 306 40615 7", true},
		{"clé de contrôle fausse", "9782010000004", false},
		{"autre clé fausse", "9780306406158", false},
		{"trop court (12 chiffres)", "978201000000", false},
		{"trop long (14 chiffres)", "97820100000031", false},
		{"contient une lettre", "978201000000X", false},
		{"chaîne vide", "", false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := EstISBN13Valide(c.entree); got != c.attendu {
				t.Errorf("EstISBN13Valide(%q) = %v ; attendu %v", c.entree, got, c.attendu)
			}
		})
	}
}

// TestEstDateISOValide vérifie que l'on rejette les dates impossibles (ex. 31
// février) tout en acceptant les vraies dates ISO « AAAA-MM-JJ ».
func TestEstDateISOValide(t *testing.T) {
	cas := []struct {
		nom     string
		entree  string
		attendu bool
	}{
		{"date valide", "2023-01-15", true},
		{"29 février année bissextile", "2024-02-29", true},
		{"31 février impossible", "2023-02-31", false},
		{"29 février année non bissextile", "2023-02-29", false},
		{"mois 13 impossible", "2023-13-01", false},
		{"jour 00 impossible", "2023-01-00", false},
		{"mauvais format (sans zéro)", "2023-1-1", false},
		{"format non ISO", "15/01/2023", false},
		{"chaîne vide", "", false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := EstDateISOValide(c.entree); got != c.attendu {
				t.Errorf("EstDateISOValide(%q) = %v ; attendu %v", c.entree, got, c.attendu)
			}
		})
	}
}

// TestNormaliserISBN vérifie que tirets et espaces sont retirés pour obtenir la
// forme canonique à 13 chiffres.
func TestNormaliserISBN(t *testing.T) {
	cas := []struct {
		entree  string
		attendu string
	}{
		{"978-2-01-000000-3", "9782010000003"},
		{"978 2 01 000000 3", "9782010000003"},
		{"9782010000003", "9782010000003"},
	}
	for _, c := range cas {
		t.Run(c.entree, func(t *testing.T) {
			if got := NormaliserISBN(c.entree); got != c.attendu {
				t.Errorf("NormaliserISBN(%q) = %q ; attendu %q", c.entree, got, c.attendu)
			}
		})
	}
}

// TestValidateurReglesUnitaires exerce chaque règle de commodité prise isolément.
func TestValidateurReglesUnitaires(t *testing.T) {
	t.Run("ChampRequis rejette une chaîne d'espaces", func(t *testing.T) {
		v := Nouveau()
		v.ChampRequis("nom", "   ")
		if v.EstValide() {
			t.Fatal("une chaîne d'espaces aurait dû être refusée")
		}
	})

	t.Run("ChampRequis accepte une valeur non vide", func(t *testing.T) {
		v := Nouveau()
		v.ChampRequis("nom", "Victor")
		if !v.EstValide() {
			t.Fatal("une valeur non vide aurait dû être acceptée")
		}
	})

	t.Run("LongueurMax", func(t *testing.T) {
		v := Nouveau()
		v.LongueurMax("titre", "abcdef", 3)
		if v.EstValide() {
			t.Fatal("une chaîne trop longue aurait dû être refusée")
		}
	})

	t.Run("LongueurMin", func(t *testing.T) {
		v := Nouveau()
		v.LongueurMin("mot_de_passe", "abc", 8)
		if v.EstValide() {
			t.Fatal("une chaîne trop courte aurait dû être refusée")
		}
	})

	t.Run("LongueurMax compte les runes et non les octets", func(t *testing.T) {
		v := Nouveau()
		// « éàç » = 3 caractères mais 6 octets en UTF-8 : la limite porte sur les runes.
		v.LongueurMax("nom", "éàç", 3)
		if !v.EstValide() {
			t.Fatal("3 caractères accentués doivent respecter une limite de 3")
		}
	})

	t.Run("Email invalide", func(t *testing.T) {
		v := Nouveau()
		v.Email("email", "pas-un-email")
		if v.EstValide() {
			t.Fatal("un e-mail invalide aurait dû être refusé")
		}
	})

	t.Run("UUID invalide", func(t *testing.T) {
		v := Nouveau()
		v.UUID("id", "xxx")
		if v.EstValide() {
			t.Fatal("un UUID invalide aurait dû être refusé")
		}
	})

	t.Run("ISBN13 invalide", func(t *testing.T) {
		v := Nouveau()
		v.ISBN13("isbn", "9782010000004")
		if v.EstValide() {
			t.Fatal("un ISBN à clé fausse aurait dû être refusé")
		}
	})

	t.Run("EntierDans hors bornes", func(t *testing.T) {
		v := Nouveau()
		v.EntierDans("annee", 1200, 1400, 2200)
		if v.EstValide() {
			t.Fatal("une valeur hors intervalle aurait dû être refusée")
		}
	})

	t.Run("EntierDans dans les bornes", func(t *testing.T) {
		v := Nouveau()
		v.EntierDans("annee", 2000, 1400, 2200)
		if !v.EstValide() {
			t.Fatal("une valeur dans l'intervalle aurait dû être acceptée")
		}
	})

	t.Run("DansEnsemble valeur non autorisée", func(t *testing.T) {
		v := Nouveau()
		v.DansEnsemble("role", "pirate", "admin", "membre")
		if v.EstValide() {
			t.Fatal("une valeur hors ensemble aurait dû être refusée")
		}
	})

	t.Run("DansEnsemble valeur autorisée", func(t *testing.T) {
		v := Nouveau()
		v.DansEnsemble("role", "membre", "admin", "membre")
		if !v.EstValide() {
			t.Fatal("une valeur de l'ensemble aurait dû être acceptée")
		}
	})
}

// TestValidateurAccumulation vérifie que plusieurs erreurs sont collectées, que la
// première erreur d'un champ est conservée, et que Erreur() renvoie bien une
// *apperreur.Erreur de code VALIDATION (HTTP 422) avec les détails champ par champ.
func TestValidateurAccumulation(t *testing.T) {
	v := Nouveau()
	v.ChampRequis("email", "")
	v.Email("email", "invalide") // second problème sur « email » : ne doit pas écraser le premier
	v.ChampRequis("nom", "")
	v.EntierDans("annee", 3000, 1400, 2200)

	err := v.Erreur()
	if err == nil {
		t.Fatal("Erreur() aurait dû renvoyer une erreur non nil")
	}

	var appErr *apperreur.Erreur
	if !errors.As(err, &appErr) {
		t.Fatalf("Erreur() aurait dû renvoyer une *apperreur.Erreur, obtenu %T", err)
	}
	if appErr.Code != apperreur.CodeValidation {
		t.Errorf("code = %q ; attendu %q", appErr.Code, apperreur.CodeValidation)
	}
	if appErr.StatutHTTP != http.StatusUnprocessableEntity {
		t.Errorf("StatutHTTP = %d ; attendu %d", appErr.StatutHTTP, http.StatusUnprocessableEntity)
	}
	// Trois champs distincts en erreur (email, nom, annee).
	if len(appErr.Details) != 3 {
		t.Errorf("nombre de champs en erreur = %d ; attendu 3 (%v)", len(appErr.Details), appErr.Details)
	}
	// La PREMIÈRE erreur sur « email » (champ obligatoire) doit être conservée.
	if appErr.Details["email"] != "ce champ est obligatoire" {
		t.Errorf("première erreur du champ email non conservée : %q", appErr.Details["email"])
	}
}

// TestValidateurValideRenvoieNil vérifie qu'un validateur sans erreur renvoie nil.
func TestValidateurValideRenvoieNil(t *testing.T) {
	v := Nouveau()
	v.ChampRequis("nom", "Victor")
	v.Email("email", "victor@exemple.fr")
	if !v.EstValide() {
		t.Fatal("le validateur aurait dû être valide")
	}
	if err := v.Erreur(); err != nil {
		t.Fatalf("Erreur() aurait dû renvoyer nil, obtenu %v", err)
	}
}
