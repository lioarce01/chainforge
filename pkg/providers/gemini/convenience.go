package gemini

// NewFlash creates a Provider using gemini-2.0-flash (fast, cost-effective default).
func NewFlash(apiKey string) (*Provider, error) {
	return New(apiKey, "gemini-2.0-flash")
}

// NewPro creates a Provider using gemini-2.0-pro.
func NewPro(apiKey string) (*Provider, error) {
	return New(apiKey, "gemini-2.0-pro")
}
