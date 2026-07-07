package models

import "time"

// PaireDeJetons est renvoyée au client après une connexion ou un rafraîchissement.
//
//   - JetonAcces : le JWT à placer dans l'en-tête « Authorization: Bearer ... »
//     pour accéder aux routes protégées. Courte durée de vie.
//   - JetonRafraichissement : le secret opaque permettant d'obtenir un nouveau
//     jeton d'accès sans se ré-identifier. Longue durée de vie, à conserver
//     précieusement côté client.
type PaireDeJetons struct {
	JetonAcces            string    `json:"jeton_acces"`
	JetonRafraichissement string    `json:"jeton_rafraichissement"`
	TypeJeton             string    `json:"type_jeton"` // toujours "Bearer"
	ExpireLe              time.Time `json:"expire_le"`  // expiration du jeton d'accès
}

// ReponseConnexion accompagne la paire de jetons des informations de profil de
// l'utilisateur connecté (pratique pour le client, qui évite un appel séparé).
type ReponseConnexion struct {
	Utilisateur *Utilisateur  `json:"utilisateur"`
	Jetons      PaireDeJetons `json:"jetons"`
}
