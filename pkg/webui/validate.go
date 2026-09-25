package webui

// Validates that which cannot be expressed by the type system.
// An optimal validation function should reduce the set of possible runtime errors to zero.
func (a App) validate() CompileErrors {
	return nil
}
