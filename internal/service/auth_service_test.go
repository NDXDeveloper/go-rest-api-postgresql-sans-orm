package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/config"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/google/uuid"
)

// gestionnaireJWTTest fabrique un GestionnaireJWT utilisable dans les tests de
// service (secret d'au moins 32 caractères).
func gestionnaireJWTTest() *auth.GestionnaireJWT {
	return auth.NouveauGestionnaireJWT(config.JWT{
		Secret:                "secret-de-test-suffisamment-long-0123456789",
		Emetteur:              "api-bibliotheque-test",
		DureeAcces:            15 * time.Minute,
		DureeRafraichissement: 24 * time.Hour,
	})
}

// codeErreur extrait le code métier d'une erreur applicative pour les assertions.
// Échoue le test si l'erreur n'est pas une *apperreur.Erreur.
func codeErreur(t *testing.T, err error) *apperreur.Erreur {
	t.Helper()
	if err == nil {
		t.Fatal("une erreur était attendue, obtenu nil")
	}
	var appErr *apperreur.Erreur
	if !errors.As(err, &appErr) {
		t.Fatalf("erreur de type %T ; attendu *apperreur.Erreur", err)
	}
	return appErr
}

// TestInscription vérifie la création d'un compte membre et le rejet d'un e-mail
// invalide.
func TestInscription(t *testing.T) {
	entreeValide := models.InscriptionEntree{
		Email:      "Nouveau.Membre@Exemple.FR",
		MotDePasse: "MotDePasse123!",
		Nom:        "Durand",
		Prenom:     "Chloé",
	}

	t.Run("le rôle est forcé à « membre » et le compte est actif", func(t *testing.T) {
		var capture *models.Utilisateur
		uRepo := &mockUtilisateurRepo{
			creer: func(_ context.Context, u *models.Utilisateur) error {
				capture = u
				u.ID = 100
				return nil
			},
		}
		svc := NouveauAuthService(uRepo, &mockJetonRepo{}, gestionnaireJWTTest())

		utilisateur, err := svc.Inscription(context.Background(), entreeValide)
		if err != nil {
			t.Fatalf("Inscription a échoué : %v", err)
		}
		if utilisateur.Role != models.RoleMembre {
			t.Errorf("Role = %q ; attendu « membre » (protection anti-Mass-Assignment)", utilisateur.Role)
		}
		if !utilisateur.Actif {
			t.Error("un nouvel inscrit devrait être actif")
		}
		// L'e-mail doit être normalisé en minuscules et sans espaces.
		if utilisateur.Email != "nouveau.membre@exemple.fr" {
			t.Errorf("Email = %q ; attendu normalisé en minuscules", utilisateur.Email)
		}
		// Le mot de passe stocké doit être haché (jamais en clair).
		if capture == nil || capture.MotDePasseHash == "" || capture.MotDePasseHash == entreeValide.MotDePasse {
			t.Error("le mot de passe aurait dû être haché avant persistance")
		}
	})

	t.Run("email invalide → VALIDATION", func(t *testing.T) {
		svc := NouveauAuthService(&mockUtilisateurRepo{}, &mockJetonRepo{}, gestionnaireJWTTest())

		entree := entreeValide
		entree.Email = "pas-un-email"

		_, err := svc.Inscription(context.Background(), entree)
		appErr := codeErreur(t, err)
		if appErr.Code != apperreur.CodeValidation {
			t.Errorf("code = %q ; attendu VALIDATION", appErr.Code)
		}
	})

	t.Run("mot de passe trop court → VALIDATION", func(t *testing.T) {
		svc := NouveauAuthService(&mockUtilisateurRepo{}, &mockJetonRepo{}, gestionnaireJWTTest())

		entree := entreeValide
		entree.MotDePasse = "court"

		_, err := svc.Inscription(context.Background(), entree)
		appErr := codeErreur(t, err)
		if appErr.Code != apperreur.CodeValidation {
			t.Errorf("code = %q ; attendu VALIDATION", appErr.Code)
		}
	})
}

// utilisateurAvecMotDePasse construit un utilisateur actif dont le hash correspond
// au mot de passe fourni (pour tester Connexion).
func utilisateurAvecMotDePasse(t *testing.T, motDePasse string, actif bool) *models.Utilisateur {
	t.Helper()
	hache, err := auth.HacherMotDePasse(motDePasse)
	if err != nil {
		t.Fatalf("HacherMotDePasse : %v", err)
	}
	return &models.Utilisateur{
		ID:             7,
		UUID:           uuid.NewString(),
		Email:          "membre@exemple.fr",
		MotDePasseHash: hache,
		Nom:            "Durand",
		Prenom:         "Chloé",
		Role:           models.RoleMembre,
		Actif:          actif,
	}
}

