// Package handler contient la couche HTTP : les gestionnaires (« handlers ») qui
// traduisent une requête HTTP en appel de service, puis le résultat en réponse
// JSON. Les handlers NE contiennent NI SQL NI logique métier : ils orchestrent
// le décodage de l'entrée, appellent le service, et écrivent la réponse.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/contexte"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// utilisateurCourant récupère l'utilisateur authentifié placé dans le contexte
// par le middleware d'authentification. Renvoie une erreur 401 si absent (ce qui
// ne devrait pas arriver sur une route correctement protégée).
func utilisateurCourant(r *http.Request) (*contexte.UtilisateurAuthentifie, error) {
	u, ok := contexte.Utilisateur(r.Context())
	if !ok {
		return nil, apperreur.NonAuthentifie("Authentification requise.")
	}
	return u, nil
}

// decoderJSON lit et valide le corps JSON d'une requête dans la structure `dest`.
//
// Trois protections importantes sont appliquées :
//   - DisallowUnknownFields : tout champ non prévu par la structure d'entrée
//     provoque une erreur. C'est un renfort contre le Mass Assignment (le client
//     ne peut pas « glisser » un champ inattendu) et une aide au diagnostic.
//   - un corps trop volumineux (limité en amont par le middleware) donne un 413 clair.
//   - un JSON mal formé donne un 400 explicite, jamais une erreur technique brute.
func decoderJSON(r *http.Request, dest any) error {
	decodeur := json.NewDecoder(r.Body)
	decodeur.DisallowUnknownFields()

	if err := decodeur.Decode(dest); err != nil {
		var erreurTaille *http.MaxBytesError
		var erreurSyntaxe *json.SyntaxError
		var erreurType *json.UnmarshalTypeError

		switch {
		case errors.As(err, &erreurTaille):
			return apperreur.CorpsTropVolumineux("Le corps de la requête est trop volumineux.")
		case errors.As(err, &erreurSyntaxe):
			return apperreur.RequeteInvalide("Le corps de la requête n'est pas un JSON valide.")
		case errors.As(err, &erreurType):
			return apperreur.RequeteInvalide("Un champ a un type incorrect : " + erreurType.Field + ".")
		case errors.Is(err, io.EOF):
			return apperreur.RequeteInvalide("Le corps de la requête est vide.")
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			champ := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return apperreur.RequeteInvalide("Champ non autorisé : " + champ + ".")
		default:
			return apperreur.RequeteInvalide("Corps de requête illisible.")
		}
	}

	// On refuse plusieurs objets JSON à la suite (« {...}{...} ») : un seul attendu.
	if decodeur.More() {
		return apperreur.RequeteInvalide("Le corps ne doit contenir qu'un seul objet JSON.")
	}
	return nil
}

// analyserParametresListe construit un models.ParametresListe SÛR à partir des
// paramètres de requête (?page=&taille=&tri=&ordre=&recherche=&<filtres>).
//
//   - colonnesTriAutorisees mappe un nom logique exposé au client (ex. "titre")
//     vers le nom de colonne SQL réel. Seules ces colonnes peuvent servir au tri :
//     c'est la LISTE BLANCHE qui empêche l'injection SQL via ORDER BY.
//   - colonneParDefaut est la colonne SQL utilisée si aucun tri valide n'est demandé.
//   - filtresAutorises liste les noms de filtres additionnels à extraire tels quels.
func analyserParametresListe(r *http.Request, colonnesTriAutorisees map[string]string, colonneParDefaut string, filtresAutorises ...string) models.ParametresListe {
	q := r.URL.Query()

	params := models.ParametresListe{
		Page:      entierOuDefaut(q.Get("page"), models.PageParDefaut),
		Taille:    entierOuDefaut(q.Get("taille"), models.TailleParDefaut),
		Recherche: strings.TrimSpace(q.Get("recherche")),
		Ordre:     strings.ToUpper(strings.TrimSpace(q.Get("ordre"))),
		Filtres:   make(map[string]string),
	}

	// Tri : on ne retient QUE si le nom demandé figure dans la liste blanche.
	if colonneSQL, autorise := colonnesTriAutorisees[q.Get("tri")]; autorise {
		params.ColonneTri = colonneSQL
	} else {
		params.ColonneTri = colonneParDefaut
	}

	// Filtres additionnels (valeurs brutes, appliquées via des paramètres « ? »
	// dans les repositories, donc sans risque d'injection).
	for _, nom := range filtresAutorises {
		if valeur := strings.TrimSpace(q.Get(nom)); valeur != "" {
			params.Filtres[nom] = valeur
		}
	}

	// Bornes de sécurité (page >= 1, taille <= max...).
	params.Normaliser()
	return params
}

// interditAdmin renvoie l'erreur 403 standard pour une action réservée aux admins.
func interditAdmin() error {
	return apperreur.Interdit("Cette action est réservée aux administrateurs.")
}

// entierOuDefaut convertit une chaîne en int, avec repli sur une valeur par défaut.
func entierOuDefaut(valeur string, defaut int) int {
	if n, err := strconv.Atoi(valeur); err == nil {
		return n
	}
	return defaut
}
