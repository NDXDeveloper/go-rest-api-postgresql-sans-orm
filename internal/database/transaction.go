package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SQLExecuteur abstrait ce qui est commun à *sql.DB et *sql.Tx : la capacité
// d'exécuter des requêtes. Grâce à cette interface, une même fonction de
// repository peut travailler indifféremment sur le pool (*sql.DB, hors
// transaction) ou sur une transaction (*sql.Tx). C'est le petit motif qui évite
// de dupliquer chaque requête en deux versions.
type SQLExecuteur interface {
	ExecContext(ctx context.Context, requete string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, requete string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, requete string, args ...any) *sql.Row
}

// EnTransaction exécute la fonction `fn` à l'intérieur d'une transaction SQL, en
// gérant automatiquement COMMIT et ROLLBACK. C'est le cœur de la démonstration
// des transactions en Go.
//
// Garanties :
//   - si `fn` renvoie une erreur          -> ROLLBACK (annulation totale) ;
//   - si `fn` réussit                     -> COMMIT (validation) ;
//   - si `fn` panique (bug inattendu)     -> ROLLBACK puis la panique est relancée,
//     pour ne jamais laisser une transaction ouverte « dans la nature ».
//
// C'est le motif « rollback automatique » : l'appelant écrit uniquement la
// logique métier dans `fn` et n'a pas à se soucier de fermer la transaction.
//
// Exemple :
//
//	err := database.EnTransaction(ctx, db, func(tx *sql.Tx) error {
//	    if _, err := tx.ExecContext(ctx, "UPDATE ...", ...); err != nil {
//	        return err // -> ROLLBACK
//	    }
//	    return nil     // -> COMMIT
//	})
func EnTransaction(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	// BeginTx ouvre la transaction. Le contexte permet d'annuler la transaction
	// si la requête HTTP est abandonnée (timeout, client déconnecté).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}

	// Filet de sécurité : en cas de panique dans `fn`, on annule la transaction
	// AVANT de laisser la panique se propager. Sans cela, la connexion resterait
	// bloquée avec une transaction ouverte.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err = fn(tx); err != nil {
		// ROLLBACK automatique. On ignore sql.ErrTxDone (transaction déjà close,
		// par exemple si `fn` a fait son propre COMMIT/ROLLBACK).
		if errAnnulation := tx.Rollback(); errAnnulation != nil && !errors.Is(errAnnulation, sql.ErrTxDone) {
			return fmt.Errorf("%w (échec du rollback : %v)", err, errAnnulation)
		}
		return err
	}

	// Tout s'est bien passé : on valide.
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("validation (commit) de la transaction : %w", err)
	}
	return nil
}
