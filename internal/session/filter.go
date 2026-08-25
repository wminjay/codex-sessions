package session

import (
	"sort"
	"strings"
)

func Filter(items []Session, query string) []Session {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return append([]Session(nil), items...)
	}
	result := make([]Session, 0, len(items))
	for _, item := range items {
		haystack := strings.ToLower(strings.Join([]string{
			item.ID, item.Title, item.Provider, item.Source, item.CWD, item.Model,
		}, " "))
		if containsAll(haystack, terms) {
			result = append(result, item)
		}
	}
	return result
}

func Group(items []Session) []Directory {
	grouped := make(map[string][]Session)
	for _, item := range items {
		grouped[item.CWD] = append(grouped[item.CWD], item)
	}
	directories := make([]Directory, 0, len(grouped))
	for cwd, group := range grouped {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].UpdatedMS == group[j].UpdatedMS {
				return group[i].ID > group[j].ID
			}
			return group[i].UpdatedMS > group[j].UpdatedMS
		})
		directories = append(directories, Directory{
			CWD:       cwd,
			Sessions:  group,
			UpdatedMS: group[0].UpdatedMS,
		})
	}
	sort.SliceStable(directories, func(i, j int) bool {
		if directories[i].UpdatedMS == directories[j].UpdatedMS {
			return directories[i].CWD > directories[j].CWD
		}
		return directories[i].UpdatedMS > directories[j].UpdatedMS
	})
	return directories
}

func FilterDirectories(items []Directory, query string) []Directory {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return append([]Directory(nil), items...)
	}
	result := make([]Directory, 0, len(items))
	for _, item := range items {
		parts := []string{item.CWD, item.Project(), item.ProviderSummary()}
		for _, thread := range item.Sessions {
			parts = append(parts, thread.Title, thread.Provider, thread.Model)
		}
		if containsAll(strings.ToLower(strings.Join(parts, " ")), terms) {
			result = append(result, item)
		}
	}
	return result
}

func Resolve(items []Session, value string) (Session, error) {
	for _, item := range items {
		if item.ID == value {
			return item, nil
		}
	}
	matches := make([]Session, 0, 2)
	for _, item := range items {
		if strings.HasPrefix(item.ID, value) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return Session{}, ErrSessionNotFound{Value: value}
	}
	if len(matches) > 1 {
		return Session{}, ErrAmbiguousSession{Value: value}
	}
	return matches[0], nil
}

type ErrSessionNotFound struct{ Value string }

func (e ErrSessionNotFound) Error() string { return "session not found: " + e.Value }

type ErrAmbiguousSession struct{ Value string }

func (e ErrAmbiguousSession) Error() string { return "session ID prefix is ambiguous: " + e.Value }

func searchTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	return fields
}

func containsAll(haystack string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
