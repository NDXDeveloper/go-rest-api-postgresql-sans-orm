package models

// Ce fichier définit les paramètres communs à toutes les listes paginées.
//
// # Sécurité du tri (point crucial)
//
// Les valeurs d'une requête SQL (WHERE colonne = ?) peuvent être passées via des
// paramètres préparés `?`, ce qui neutralise les injections SQL. En revanche,
// un NOM DE COLONNE ou le sens du tri (ASC/DESC) NE PEUVENT PAS être des
// paramètres `?` : ils font partie de la structure de la requête.
//
// Conséquence : si l'on interpolait directement « ORDER BY » + valeur fournie
// par le client, on ouvrirait une faille d'injection SQL. La parade est une
// LISTE BLANCHE (whitelist) : le client envoie un nom logique (« titre »), que
// l'on traduit en nom de colonne réel UNIQUEMENT s'il figure dans la liste
// autorisée. Le champ ColonneTri ci-dessous ne contient donc JAMAIS une valeur
// brute du client : toujours une colonne validée, sûre à interpoler.

const (
	// PageParDefaut : première page si le client n'en précise pas.
	PageParDefaut = 1
	// TailleParDefaut : nombre d'éléments par page par défaut.
	TailleParDefaut = 20
	// TailleMax : borne haute pour éviter qu'un client ne demande des millions
	// de lignes d'un coup (protection mémoire / DoS).
	TailleMax = 100
	// OrdreAsc : tri ascendant (valeur SQL « ASC »). Avec OrdreDesc, ce sont les
	// deux seules valeurs autorisées pour le sens du tri.
	OrdreAsc = "ASC"
	// OrdreDesc : tri descendant (valeur SQL « DESC »).
	OrdreDesc = "DESC"
)

// ParametresListe transporte les options de pagination, tri, recherche et
// filtrage depuis le handler jusqu'au repository.
type ParametresListe struct {
	// Page (1-indexée) et Taille définissent la fenêtre de pagination.
	Page   int
	Taille int

	// ColonneTri est un nom de colonne SQL DÉJÀ VALIDÉ contre une liste blanche.
	// Il est donc sûr de l'interpoler dans « ORDER BY ». Voir l'avertissement
	// de sécurité en tête de fichier.
	ColonneTri string

	// Ordre vaut exactement "ASC" ou "DESC" (jamais une valeur brute du client).
	Ordre string

	// Recherche est un terme de recherche plein-texte simple (LIKE). La valeur
	// est passée en paramètre `?`, donc protégée contre l'injection.
	Recherche string

	// Filtres contient des filtres additionnels propres à chaque entité
	// (clé = nom logique, valeur = valeur brute). Chaque repository décide
	// comment les appliquer, toujours via des paramètres `?`.
	Filtres map[string]string
}

// Offset traduit la pagination (page, taille) en décalage SQL pour LIMIT/OFFSET.
//
//	SELECT ... LIMIT {Taille} OFFSET {Offset}
func (p ParametresListe) Offset() int {
	return (p.Page - 1) * p.Taille
}

// Normaliser applique des bornes de sécurité aux paramètres bruts.
// On garantit ainsi Page >= 1 et 1 <= Taille <= TailleMax.
func (p *ParametresListe) Normaliser() {
	if p.Page < 1 {
		p.Page = PageParDefaut
	}
	if p.Taille < 1 {
		p.Taille = TailleParDefaut
	}
	if p.Taille > TailleMax {
		p.Taille = TailleMax
	}
	if p.Ordre != OrdreAsc && p.Ordre != OrdreDesc {
		p.Ordre = OrdreAsc
	}
}
