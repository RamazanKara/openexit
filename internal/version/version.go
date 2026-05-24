package version

var (
	Name    = "openexit"
	Version = "0.1.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func Info() map[string]string {
	return map[string]string{
		"name":    Name,
		"version": Version,
		"commit":  Commit,
		"date":    Date,
	}
}
