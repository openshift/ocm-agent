package httpchecker

// HTTPChecker defines an interface for checking URL availability.
type HTTPChecker interface {
	UrlAvailabilityCheck(url string) error
}
