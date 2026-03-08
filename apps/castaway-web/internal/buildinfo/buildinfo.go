package buildinfo

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func String() string {
	return "version=" + Version + " commit=" + Commit + " build_date=" + BuildDate
}
