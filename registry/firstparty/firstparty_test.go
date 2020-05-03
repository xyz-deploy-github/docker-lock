package firstparty

import (
	"testing"

	"github.com/michaelperel/docker-lock/registry"
)

// TestAllWrappers ensures that all wrappers officially
// supported by docker-lock's maintainers are returned.
func TestAllWrappers(t *testing.T) {
	client := &registry.HTTPClient{}

	wrappers, err := AllWrappers("", client)
	if err != nil {
		t.Fatal("could not get wrappers")
	}

	expectedNumWrappers := 2
	numWrappers := len(wrappers)

	if numWrappers != expectedNumWrappers {
		t.Fatalf("got '%d' wrappers, want '%d'",
			numWrappers,
			expectedNumWrappers,
		)
	}

	if _, ok := wrappers[0].(*DockerWrapper); !ok {
		t.Fatal("expected DockerWrapper")
	}

	if _, ok := wrappers[1].(*ACRWrapper); !ok {
		t.Fatal("expected ACRWrapper")
	}
}

// TestDefaultWrapper ensures that the default wrapper
// is one that can handle images without a prefix.
func TestDefaultWrapper(t *testing.T) {
	client := &registry.HTTPClient{}

	wrapper, err := DefaultWrapper("", client)
	if err != nil {
		t.Fatal("could not get default wrapper")
	}

	if _, ok := wrapper.(*DockerWrapper); !ok {
		t.Fatal("expected DockerWrapper")
	}
}
