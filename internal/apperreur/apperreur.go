// Package apperreur définit le type d'erreur applicative utilisé dans tout le
// projet, ainsi que les constructeurs associés.
//
// # Pourquoi un type d'erreur dédié ?
//
// En Go, une erreur est une simple valeur qui implémente l'interface `error`.
// C'est puissant mais insuffisant pour une API REST : quand une couche basse
// (par exemple un repository) échoue, la couche haute (le handler HTTP) doit
// savoir QUEL code HTTP renvoyer (404 ? 409 ? 500 ?) et QUEL message afficher
// à l'utilisateur SANS jamais divulguer de détail technique (fuite SQL, chemin
// de fichier, etc.).
//
// On introduit donc un type `Erreur` qui transporte trois informations :
//   - un code métier stable (utile pour les clients de l'API) ;
//   - un message en français destiné à l'utilisateur final (jamais de SQL) ;
//   - le code HTTP correspondant.
//
// L'erreur technique d'origine (la « cause ») est conservée en interne pour la
// journalisation, mais n'est JAMAIS renvoyée au client. C'est une bonne
// pratique de sécurité : on ne veut pas qu'un attaquant apprenne la structure
// de la base à partir d'un message d'erreur.
package apperreur

import (
	"errors"
	"fmt"
	"net/http"
)

// Code représente un code d'erreur métier stable et lisible par une machine.
//
// Contrairement au code HTTP (qui est numérique et partagé par de nombreuses
// causes), ce code permet à un client de l'API de réagir précisément :
// par exemple, distinguer « email déjà utilisé » (CONFLIT) d'une autre erreur 409.
type Code string

// Ensemble des codes métier reconnus par l'application.
//
// Bonne pratique : on centralise ces constantes pour éviter les fautes de frappe
// et garantir que la documentation (API.md, openapi.yaml) reste synchronisée
// avec le code.
const (
	// CodeRequeteInvalide : le corps ou les paramètres de la requête sont
	// mal formés (JSON illisible, type incorrect...). → HTTP 400.
	CodeRequeteInvalide Code = "REQUETE_INVALIDE"

	// CodeValidation : la requête est bien formée mais viole une règle métier
	// (email invalide, champ manquant...). → HTTP 422.
	CodeValidation Code = "VALIDATION"

	// CodeNonAuthentifie : aucune identité valide n'a été fournie. → HTTP 401.
	CodeNonAuthentifie Code = "NON_AUTHENTIFIE"

	// CodeInterdit : l'utilisateur est authentifié mais n'a pas les droits
	// suffisants (mauvais rôle). → HTTP 403.
	CodeInterdit Code = "INTERDIT"

	// CodeNonTrouve : la ressource demandée n'existe pas. → HTTP 404.
	CodeNonTrouve Code = "NON_TROUVE"

	// CodeConflit : la requête entre en conflit avec l'état actuel
	// (doublon d'une clé unique, règle métier bloquante...). → HTTP 409.
	CodeConflit Code = "CONFLIT"

	// CodeTropDeRequetes : le limiteur de débit a été déclenché. → HTTP 429.
	CodeTropDeRequetes Code = "TROP_DE_REQUETES"

	// CodeInterne : une erreur inattendue s'est produite côté serveur. → HTTP 500.
	// Le détail technique est journalisé mais jamais exposé.
	CodeInterne Code = "ERREUR_INTERNE"

	// CodeServiceIndisponible : le service ne peut pas répondre pour l'instant
	// (dépendance en panne, ex. base de données injoignable). → HTTP 503.
	CodeServiceIndisponible Code = "SERVICE_INDISPONIBLE"
)

// Erreur est le type d'erreur applicative transporté à travers les couches.
//
// Il implémente l'interface `error` (méthode Error) et l'interface d'unwrapping
// (méthode Unwrap), ce qui le rend compatible avec `errors.Is` et `errors.As`.
type Erreur struct {
	// Code est le code métier stable (voir les constantes ci-dessus).
	Code Code

	// Message est le texte en français destiné à l'utilisateur final.
	// Il ne doit JAMAIS contenir de détail technique (requête SQL, nom de table...).
	Message string

	// StatutHTTP est le code de statut HTTP à renvoyer au client.
	StatutHTTP int

	// Details contient, pour les erreurs de validation, la liste des problèmes
	// champ par champ (clé = nom du champ, valeur = explication). Vide sinon.
	Details map[string]string

	// cause est l'erreur technique d'origine. Elle est PRIVÉE : on l'utilise
	// uniquement pour la journalisation serveur, jamais dans la réponse HTTP.
	cause error
}

// Error implémente l'interface `error`.
//
// On inclut la cause si elle existe, car cette chaîne n'est utilisée que pour
// les logs internes (le handler, lui, n'expose que le champ Message).
func (e *Erreur) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s (%s): %v", e.Message, e.Code, e.cause)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// Unwrap permet à `errors.Is` et `errors.As` de remonter jusqu'à la cause.
//
// Exemple : `errors.Is(err, sql.ErrNoRows)` fonctionnera même si l'erreur a été
// enveloppée dans un *Erreur, à condition que la cause soit sql.ErrNoRows.
func (e *Erreur) Unwrap() error {
	return e.cause
}

