// Package auth regroupe la sécurité de l'authentification : hachage des mots de
// passe (bcrypt) et gestion des jetons (JWT d'accès + refresh tokens).
package auth

import "golang.org/x/crypto/bcrypt"

// coutBcrypt règle le « coût » (facteur de travail) de bcrypt. Chaque incrément
// double le temps de calcul. 12 est un bon compromis en 2025 : assez lent pour
// gêner une attaque par force brute, assez rapide pour ne pas pénaliser un login
// légitime (~100-300 ms). On l'augmentera avec les années et le matériel.
const coutBcrypt = 12

// HacherMotDePasse transforme un mot de passe en clair en un haché bcrypt sûr.
//
// Pourquoi bcrypt (et pas SHA-256 « simple ») ?
//   - bcrypt intègre un SEL aléatoire unique par mot de passe : deux utilisateurs
//     avec le même mot de passe auront des hachés différents (pas d'attaque par
//     table arc-en-ciel) ;
//   - bcrypt est LENT à dessein (facteur de coût), ce qui ralentit fortement les
//     attaques par force brute, contrairement à un hachage cryptographique rapide.
//
// Le sel et le coût sont stockés DANS la chaîne de sortie : on n'a rien à gérer
// nous-mêmes.
func HacherMotDePasse(motDePasseEnClair string) (string, error) {
	hache, err := bcrypt.GenerateFromPassword([]byte(motDePasseEnClair), coutBcrypt)
	if err != nil {
		return "", err
	}
	return string(hache), nil
}

// VerifierMotDePasse compare un mot de passe en clair au haché stocké.
//
// bcrypt.CompareHashAndPassword effectue une comparaison à TEMPS CONSTANT :
// la durée ne dépend pas de l'endroit où les octets diffèrent, ce qui empêche les
// attaques temporelles (« timing attacks »). On ne compare donc JAMAIS deux
// hachés avec un simple « == ».
func VerifierMotDePasse(hacheStocke, motDePasseEnClair string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hacheStocke), []byte(motDePasseEnClair)) == nil
}
