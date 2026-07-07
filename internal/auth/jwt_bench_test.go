package auth

import "testing"

// puitsChaine empêche l'élimination du code mesuré par le compilateur.
var puitsChaine string

// BenchmarkGenererJetonAcces mesure le coût de génération d'un jeton d'accès signé
// (HMAC-SHA256 + sérialisation JSON des revendications).
func BenchmarkGenererJetonAcces(b *testing.B) {
	g := NouveauGestionnaireJWT(configJWTTest())
	u := utilisateurTest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jeton, _, err := g.GenererJetonAcces(u)
		if err != nil {
			b.Fatal(err)
		}
		puitsChaine = jeton
	}
}

// BenchmarkVerifierJetonAcces mesure le coût de vérification d'un jeton d'accès
// (analyse + contrôle de la signature et des dates).
func BenchmarkVerifierJetonAcces(b *testing.B) {
	g := NouveauGestionnaireJWT(configJWTTest())
	jeton, _, err := g.GenererJetonAcces(utilisateurTest())
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rev, err := g.VerifierJetonAcces(jeton)
		if err != nil {
			b.Fatal(err)
		}
		puitsChaine = rev.Subject
	}
}
