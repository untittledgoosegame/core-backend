package provider

type Provider string

const (
	YouTube      Provider = "youtube"
	Roblox       Provider = "roblox"
	Instagram    Provider = "instagram"
	X            Provider = "x"
	Spotify      Provider = "spotify"
	GitHub       Provider = "github"
	RiotGames    Provider = "riot_games"
	LinkedIn     Provider = "linkedin"
	Reddit       Provider = "reddit"
	PlayStation  Provider = "playstation"
	CustomDomain Provider = "custom_domain"
	Crunchyroll  Provider = "crunchyroll"
	AmazonPrime  Provider = "amazon_prime"
	Xbox         Provider = "xbox"
	PayPal       Provider = "paypal"
	Steam        Provider = "steam"
	EpicGames    Provider = "epic_games"
)

// All lists the providers in display order.
var All = []Provider{
	YouTube, Roblox, Instagram, X, Spotify, GitHub, RiotGames, LinkedIn, Reddit,
	PlayStation, CustomDomain, Crunchyroll, AmazonPrime, Xbox, PayPal, Steam, EpicGames,
}

func (p Provider) Valid() bool {
	for _, known := range All {
		if p == known {
			return true
		}
	}
	return false
}

// Flags returns every provider mapped to false, with the connected ones set to true.
func Flags(connected map[Provider]bool) map[Provider]bool {
	flags := make(map[Provider]bool, len(All))
	for _, p := range All {
		flags[p] = connected[p]
	}
	return flags
}
