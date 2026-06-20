package version

const AppName = "spick"

var Version = "dev"

func String() string {
	return AppName + " " + Version
}
