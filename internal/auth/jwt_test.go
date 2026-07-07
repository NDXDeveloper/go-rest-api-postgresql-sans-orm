package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/exemple/api-bibliotheque/internal/config"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// configJWTTest renvoie une configuration JWT valide pour les tests (secret d'au
// moins 32 caractères, comme l'exige la configuration de production).
func configJWTTest() config.JWT {
	return config.JWT{
		Secret:                "secret-de-test-suffisamment-long-0123456789",
		Emetteur:              "api-bibliotheque-test",
		DureeAcces:            15 * time.Minute,
		DureeRafraichissement: 24 * time.Hour,
	}
}

// utilisateurTest renvoie un utilisateur factice pour la génération de jetons.
func utilisateurTest() *models.Utilisateur {
	return &models.Utilisateur{
		ID:    42,
		UUID:  "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
		Email: "membre@exemple.fr",
		Role:  models.RoleMembre,
	}
}

// TestGenererEtVerifierJetonAcces vérifie le cycle complet d'un jeton d'accès :
// génération puis vérification, avec restitution fidèle du Subject, du Role et de
// l'Email.
func TestGenererEtVerifierJetonAcces(t *testing.T) {
	g := NouveauGestionnaireJWT(configJWTTest())
	u := utilisateurTest()

	jeton, expiration, err := g.GenererJetonAcces(u)
	if err != nil {
		t.Fatalf("GenererJetonAcces a échoué : %v", err)
	}
	if jeton == "" {
		t.Fatal("le jeton généré ne doit pas être vide")
	}
	if !expiration.After(time.Now()) {
		t.Error("l'expiration du jeton devrait être dans le futur")
	}

	rev, err := g.VerifierJetonAcces(jeton)
	if err != nil {
		t.Fatalf("VerifierJetonAcces a échoué sur un jeton valide : %v", err)
	}
	if rev.Subject != u.UUID {
		t.Errorf("Subject = %q ; attendu %q", rev.Subject, u.UUID)
	}
	if rev.Email != u.Email {
		t.Errorf("Email = %q ; attendu %q", rev.Email, u.Email)
	}
	if rev.Role != string(u.Role) {
		t.Errorf("Role = %q ; attendu %q", rev.Role, u.Role)
	}
}

// TestVerifierJetonFalsifie vérifie qu'un jeton altéré est rejeté.
func TestVerifierJetonFalsifie(t *testing.T) {
	g := NouveauGestionnaireJWT(configJWTTest())

	jeton, _, err := g.GenererJetonAcces(utilisateurTest())
	if err != nil {
		t.Fatalf("GenererJetonAcces a échoué : %v", err)
	}

	// On corrompt le PREMIER caractère de la signature (3ᵉ segment). Contrairement
	// au dernier caractère d'un segment base64url (qui peut ne porter que des bits
	// de bourrage non significatifs), le premier caractère porte toujours des bits
	// significatifs : sa modification change à coup sûr la signature décodée. On le
	// remplace par un caractère base64url garanti différent, pour un test déterministe.
	parties := strings.Split(jeton, ".")
	if len(parties) != 3 {
		t.Fatalf("un JWT doit comporter 3 segments, obtenu %d", len(parties))
	}
	signature := []byte(parties[2])
	if signature[0] == 'A' {
		signature[0] = 'B'
	} else {
		signature[0] = 'A'
	}
	parties[2] = string(signature)
	falsifie := strings.Join(parties, ".")

	if _, err := g.VerifierJetonAcces(falsifie); err == nil {
		t.Error("un jeton falsifié aurait dû être rejeté")
	}

	// Un charabia total doit aussi être rejeté.
	if _, err := g.VerifierJetonAcces("pas.un.jeton"); err == nil {
		t.Error("une chaîne non JWT aurait dû être rejetée")
	}
}

// TestVerifierJetonAutreSecret vérifie qu'un jeton signé avec un AUTRE secret est
// rejeté : la signature ne correspond plus (attaque par jeton forgé).
func TestVerifierJetonAutreSecret(t *testing.T) {
	g := NouveauGestionnaireJWT(configJWTTest())

	autreConfig := configJWTTest()
	autreConfig.Secret = "un-tout-autre-secret-tout-aussi-long-987654321"
	gAutre := NouveauGestionnaireJWT(autreConfig)

	jetonAutre, _, err := gAutre.GenererJetonAcces(utilisateurTest())
	if err != nil {
		t.Fatalf("GenererJetonAcces (autre secret) a échoué : %v", err)
	}

	if _, err := g.VerifierJetonAcces(jetonAutre); err == nil {
		t.Error("un jeton signé avec un autre secret aurait dû être rejeté")
	}
}

// TestGenererJetonRafraichissement vérifie qu'un refresh token est un secret non
// vide, distinct de son haché, et que HacherJeton est déterministe.
func TestGenererJetonRafraichissement(t *testing.T) {
	g := NouveauGestionnaireJWT(configJWTTest())

	clair, hache, expiration, err := g.GenererJetonRafraichissement()
	if err != nil {
		t.Fatalf("GenererJetonRafraichissement a échoué : %v", err)
	}
	if clair == "" {
		t.Error("le jeton en clair ne doit pas être vide")
	}
	if hache == "" {
		t.Error("le haché ne doit pas être vide")
	}
	if clair == hache {
		t.Error("le clair et le haché ne doivent jamais être identiques")
	}
	if !expiration.After(time.Now()) {
		t.Error("l'expiration du refresh token devrait être dans le futur")
	}

	// HacherJeton doit être déterministe : même entrée => même sortie.
	if HacherJeton(clair) != hache {
		t.Error("HacherJeton(clair) devrait reproduire le haché renvoyé")
	}
	premierHache := HacherJeton("abc")
	secondHache := HacherJeton("abc")
	if premierHache != secondHache {
		t.Error("HacherJeton devrait être déterministe")
	}
	// Deux jetons successifs doivent différer (tirage aléatoire).
	clair2, _, _, err := g.GenererJetonRafraichissement()
	if err != nil {
		t.Fatalf("second GenererJetonRafraichissement : %v", err)
	}
	if clair == clair2 {
		t.Error("deux refresh tokens successifs devraient différer")
	}
}
