package netctl

// PolicyState сериализуется в JSON для отката между Apply.
type PolicyState struct {
	Version int           `json:"version"`
	Rules   []RuleEntry   `json:"rules"`
	Routes  []RouteEntry  `json:"routes"`
}

type RuleEntry struct {
	Pref  int    `json:"pref"`
	From  string `json:"from"`
	Table int    `json:"table"`
}

type RouteEntry struct {
	Table int    `json:"table"`
	Dev   string `json:"dev"`
}
