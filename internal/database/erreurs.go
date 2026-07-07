package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Codes d'erreur PostgreSQL (« SQLSTATE ») que l'on sait interpréter. Les
// traduire en concepts métier permet aux repositories de renvoyer une
// *apperreur.Erreur claire (409 Conflit, etc.) plutôt qu'une erreur technique brute.
//
// Contrairement à MySQL/MariaDB (codes numériques comme 1062), PostgreSQL utilise
// des codes SQLSTATE normalisés de 5 caractères.
// Référence : https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	codeDoublon        = "23505" // unique_violation : violation d'une contrainte UNIQUE
	codeCleEtrangere   = "23503" // foreign_key_violation : INSERT/UPDATE référençant un parent inexistant
	codeRestrict       = "23001" // restrict_violation : DELETE/UPDATE bloqué par une FK « ON DELETE/UPDATE RESTRICT »
	codeCheck          = "23514" // check_violation : violation d'une contrainte CHECK
	codeRaiseException = "P0001" // raise_exception : RAISE EXCEPTION explicite en PL/pgSQL (équivalent du SIGNAL de MariaDB)
)

// commePg tente de convertir une erreur en *pgconn.PgError afin d'inspecter son
// code SQLSTATE. Renvoie (nil, false) si ce n'en est pas une.
func commePg(err error) (*pgconn.PgError, bool) {
	var e *pgconn.PgError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// EstErreurDoublon indique si l'erreur provient d'une violation de contrainte
// UNIQUE (par exemple, un e-mail ou un ISBN déjà présent). Le repository peut
// alors renvoyer un 409 Conflit avec un message adapté.
func EstErreurDoublon(err error) bool {
	e, ok := commePg(err)
	return ok && e.Code == codeDoublon
}

// EstErreurCleEtrangere indique si l'erreur provient d'une contrainte de clé
// étrangère : soit on référence un parent inexistant (INSERT/UPDATE → 23503),
// soit on tente de supprimer une ligne encore référencée par une FK « RESTRICT »
// (DELETE → 23001). PIÈGE POSTGRESQL : une suppression bloquée par ON DELETE
// RESTRICT lève « restrict_violation » (23001), un code DISTINCT de
// « foreign_key_violation » (23503) — il faut donc tester les deux. (MySQL, lui,
// utilise 1451/1452.)
func EstErreurCleEtrangere(err error) bool {
	e, ok := commePg(err)
	return ok && (e.Code == codeCleEtrangere || e.Code == codeRestrict)
}

// MessageSignal extrait le texte d'une exception levée par un trigger ou une
// procédure via « RAISE EXCEPTION '...' » (SQLSTATE P0001). Ces messages sont
// RÉDIGÉS en français et pensés pour l'utilisateur : on peut donc les exposer
// tels quels (à la différence des autres erreurs SQL). Renvoie (message, true)
// si applicable. C'est l'équivalent PostgreSQL du SIGNAL SQLSTATE '45000' de MariaDB.
func MessageSignal(err error) (string, bool) {
	e, ok := commePg(err)
	if ok && e.Code == codeRaiseException {
		return e.Message, true
	}
	return "", false
}
