// Package validation fournit une validation des entrées ENTIÈREMENT écrite à la
// main, sans bibliothèque externe ni « tags de struct » magiques.
//
// # Pourquoi valider soi-même ?
//
// Sans ORM ni framework de validation, on garde un contrôle total et lisible sur
// les règles métier. C'est aussi plus pédagogique : chaque règle est du code Go
// explicite que l'on peut lire et comprendre.
//
// # Le motif « accumulateur d'erreurs »
//
// Plutôt que de s'arrêter à la première erreur, on les collecte TOUTES dans un
// Validateur, puis on renvoie l'ensemble d'un coup. L'utilisateur voit ainsi
// tous ses problèmes de saisie en une seule réponse (bien meilleure expérience).
//
// La validation produit une *apperreur.Erreur de type « VALIDATION » (HTTP 422)
// dont le champ Details liste, pour chaque champ fautif, l'explication en français.
package validation

import (
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
)

// # À propos des variables au niveau du package
//
// Ces `regexp.Regexp` sont compilées UNE SEULE FOIS au démarrage via
// MustCompile. Ce sont des valeurs IMMUABLES (jamais réassignées), et non un
// état mutable partagé : c'est l'idiome Go recommandé pour les expressions
// régulières, sûr pour un usage concurrent. La règle « pas de variable globale »
// vise l'état mutable partagé (connexions, config...), pas ces constantes.
var (
	// Format d'un UUID (version 4 en pratique) : 8-4-4-4-12 caractères hexadécimaux.
	motifUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	// Format d'une date ISO 8601 « AAAA-MM-JJ ». On complète ce contrôle de forme
	// par un vrai parsing pour rejeter les dates impossibles (ex. 2023-02-31).
	motifDateISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	// Un ISBN-13 est composé de 13 chiffres (on tolère les tirets à l'affichage,
	// mais on les retire avant de valider la clé de contrôle).
	motifChiffres13 = regexp.MustCompile(`^\d{13}$`)
)

// EstEmailValide vérifie qu'une chaîne est une adresse e-mail plausible.
//
// On s'appuie sur net/mail.ParseAddress (bibliothèque standard) qui applique les
// règles RFC 5322, PUIS on s'assure que l'entrée ne contient pas de nom
// d'affichage (« Jean <j@ex.fr> ») en comparant l'adresse extraite à l'entrée.
// La validation d'e-mail par regex seule est notoirement imparfaite : mieux vaut
// réutiliser l'analyseur standard.
func EstEmailValide(valeur string) bool {
	valeur = strings.TrimSpace(valeur)
	if valeur == "" || len(valeur) > 254 { // 254 = longueur max d'une adresse (RFC 5321)
		return false
	}
	adresse, err := mail.ParseAddress(valeur)
	return err == nil && adresse.Address == valeur
}

// EstUUIDValide vérifie le format d'un UUID.
func EstUUIDValide(valeur string) bool {
	return motifUUID.MatchString(valeur)
}

// EstDateISOValide vérifie qu'une chaîne est une date « AAAA-MM-JJ » réelle.
func EstDateISOValide(valeur string) bool {
	if !motifDateISO.MatchString(valeur) {
		return false
	}
	// time.Parse rejette les dates syntaxiquement correctes mais impossibles,
	// par exemple le 31 février.
	_, err := time.Parse("2006-01-02", valeur)
	return err == nil
}

// EstISBN13Valide vérifie un ISBN-13, y compris sa CLÉ DE CONTRÔLE.
//
// L'ISBN-13 n'est pas qu'une suite de 13 chiffres : le dernier chiffre est une
// clé calculée à partir des 12 premiers (somme pondérée 1,3,1,3... modulo 10).
// Vérifier cette clé détecte la plupart des fautes de frappe. C'est un bon
// exemple de validation métier « intelligente ».
func EstISBN13Valide(valeur string) bool {
	// On retire les tirets et espaces de présentation avant de valider.
	nettoye := strings.NewReplacer("-", "", " ", "").Replace(valeur)
	if !motifChiffres13.MatchString(nettoye) {
		return false
	}
	somme := 0
	for i := range 12 {
		chiffre := int(nettoye[i] - '0')
		if i%2 == 0 {
			somme += chiffre // poids 1 pour les positions paires (0,2,4...)
		} else {
			somme += chiffre * 3 // poids 3 pour les positions impaires
		}
	}
	cleAttendue := (10 - (somme % 10)) % 10
	cleReelle := int(nettoye[12] - '0')
	return cleAttendue == cleReelle
}

// NormaliserISBN retire tirets et espaces d'un ISBN pour le stocker sous forme
// canonique (13 chiffres). Utile avant l'insertion en base.
func NormaliserISBN(valeur string) string {
	return strings.NewReplacer("-", "", " ", "").Replace(valeur)
}