// TestConnexion couvre les quatre scénarios clés de l'authentification.
func TestConnexion(t *testing.T) {
	const motDePasse = "MotDePasse123!"

	t.Run("identifiants valides → paire de jetons", func(t *testing.T) {
		u := utilisateurAvecMotDePasse(t, motDePasse, true)
		uRepo := &mockUtilisateurRepo{
			parEmail: func(_ context.Context, _ string) (*models.Utilisateur, error) { return u, nil },
		}
		var jetonEnregistre bool
		jRepo := &mockJetonRepo{
			enregistrer: func(_ context.Context, _ int64, _ string, _ time.Time) error {
				jetonEnregistre = true
				return nil
			},
		}
		svc := NouveauAuthService(uRepo, jRepo, gestionnaireJWTTest())

		resultat, err := svc.Connexion(context.Background(), models.ConnexionEntree{
			Email:      "membre@exemple.fr",
			MotDePasse: motDePasse,
		})
		if err != nil {
			t.Fatalf("Connexion a échoué : %v", err)
		}
		if resultat.Jetons.JetonAcces == "" || resultat.Jetons.JetonRafraichissement == "" {
			t.Error("la paire de jetons devrait être renseignée")
		}
		if resultat.Jetons.TypeJeton != "Bearer" {
			t.Errorf("TypeJeton = %q ; attendu « Bearer »", resultat.Jetons.TypeJeton)
		}
		if !jetonEnregistre {
			t.Error("le refresh token aurait dû être enregistré en base")
		}
	})

	t.Run("mauvais mot de passe → NON_AUTHENTIFIE (401)", func(t *testing.T) {
		u := utilisateurAvecMotDePasse(t, motDePasse, true)
		uRepo := &mockUtilisateurRepo{
			parEmail: func(_ context.Context, _ string) (*models.Utilisateur, error) { return u, nil },
		}
		svc := NouveauAuthService(uRepo, &mockJetonRepo{}, gestionnaireJWTTest())

		_, err := svc.Connexion(context.Background(), models.ConnexionEntree{
			Email:      "membre@exemple.fr",
			MotDePasse: "MauvaisMotDePasse",
		})
		appErr := codeErreur(t, err)
		if appErr.Code != apperreur.CodeNonAuthentifie {
			t.Errorf("code = %q ; attendu NON_AUTHENTIFIE", appErr.Code)
		}
		if appErr.StatutHTTP != http.StatusUnauthorized {
			t.Errorf("StatutHTTP = %d ; attendu 401", appErr.StatutHTTP)
		}
	})

	t.Run("compte inactif → INTERDIT (403)", func(t *testing.T) {
		u := utilisateurAvecMotDePasse(t, motDePasse, false) // inactif
		uRepo := &mockUtilisateurRepo{
			parEmail: func(_ context.Context, _ string) (*models.Utilisateur, error) { return u, nil },
		}
		svc := NouveauAuthService(uRepo, &mockJetonRepo{}, gestionnaireJWTTest())

		// Mot de passe CORRECT : on doit atteindre le contrôle du compte actif.
		_, err := svc.Connexion(context.Background(), models.ConnexionEntree{
			Email:      "membre@exemple.fr",
			MotDePasse: motDePasse,
		})
		appErr := codeErreur(t, err)
		if appErr.Code != apperreur.CodeInterdit {
			t.Errorf("code = %q ; attendu INTERDIT", appErr.Code)
		}
		if appErr.StatutHTTP != http.StatusForbidden {
			t.Errorf("StatutHTTP = %d ; attendu 403", appErr.StatutHTTP)
		}
	})

	t.Run("email inconnu → NON_AUTHENTIFIE (jamais NON_TROUVE)", func(t *testing.T) {
		uRepo := &mockUtilisateurRepo{
			parEmail: func(_ context.Context, _ string) (*models.Utilisateur, error) {
				return nil, apperreur.NonTrouve("Utilisateur introuvable.")
			},
		}
		svc := NouveauAuthService(uRepo, &mockJetonRepo{}, gestionnaireJWTTest())

		_, err := svc.Connexion(context.Background(), models.ConnexionEntree{
			Email:      "inconnu@exemple.fr",
			MotDePasse: motDePasse,
		})
		appErr := codeErreur(t, err)
		// On ne doit JAMAIS révéler qu'un e-mail est inconnu (anti-énumération).
		if appErr.Code != apperreur.CodeNonAuthentifie {
			t.Errorf("code = %q ; attendu NON_AUTHENTIFIE (pas NON_TROUVE)", appErr.Code)
		}
		if appErr.Code == apperreur.CodeNonTrouve {
			t.Error("le code NON_TROUVE fuite l'existence des comptes")
		}
	})
}
