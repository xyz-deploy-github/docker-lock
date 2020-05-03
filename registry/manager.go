package registry

import "strings"

// WrapperManager selects which registry wrapper to use at runtime.
type WrapperManager struct {
	defaultWrapper Wrapper
	wrappers       []Wrapper
}

// NewWrapperManager creates a WrapperManager with a default wrapper
// that is selected if no prefix matches in Wrapper.
func NewWrapperManager(defaultWrapper Wrapper) *WrapperManager {
	return &WrapperManager{defaultWrapper: defaultWrapper}
}

// Add adds a registry wrapper to the WrapperManager's collection of wrappers
// to use at runtime.
func (m *WrapperManager) Add(wrappers ...Wrapper) {
	m.wrappers = append(m.wrappers, wrappers...)
}

// Wrapper selects a registry wrapper if the image name starts with
// the wrapper's prefix. If no match is found, the default wrapper
// is used.
func (m *WrapperManager) Wrapper(imageName string) Wrapper {
	for _, wrapper := range m.wrappers {
		p := wrapper.Prefix()
		if p != "" && strings.HasPrefix(imageName, p) {
			return wrapper
		}
	}

	return m.defaultWrapper
}
