package user

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

const (
	minUsernameLen = 3
	maxUsernameLen = 30
	maxNameLen     = 50
	maxBioLen      = 160
	minAge         = 13
	suggestCount   = 5
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._]*[a-z0-9])?$`)

var reservedUsernames = map[string]bool{
	"admin": true, "administrator": true, "root": true, "support": true, "help": true,
	"api": true, "system": true, "goose": true, "moderator": true, "staff": true,
	"me": true, "settings": true, "login": true, "register": true, "null": true,
}

func UsernameProblem(username string) string {
	switch {
	case len(username) < minUsernameLen || len(username) > maxUsernameLen:
		return fmt.Sprintf("must be %d-%d characters", minUsernameLen, maxUsernameLen)
	case !usernamePattern.MatchString(username):
		return "only lowercase letters, numbers, dots and underscores; must start and end with a letter or number"
	case strings.Contains(username, ".."), strings.Contains(username, "__"):
		return "no repeated dots or underscores"
	case reservedUsernames[username]:
		return "reserved"
	}
	return ""
}

func nameParts(name string) []string {
	decomposed := norm.NFD.String(strings.ToLower(name))
	var b strings.Builder
	for _, r := range decomposed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsSpace(r), r == '-', r == '_', r == '.':
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}

func usernameCandidates(name string) []string {
	parts := nameParts(name)
	if len(parts) == 0 {
		parts = []string{"goose"}
	}
	first, last := parts[0], parts[len(parts)-1]
	joined := strings.Join(parts, "")

	raw := []string{joined, strings.Join(parts, "."), strings.Join(parts, "_")}
	if len(parts) > 1 {
		raw = append(raw, first+last[:1], first[:1]+last, last+"."+first)
	}
	raw = append(raw, first)

	seen := map[string]bool{}
	var out []string
	add := func(c string) {
		if len(c) > maxUsernameLen {
			c = strings.TrimRight(c[:maxUsernameLen], "._")
		}
		if !seen[c] && UsernameProblem(c) == "" {
			seen[c] = true
			out = append(out, c)
		}
	}
	for _, c := range raw {
		add(c)
	}
	base := joined
	if len(base) > maxUsernameLen-4 {
		base = base[:maxUsernameLen-4]
	}
	for len(out) < suggestCount*3 {
		add(fmt.Sprintf("%s%d", base, rand.IntN(9000)+100))
	}
	return out
}

func (s *Store) SuggestUsernames(ctx context.Context, name string) ([]string, error) {
	candidates := usernameCandidates(name)
	taken, err := s.TakenUsernames(ctx, candidates)
	if err != nil {
		return nil, err
	}
	suggestions := make([]string, 0, suggestCount)
	for _, c := range candidates {
		if !taken[c] {
			suggestions = append(suggestions, c)
			if len(suggestions) == suggestCount {
				break
			}
		}
	}
	return suggestions, nil
}

func (s *Store) UsernameAvailable(ctx context.Context, username string) (bool, string, error) {
	if problem := UsernameProblem(username); problem != "" {
		return false, problem, nil
	}
	taken, err := s.TakenUsernames(ctx, []string{username})
	if err != nil {
		return false, "", err
	}
	if taken[username] {
		return false, "taken", nil
	}
	return true, "", nil
}

func NormalizeProfile(p Profile, now time.Time) (Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Username = strings.ToLower(strings.TrimSpace(p.Username))
	p.Bio = strings.TrimSpace(p.Bio)
	p.AvatarURL = strings.TrimSpace(p.AvatarURL)

	if n := utf8.RuneCountInString(p.Name); n == 0 || n > maxNameLen {
		return p, apperr.BadRequest(fmt.Sprintf("name must be 1-%d characters", maxNameLen))
	}
	if problem := UsernameProblem(p.Username); problem != "" {
		return p, apperr.BadRequest("username " + problem)
	}
	dob, err := time.Parse(time.DateOnly, p.DOB)
	if err != nil {
		return p, apperr.BadRequest("dob must be YYYY-MM-DD")
	}
	if dob.After(now) || dob.AddDate(minAge, 0, 0).After(now) {
		return p, apperr.BadRequest(fmt.Sprintf("you must be at least %d years old", minAge))
	}
	if utf8.RuneCountInString(p.Bio) > maxBioLen {
		return p, apperr.BadRequest(fmt.Sprintf("bio must be at most %d characters", maxBioLen))
	}
	if p.AvatarURL != "" {
		u, err := url.Parse(p.AvatarURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return p, apperr.BadRequest("avatar_url must be an https URL")
		}
	}
	return p, nil
}
