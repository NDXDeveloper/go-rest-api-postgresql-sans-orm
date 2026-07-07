package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/config"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Revendications (« claims ») transportées dans le jeton d'accès JWT.
//
// On y met le strict nécessaire à l'autorisation : l'identifiant public (dans le
// « Subject » standard), l'e-mail et le rôle. On n'y met JAMAIS d'information
// sensible (mot de passe...) : un JWT est signé mais PAS chiffré, son contenu est
// lisible par quiconque le possède.
type Revendications struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

// GestionnaireJWT produit et vérifie les jetons. Toutes ses dépendances (secret,
// durées...) sont injectées : aucune variable globale.
type GestionnaireJWT struct {
	secret                []byte
	emetteur              string
	dureeAcces            time.Duration
	dureeRafraichissement time.Duration
}

// NouveauGestionnaireJWT construit le gestionnaire à partir de la configuration.
func NouveauGestionnaireJWT(cfg config.JWT) *GestionnaireJWT {
	return &GestionnaireJWT{
		secret:                []byte(cfg.Secret),
		emetteur:              cfg.Emetteur,
		dureeAcces:            cfg.DureeAcces,
		dureeRafraichissement: cfg.DureeRafraichissement,
	}
}

// DureeRafraichissement expose la durée de vie d'un refresh token (utile au
// service d'authentification pour calculer l'expiration à stocker).
func (g *GestionnaireJWT) DureeRafraichissement() time.Duration {
	return g.dureeRafraichissement
}

// GenererJetonAcces crée un JWT signé (HMAC-SHA256) pour l'utilisateur donné et
// renvoie le jeton ainsi que sa date d'expiration.
//
// Le jeton d'accès est VOLONTAIREMENT à courte durée de vie (quelques minutes) :
// s'il est volé, la fenêtre d'exploitation reste réduite. Pour rester connecté
// sans se ré-identifier, le client utilise le refresh token (longue durée).
func (g *GestionnaireJWT) GenererJetonAcces(u *models.Utilisateur) (string, time.Time, error) {
	maintenant := time.Now()
	expiration := maintenant.Add(g.dureeAcces)

	revendications := Revendications{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.emetteur,
			Subject:   u.UUID, // identifiant public de l'utilisateur
			IssuedAt:  jwt.NewNumericDate(maintenant),
			ExpiresAt: jwt.NewNumericDate(expiration),
			ID:        uuid.NewString(), // identifiant unique du jeton (jti)
		},
		Email: u.Email,
		Role:  string(u.Role),
	}

	jeton := jwt.NewWithClaims(jwt.SigningMethodHS256, revendications)
	signe, err := jeton.SignedString(g.secret)
	if err != nil {
		return "", time.Time{}, apperreur.Interne("signature du jeton d'accès").AvecCause(err)
	}
	return signe, expiration, nil
}

// VerifierJetonAcces valide la signature et les dates d'un jeton, et renvoie ses
// revendications. Toute erreur donne un 401 (non authentifié).
//
// POINT DE SÉCURITÉ CRUCIAL : on impose l'algorithme attendu (HS256). Sans ce
// contrôle, un attaquant pourrait forger un jeton avec « alg: none » (aucune
// signature) ou tenter une confusion d'algorithme. jwt.WithValidMethods verrouille
// ce vecteur d'attaque classique.
func (g *GestionnaireJWT) VerifierJetonAcces(jetonSigne string) (*Revendications, error) {
	revendications := &Revendications{}

	jeton, err := jwt.ParseWithClaims(jetonSigne, revendications,
		func(t *jwt.Token) (any, error) {
			// Double vérification de l'algorithme (défense en profondeur).
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("algorithme de signature inattendu : %v", t.Header["alg"])
			}
			return g.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(g.emetteur),
	)
	if err != nil || !jeton.Valid {
		return nil, apperreur.NonAuthentifie("Jeton d'accès invalide ou expiré.")
	}
	return revendications, nil
}

// GenererJetonRafraichissement crée un refresh token ALÉATOIRE (non deviné) et
// renvoie : le jeton EN CLAIR (à donner au client), son HACHÉ (à stocker en base)
// et sa date d'expiration.
//
// Un refresh token n'est pas un JWT : c'est un simple secret opaque tiré au sort
// avec crypto/rand (générateur cryptographiquement sûr). On ne stocke que son
// haché, comme pour un mot de passe.
func (g *GestionnaireJWT) GenererJetonRafraichissement() (clair string, hache string, expiration time.Time, err error) {
	octets := make([]byte, 32) // 256 bits d'entropie
	if _, err = rand.Read(octets); err != nil {
		return "", "", time.Time{}, apperreur.Interne("génération du jeton de rafraîchissement").AvecCause(err)
	}
	clair = base64.RawURLEncoding.EncodeToString(octets)
	hache = HacherJeton(clair)
	expiration = time.Now().Add(g.dureeRafraichissement)
	return clair, hache, expiration, nil
}

// HacherJeton calcule le SHA-256 (hexadécimal) d'un jeton. Pour un secret à haute
// entropie tiré au hasard, SHA-256 suffit (pas besoin de bcrypt, qui sert à
// ralentir les attaques sur des secrets FAIBLES comme les mots de passe humains).
func HacherJeton(jeton string) string {
	somme := sha256.Sum256([]byte(jeton))
	return hex.EncodeToString(somme[:])
}
