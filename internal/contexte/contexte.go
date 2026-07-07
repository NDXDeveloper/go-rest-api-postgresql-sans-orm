// Package contexte centralise les valeurs que l'on transporte dans le
// context.Context d'une requête HTTP (identifiant de requête, utilisateur
// authentifié).
//
// # Pourquoi un package dédié ?
//
//   - Pour éviter les dépendances circulaires : plusieurs couches (middleware,
//     handler, réponse) ont besoin de lire/écrire ces valeurs. En les isolant
//     dans un petit package de bas niveau, chacun peut l'importer sans cycle.
//
//   - Pour la SÉCURITÉ DES TYPES : on n'utilise jamais une chaîne comme clé de
//     contexte (« request_id »). Une clé de type non exporté (`type cle int`)
//     garantit qu'aucun autre package ne peut, par accident ou malice, écraser
//     notre valeur. C'est la recommandation officielle du package context.
package contexte

import (
	"context"

	"github.com/exemple/api-bibliotheque/internal/models"
)

// cle est un type PRIVÉ utilisé comme clé de contexte. Étant non exporté, il est
// impossible pour un autre package de fabriquer la même clé : nos valeurs sont
// donc protégées contre les collisions.
type cle int

const (
	cleIdentifiantRequete cle = iota
	cleUtilisateur
)

// UtilisateurAuthentifie contient l'identité extraite d'un jeton JWT valide.
// Le middleware d'authentification le place dans le contexte ; les handlers le
// relisent pour savoir « qui » effectue l'action et avec quel rôle.
type UtilisateurAuthentifie struct {
	UUID  string
	Email string
	Role  models.Role
}

// AvecIdentifiantRequete renvoie un contexte enrichi de l'identifiant de requête.
func AvecIdentifiantRequete(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, cleIdentifiantRequete, id)
}

// IdentifiantRequete lit l'identifiant de requête (chaîne vide si absent).
func IdentifiantRequete(ctx context.Context) string {
	if v, ok := ctx.Value(cleIdentifiantRequete).(string); ok {
		return v
	}
	return ""
}

// AvecUtilisateur renvoie un contexte enrichi de l'utilisateur authentifié.
func AvecUtilisateur(ctx context.Context, u *UtilisateurAuthentifie) context.Context {
	return context.WithValue(ctx, cleUtilisateur, u)
}

// Utilisateur lit l'utilisateur authentifié. Le booléen vaut false si la requête
// n'est pas authentifiée (route publique ou jeton absent).
func Utilisateur(ctx context.Context) (*UtilisateurAuthentifie, bool) {
	u, ok := ctx.Value(cleUtilisateur).(*UtilisateurAuthentifie)
	return u, ok
}
