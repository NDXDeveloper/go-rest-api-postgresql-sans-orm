package apperreur

import (
	"errors"
	"net/http"
	"testing"
)

// TestConstructeurs vérifie que chaque constructeur produit le bon code métier ET
// le bon code HTTP. C'est la garantie de cohérence sur laquelle repose toute la
// couche de réponse HTTP.
func TestConstructeurs(t *testing.T) {
	cas := []struct {
		nom           string
		err           *Erreur
		codeAttendu   Code
		statutAttendu int
	}{
		{"RequeteInvalide", RequeteInvalide("m"), CodeRequeteInvalide, http.StatusBadRequest},
		{"Validation", Validation("m", nil), CodeValidation, http.StatusUnprocessableEntity},
		{"NonAuthentifie", NonAuthentifie("m"), CodeNonAuthentifie, http.StatusUnauthorized},
		{"Interdit", Interdit("m"), CodeInterdit, http.StatusForbidden},
		{"NonTrouve", NonTrouve("m"), CodeNonTrouve, http.StatusNotFound},
		{"Conflit", Conflit("m"), CodeConflit, http.StatusConflict},
		{"TropDeRequetes", TropDeRequetes("m"), CodeTropDeRequetes, http.StatusTooManyRequests},
		{"CorpsTropVolumineux", CorpsTropVolumineux("m"), CodeRequeteInvalide, http.StatusRequestEntityTooLarge},
		{"Interne", Interne("m"), CodeInterne, http.StatusInternalServerError},
		{"ServiceIndisponible", ServiceIndisponible("m"), CodeServiceIndisponible, http.StatusServiceUnavailable},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if c.err.Code != c.codeAttendu {
				t.Errorf("Code = %q ; attendu %q", c.err.Code, c.codeAttendu)
			}
			if c.err.StatutHTTP != c.statutAttendu {
				t.Errorf("StatutHTTP = %d ; attendu %d", c.err.StatutHTTP, c.statutAttendu)
			}
		})
	}
}

// TestInterneMessageParDefaut vérifie qu'un message vide est remplacé par un texte
// générique (on n'expose jamais de vide au client).
func TestInterneMessageParDefaut(t *testing.T) {
	err := Interne("")
	if err.Message == "" {
		t.Fatal("Interne(\"\") aurait dû fournir un message par défaut")
	}
}

// TestDepuisNil vérifie que Depuis(nil) renvoie nil (aucune erreur).
func TestDepuisNil(t *testing.T) {
	if Depuis(nil) != nil {
		t.Fatal("Depuis(nil) aurait dû renvoyer nil")
	}
}

// TestDepuisErreurArbitraire vérifie qu'une erreur inconnue devient un 500 (aucune
// fuite technique vers le client), tout en conservant la cause pour les logs.
func TestDepuisErreurArbitraire(t *testing.T) {
	cause := errors.New("détail technique interne")
	appErr := Depuis(cause)
	if appErr == nil {
		t.Fatal("Depuis(err) ne doit pas renvoyer nil pour une erreur non nil")
	}
	if appErr.Code != CodeInterne {
		t.Errorf("Code = %q ; attendu %q", appErr.Code, CodeInterne)
	}
	if appErr.StatutHTTP != http.StatusInternalServerError {
		t.Errorf("StatutHTTP = %d ; attendu 500", appErr.StatutHTTP)
	}
	// La cause doit rester accessible via errors.Is (pour la journalisation).
	if !errors.Is(appErr, cause) {
		t.Error("la cause d'origine aurait dû être conservée (Unwrap)")
	}
}

// TestDepuisErreurApplicative vérifie qu'une *Erreur déjà applicative est renvoyée
// telle quelle (même pointeur, pas de ré-enveloppement).
func TestDepuisErreurApplicative(t *testing.T) {
	origine := NonTrouve("Livre introuvable.")
	if got := Depuis(origine); got != origine {
		t.Errorf("Depuis(*Erreur) aurait dû renvoyer le même pointeur")
	}
}

// TestEstCode vérifie la détection d'un code métier, y compris à travers un
// enveloppement (errors.As).
func TestEstCode(t *testing.T) {
	err := NonTrouve("introuvable")
	if !EstCode(err, CodeNonTrouve) {
		t.Error("EstCode aurait dû reconnaître CodeNonTrouve")
	}
	if EstCode(err, CodeConflit) {
		t.Error("EstCode ne devait pas confondre NonTrouve et Conflit")
	}
	if EstCode(errors.New("erreur brute"), CodeInterne) {
		t.Error("EstCode ne doit reconnaître que les *Erreur applicatives")
	}
	if EstCode(nil, CodeInterne) {
		t.Error("EstCode(nil, ...) doit être faux")
	}
}

// TestUnwrapAvecCause vérifie que AvecCause rend la cause détectable par errors.Is
// (compatibilité avec le mécanisme standard d'enveloppement d'erreurs).
func TestUnwrapAvecCause(t *testing.T) {
	sentinelle := errors.New("erreur de bas niveau")
	err := Interne("échec de lecture").AvecCause(sentinelle)

	if !errors.Is(err, sentinelle) {
		t.Error("errors.Is aurait dû remonter jusqu'à la cause via Unwrap")
	}
	if err.Unwrap() != sentinelle {
		t.Error("Unwrap() aurait dû renvoyer la cause attachée")
	}
	// Le message d'erreur inclut la cause (utile pour les logs internes uniquement).
	if err.Error() == "" {
		t.Error("Error() ne doit pas être vide")
	}
}
