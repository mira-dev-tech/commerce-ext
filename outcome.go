package commerceext

// Outcome is the result of a blocking hook (checkout.validate, payment.route, …).
type Outcome struct {
	Allow   bool
	Code    string
	Message string
}

// Allowed returns a permissive outcome.
func Allowed() Outcome {
	return Outcome{Allow: true}
}

// Denied returns a blocking outcome.
func Denied(code, message string) Outcome {
	return Outcome{Allow: false, Code: code, Message: message}
}
