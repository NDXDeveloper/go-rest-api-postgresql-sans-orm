// Package models définit les structures de données (« entités ») manipulées par
// l'application, ainsi que les structures d'entrée (« DTO ») reçues dans le
// corps des requêtes HTTP.
//
// # Deux familles de structures, et pourquoi
//
//  1. Les ENTITÉS (ex. Utilisateur) reflètent l'état d'une ligne en base. Elles
//     sont produites par les repositories et sérialisées en JSON pour le client.
//
//  2. Les ENTRÉES (ex. InscriptionEntree) décrivent EXACTEMENT ce qu'un client
//     a le droit d'envoyer. On ne désérialise JAMAIS le JSON client directement
//     dans une entité : cela ouvrirait la faille du « Mass Assignment » (un
//     client malveillant pourrait forcer role="admin" ou actif=true).
//     En séparant les deux, le client ne peut modifier que les champs prévus.
//
// Toutes les structures utilisent des balises `json` en français pour rester
// cohérentes avec la vocation francophone du projet.
package models

import "time"

// Role énumère les rôles applicatifs. On les utilise pour l'autorisation
// (voir le middleware d'autorisation par rôle).
type Role string

const (
	// RoleAdmin : accès complet (gestion des utilisateurs, du catalogue...).
	RoleAdmin Role = "admin"
	// RoleBibliothecaire : gère le catalogue et les emprunts.
	RoleBibliothecaire Role = "bibliothecaire"
	// RoleMembre : rôle par défaut, peut emprunter et consulter.
	RoleMembre Role = "membre"
)

// EstValide indique si le rôle fait partie des valeurs autorisées.
// Utile lors de la validation d'une entrée où un admin fixe un rôle.
func (r Role) EstValide() bool {
	switch r {
	case RoleAdmin, RoleBibliothecaire, RoleMembre:
		return true
	default:
		return false
	}
}

// Utilisateur représente un compte en base de données.
//
// Remarques importantes sur les balises JSON :
//   - ID (clé primaire technique AUTO_INCREMENT) porte `json:"-"` : il n'est
//     JAMAIS exposé. Exposer une clé séquentielle permettrait l'énumération des
//     ressources (« IDOR »). On expose à la place l'UUID, non devinable.
//   - MotDePasseHash porte `json:"-"` : un secret ne doit jamais quitter le
//     serveur, même haché.
type Utilisateur struct {
	ID             int64      `json:"-"`                     // clé interne, jamais exposée
	UUID           string     `json:"id"`                    // identifiant public (UUID v4)
	Email          string     `json:"email"`                 //
	MotDePasseHash string     `json:"-"`                     // haché avec bcrypt, jamais sérialisé
	Nom            string     `json:"nom"`                   //
	Prenom         string     `json:"prenom"`                //
	Role           Role       `json:"role"`                  //
	Actif          bool       `json:"actif"`                 // un compte inactif ne peut pas se connecter
	CreeLe         time.Time  `json:"cree_le"`               //
	ModifieLe      time.Time  `json:"modifie_le"`            //
	SupprimeLe     *time.Time `json:"supprime_le,omitempty"` // non nul => supprimé logiquement
}

// InscriptionEntree est le corps attendu pour l'inscription publique.
//
// On ne prévoit volontairement PAS de champ « role » : tout nouvel inscrit
// devient « membre ». C'est la protection anti-Mass-Assignment en action.
type InscriptionEntree struct {
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
	Nom        string `json:"nom"`
	Prenom     string `json:"prenom"`
}

// ConnexionEntree est le corps attendu pour l'authentification.
type ConnexionEntree struct {
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
}

// RafraichissementEntree est le corps attendu pour obtenir un nouveau jeton
// d'accès à partir d'un refresh token.
type RafraichissementEntree struct {
	JetonRafraichissement string `json:"jeton_rafraichissement"`
}

// CreerUtilisateurEntree est réservé aux administrateurs : contrairement à
// l'inscription publique, il permet de fixer explicitement le rôle.
type CreerUtilisateurEntree struct {
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
	Nom        string `json:"nom"`
	Prenom     string `json:"prenom"`
	Role       Role   `json:"role"`
}

// MettreAJourUtilisateurEntree correspond à une mise à jour COMPLÈTE (PUT) des
// champs modifiables par l'utilisateur lui-même.
type MettreAJourUtilisateurEntree struct {
	Nom    string `json:"nom"`
	Prenom string `json:"prenom"`
}

// ModifierUtilisateurEntree correspond à une mise à jour PARTIELLE (PATCH).
//
// Les champs sont des POINTEURS afin de distinguer trois cas :
//   - champ absent du JSON      => pointeur nil        => on ne touche à rien ;
//   - champ présent avec valeur => pointeur non nil    => on applique la valeur.
//
// C'est la manière idiomatique en Go de gérer un PATCH partiel sans ORM.
type ModifierUtilisateurEntree struct {
	Nom    *string `json:"nom"`
	Prenom *string `json:"prenom"`
	Actif  *bool   `json:"actif"` // réservé aux administrateurs
	Role   *Role   `json:"role"`  // réservé aux administrateurs
}
