// Package reponse fournit des helpers pour écrire des réponses HTTP JSON
// homogènes dans toute l'API.
//
// # Une enveloppe unique pour toutes les réponses
//
// Chaque réponse — succès comme erreur — respecte la même structure :
//
//	Succès :
//	  { "succes": true, "donnees": { ... }, "meta": { ... } }
//
//	Erreur :
//	  { "succes": false, "erreur": { "code": "...", "message": "...", "details": { ... } } }
//
// Un format homogène simplifie énormément la vie des clients : ils testent
// toujours le champ `succes`, puis lisent `donnees` ou `erreur`.
//
// # Sécurité : aucune fuite technique
//
// Les erreurs sont toujours converties via apperreur.Depuis, qui remplace toute
// erreur inconnue par un message 500 générique. La cause technique (requête SQL,
// etc.) est journalisée côté serveur mais JAMAIS renvoyée au client.
package reponse

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/contexte"
)

// MetaPagination accompagne les listes paginées pour informer le client de sa
// position dans l'ensemble des résultats.
type MetaPagination struct {
	Page          int `json:"page"`
	TailleParPage int `json:"taille_par_page"`
	TotalElements int `json:"total_elements"`
	TotalPages    int `json:"total_pages"`
}

// enveloppe est la structure racine de toute réponse. Les champs `omitempty`
// disparaissent du JSON quand ils sont vides, ce qui donne des réponses propres.
type enveloppe struct {
	Succes  bool            `json:"succes"`
	Donnees any             `json:"donnees,omitempty"`
	Meta    *MetaPagination `json:"meta,omitempty"`
	Erreur  *corpsErreur    `json:"erreur,omitempty"`
}

// corpsErreur est la représentation JSON d'une erreur exposée au client.
type corpsErreur struct {
	Code    apperreur.Code    `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// ecrireJSON sérialise `corps` en JSON et l'envoie avec le code de statut donné.
//
// Bonnes pratiques appliquées :
//   - on fixe l'en-tête Content-Type AVANT d'écrire le corps ;
//   - on charge le charset UTF-8 explicitement (API JSON UTF-8) ;
//   - on écrit le WriteHeader une seule fois, après les en-têtes.
func ecrireJSON(w http.ResponseWriter, statut int, corps any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)

	// Si l'encodage échoue (rare : type non sérialisable), on ne peut plus
	// changer le code de statut déjà écrit. On se contente de ne rien renvoyer
	// de plus ; l'erreur serait de toute façon un bug de développement.
	_ = json.NewEncoder(w).Encode(corps)
}

// Succes écrit une réponse de succès (sans pagination).
//
//	reponse.Succes(w, http.StatusOK, livre)
func Succes(w http.ResponseWriter, statut int, donnees any) {
	ecrireJSON(w, statut, enveloppe{Succes: true, Donnees: donnees})
}

// SuccesPagine écrit une réponse de succès accompagnée des métadonnées de
// pagination. `total` est le nombre TOTAL d'éléments (toutes pages confondues).
func SuccesPagine(w http.ResponseWriter, statut int, donnees any, page, taille, total int) {
	totalPages := 0
	if taille > 0 {
		totalPages = (total + taille - 1) / taille // division entière arrondie au supérieur
	}
	ecrireJSON(w, statut, enveloppe{
		Succes:  true,
		Donnees: donnees,
		Meta: &MetaPagination{
			Page:          page,
			TailleParPage: taille,
			TotalElements: total,
			TotalPages:    totalPages,
		},
	})
}

// SansContenu écrit une réponse 204 No Content (typiquement après un DELETE).
// Aucun corps n'est envoyé, conformément à la sémantique HTTP.
func SansContenu(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Erreur convertit n'importe quelle erreur en réponse JSON homogène.
//
// C'est le point de passage OBLIGATOIRE pour toute erreur renvoyée au client :
//   - une *apperreur.Erreur fournit son code métier, son message sûr et son
//     statut HTTP ;
//   - toute autre erreur est transformée en 500 générique (aucune fuite).
//
// Le logger sert à tracer côté serveur les erreurs internes (5xx) avec leur
// cause technique complète et l'identifiant de requête, sans jamais l'exposer.
func Erreur(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	appErr := apperreur.Depuis(err)

	// Les erreurs serveur (5xx) sont anormales : on les journalise en niveau
	// Error avec toute la cause technique pour pouvoir enquêter. Les erreurs
	// client (4xx) sont attendues (mauvaise saisie...) : on ne les journalise
	// qu'en Debug pour ne pas polluer les logs.
	idRequete := contexte.IdentifiantRequete(r.Context())
	if appErr.StatutHTTP >= 500 {
		logger.ErrorContext(r.Context(), "erreur interne",
			slog.String("identifiant_requete", idRequete),
			slog.String("methode", r.Method),
			slog.String("chemin", r.URL.Path),
			slog.String("code", string(appErr.Code)),
			slog.Any("cause", err), // cause complète, uniquement dans les logs
		)
	} else {
		logger.DebugContext(r.Context(), "erreur client",
			slog.String("identifiant_requete", idRequete),
			slog.String("code", string(appErr.Code)),
			slog.String("message", appErr.Message),
		)
	}

	ecrireJSON(w, appErr.StatutHTTP, enveloppe{
		Succes: false,
		Erreur: &corpsErreur{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
}
