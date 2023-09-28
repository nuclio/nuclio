package options

type ExportFunction struct {
	NoScrub         bool
	SkipSpecCleanup bool
	WithPrevState   bool
	PrevState       string
}
