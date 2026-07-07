package auth

import "testing"

// TestHacherEtVerifierMotDePasse vérifie le cycle complet : un mot de passe haché
// puis vérifié doit correspondre.
func TestHacherEtVerifierMotDePasse(t *testing.T) {
	const motDePasse = "MotDePasse123!"

	hache, err := HacherMotDePasse(motDePasse)
	if err != nil {
		t.Fatalf("HacherMotDePasse a échoué : %v", err)
	}
	if hache == "" {
		t.Fatal("le haché ne doit pas être vide")
	}
	if hache == motDePasse {
		t.Fatal("le haché ne doit jamais être égal au mot de passe en clair")
	}
	if !VerifierMotDePasse(hache, motDePasse) {
		t.Error("le bon mot de passe aurait dû être accepté")
	}
}

// TestVerifierMauvaisMotDePasse vérifie qu'un mot de passe erroné est rejeté.
func TestVerifierMauvaisMotDePasse(t *testing.T) {
	hache, err := HacherMotDePasse("MotDePasse123!")
	if err != nil {
		t.Fatalf("HacherMotDePasse a échoué : %v", err)
	}
	if VerifierMotDePasse(hache, "MauvaisMotDePasse") {
		t.Error("un mauvais mot de passe n'aurait pas dû être accepté")
	}
}

// TestHachagesDifferentsGraceAuSel vérifie que deux hachages du MÊME mot de passe
// diffèrent : bcrypt intègre un sel aléatoire unique (protection anti-rainbow-table).
func TestHachagesDifferentsGraceAuSel(t *testing.T) {
	const motDePasse = "MotDePasse123!"

	hache1, err := HacherMotDePasse(motDePasse)
	if err != nil {
		t.Fatalf("premier hachage : %v", err)
	}
	hache2, err := HacherMotDePasse(motDePasse)
	if err != nil {
		t.Fatalf("second hachage : %v", err)
	}

	if hache1 == hache2 {
		t.Error("deux hachages du même mot de passe devraient différer (sel aléatoire)")
	}
	// Les deux hachés, bien que différents, doivent vérifier le même mot de passe.
	if !VerifierMotDePasse(hache1, motDePasse) || !VerifierMotDePasse(hache2, motDePasse) {
		t.Error("les deux hachés devraient tous deux valider le mot de passe d'origine")
	}
}
