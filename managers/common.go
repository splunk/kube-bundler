package managers

const (
	DefaultResourceName = "default"
)

// InstallReference identifies an installation
type InstallReference struct {
	Name      string
	Namespace string
}
