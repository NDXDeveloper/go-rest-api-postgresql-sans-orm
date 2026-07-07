package validation

import "testing"

// « puits » (sink) au niveau du package : en y stockant le résultat, on empêche
// le compilateur d'éliminer l'appel mesuré (« dead code elimination »), ce qui
// fausserait le benchmark.
var puitsBool bool

// BenchmarkEstISBN13Valide mesure le coût de la validation d'un ISBN-13 (nettoyage
// des séparateurs + calcul de la clé de contrôle).
func BenchmarkEstISBN13Valide(b *testing.B) {
	for i := 0; i < b.N; i++ {
		puitsBool = EstISBN13Valide("978-2-01-000000-3")
	}
}

// BenchmarkEstEmailValide mesure le coût de la validation d'une adresse e-mail
// (analyse via net/mail).
func BenchmarkEstEmailValide(b *testing.B) {
	for i := 0; i < b.N; i++ {
		puitsBool = EstEmailValide("jean.dupont@exemple.fr")
	}
}