// AvecCause attache (ou remplace) l'erreur technique d'origine et renvoie
// l'erreur pour permettre le chaînage. Utile dans les repositories :
//
//	return apperreur.Interne("échec de lecture").AvecCause(err)
func (e *Erreur) AvecCause(cause error) *Erreur {
	e.cause = cause
	return e
}

// AvecDetails attache la liste des erreurs de validation champ par champ.
func (e *Erreur) AvecDetails(details map[string]string) *Erreur {
	e.Details = details
	return e
}

// --- Constructeurs ---------------------------------------------------------
//
// On expose un constructeur par famille d'erreur. Cela rend le code appelant
// très lisible (« apperreur.NonTrouve("livre introuvable") ») et garantit que
// le code HTTP est toujours cohérent avec le code métier.

// RequeteInvalide construit une erreur 400 (corps/paramètres mal formés).
func RequeteInvalide(message string) *Erreur {
	return &Erreur{Code: CodeRequeteInvalide, Message: message, StatutHTTP: http.StatusBadRequest}
}

// Validation construit une erreur 422 accompagnée du détail des champs fautifs.
func Validation(message string, details map[string]string) *Erreur {
	return &Erreur{
		Code:       CodeValidation,
		Message:    message,
		StatutHTTP: http.StatusUnprocessableEntity,
		Details:    details,
	}
}

// NonAuthentifie construit une erreur 401 (identité absente ou invalide).
func NonAuthentifie(message string) *Erreur {
	return &Erreur{Code: CodeNonAuthentifie, Message: message, StatutHTTP: http.StatusUnauthorized}
}

// Interdit construit une erreur 403 (droits insuffisants).
func Interdit(message string) *Erreur {
	return &Erreur{Code: CodeInterdit, Message: message, StatutHTTP: http.StatusForbidden}
}

// NonTrouve construit une erreur 404 (ressource inexistante).
func NonTrouve(message string) *Erreur {
	return &Erreur{Code: CodeNonTrouve, Message: message, StatutHTTP: http.StatusNotFound}
}

// Conflit construit une erreur 409 (doublon, règle métier bloquante...).
func Conflit(message string) *Erreur {
	return &Erreur{Code: CodeConflit, Message: message, StatutHTTP: http.StatusConflict}
}

// TropDeRequetes construit une erreur 429 (limiteur de débit).
func TropDeRequetes(message string) *Erreur {
	return &Erreur{Code: CodeTropDeRequetes, Message: message, StatutHTTP: http.StatusTooManyRequests}
}

// CorpsTropVolumineux construit une erreur 413 (Payload Too Large), renvoyée
// quand le corps de la requête dépasse la taille maximale autorisée.
func CorpsTropVolumineux(message string) *Erreur {
	return &Erreur{Code: CodeRequeteInvalide, Message: message, StatutHTTP: http.StatusRequestEntityTooLarge}
}

// Interne construit une erreur 500 générique.
//
// Piège classique à éviter : NE PAS mettre l'erreur technique dans le champ
// Message. On journalise la cause (via AvecCause) mais on renvoie au client un
// message neutre.
func Interne(message string) *Erreur {
	if message == "" {
		message = "Une erreur interne est survenue. Veuillez réessayer plus tard."
	}
	return &Erreur{Code: CodeInterne, Message: message, StatutHTTP: http.StatusInternalServerError}
}

// EstCode indique si l'erreur (éventuellement enveloppée) est une *Erreur
// applicative portant le code donné. Pratique pour réagir à un cas précis, par
// exemple transformer un « NON_TROUVE » en « NON_AUTHENTIFIE » lors du login
// (afin de ne pas révéler si un e-mail existe).
func EstCode(err error, code Code) bool {
	var appErr *Erreur
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// ServiceIndisponible construit une erreur 503 (dépendance en panne).
func ServiceIndisponible(message string) *Erreur {
	return &Erreur{Code: CodeServiceIndisponible, Message: message, StatutHTTP: http.StatusServiceUnavailable}
}

// Depuis convertit n'importe quelle erreur en *Erreur applicative.
//
//   - Si l'erreur est déjà un *Erreur, on la renvoie telle quelle.
//   - Sinon, on l'enveloppe dans une erreur 500 générique en conservant la cause
//     pour la journalisation.
//
// Cette fonction est appelée par le helper de réponse HTTP : elle garantit
// qu'AUCUNE erreur brute (et donc aucune fuite technique) ne parvient au client.
func Depuis(err error) *Erreur {
	if err == nil {
		return nil
	}
	var appErr *Erreur
	if errors.As(err, &appErr) {
		return appErr
	}
	return Interne("").AvecCause(err)
}