// longueurRunes compte les CARACTÈRES (runes) et non les octets. En UTF-8, « é »
// occupe 2 octets mais 1 caractère : compter les octets fausserait les limites.
func longueurRunes(valeur string) int {
	return utf8.RuneCountInString(valeur)
}

// Validateur accumule les erreurs de validation, champ par champ.
type Validateur struct {
	erreurs map[string]string
}

// Nouveau crée un Validateur vide, prêt à recevoir des règles.
func Nouveau() *Validateur {
	return &Validateur{erreurs: make(map[string]string)}
}

// AjouterErreur enregistre une erreur pour un champ (sans écraser la première
// erreur déjà présente sur ce champ : on garde la plus prioritaire).
func (v *Validateur) AjouterErreur(champ, message string) {
	if _, existe := v.erreurs[champ]; !existe {
		v.erreurs[champ] = message
	}
}

// Verifier ajoute une erreur si la condition est fausse. C'est la brique de base
// qui rend les validations très lisibles :
//
//	v.Verifier(age >= 18, "age", "doit être majeur")
func (v *Validateur) Verifier(condition bool, champ, message string) {
	if !condition {
		v.AjouterErreur(champ, message)
	}
}

// EstValide indique qu'aucune erreur n'a été collectée.
func (v *Validateur) EstValide() bool {
	return len(v.erreurs) == 0
}

// Erreur renvoie nil si tout est valide, sinon une *apperreur.Erreur de type
// VALIDATION (HTTP 422) contenant le détail des champs fautifs.
func (v *Validateur) Erreur() error {
	if v.EstValide() {
		return nil
	}
	return apperreur.Validation("Un ou plusieurs champs sont invalides.", v.erreurs)
}

// --- Règles de commodité ---------------------------------------------------
//
// Ces méthodes encapsulent les vérifications les plus courantes. Elles rendent
// le code des services concis et déclaratif.

// ChampRequis vérifie qu'une chaîne n'est pas vide (après suppression des espaces).
func (v *Validateur) ChampRequis(champ, valeur string) {
	v.Verifier(strings.TrimSpace(valeur) != "", champ, "ce champ est obligatoire")
}

// LongueurMax vérifie qu'une chaîne ne dépasse pas `max` caractères.
func (v *Validateur) LongueurMax(champ, valeur string, max int) {
	v.Verifier(longueurRunes(valeur) <= max, champ, "ne doit pas dépasser "+itoa(max)+" caractères")
}

// LongueurMin vérifie qu'une chaîne fait au moins `min` caractères.
func (v *Validateur) LongueurMin(champ, valeur string, min int) {
	v.Verifier(longueurRunes(valeur) >= min, champ, "doit contenir au moins "+itoa(min)+" caractères")
}

// Email vérifie le format d'une adresse e-mail.
func (v *Validateur) Email(champ, valeur string) {
	v.Verifier(EstEmailValide(valeur), champ, "adresse e-mail invalide")
}

// UUID vérifie le format d'un identifiant public.
func (v *Validateur) UUID(champ, valeur string) {
	v.Verifier(EstUUIDValide(valeur), champ, "identifiant invalide")
}

// DateISO vérifie qu'une chaîne est une date « AAAA-MM-JJ » valide.
func (v *Validateur) DateISO(champ, valeur string) {
	v.Verifier(EstDateISOValide(valeur), champ, "date invalide (format attendu : AAAA-MM-JJ)")
}

// ISBN13 vérifie un ISBN-13 avec sa clé de contrôle.
func (v *Validateur) ISBN13(champ, valeur string) {
	v.Verifier(EstISBN13Valide(valeur), champ, "ISBN-13 invalide")
}

// EntierDans vérifie qu'un entier est compris dans l'intervalle [min, max].
func (v *Validateur) EntierDans(champ string, valeur, min, max int) {
	v.Verifier(valeur >= min && valeur <= max, champ,
		"doit être compris entre "+itoa(min)+" et "+itoa(max))
}

// DansEnsemble vérifie qu'une valeur figure parmi les valeurs autorisées.
// Sert notamment à valider les énumérations (rôles, statuts...).
func (v *Validateur) DansEnsemble(champ, valeur string, autorisees ...string) {
	if !slices.Contains(autorisees, valeur) {
		v.AjouterErreur(champ, "valeur non autorisée (attendu : "+strings.Join(autorisees, ", ")+")")
	}
}

// itoa est un petit utilitaire local pour convertir un entier en chaîne sans
// tirer strconv dans chaque appel (lisibilité). Volontairement minimaliste.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negatif := n < 0
	if negatif {
		n = -n
	}
	var chiffres [20]byte
	i := len(chiffres)
	for n > 0 {
		i--
		chiffres[i] = byte('0' + n%10)
		n /= 10
	}
	if negatif {
		i--
		chiffres[i] = '-'
	}
	return string(chiffres[i:])
}
