package adapters

type PackageDependency struct {
	Name        string
	VersionSpec string
	Manager     string
	Registry    string
}
