package unittests

import (
	"testing"

	"github.com/michaelperel/docker-lock/registry"
	"github.com/michaelperel/docker-lock/registry/contrib"
	_ "github.com/michaelperel/docker-lock/registry/contrib"
	"github.com/michaelperel/docker-lock/registry/firstparty"
)

// TestFirstPartyGetAllWrappers ensures that all wrappers officially
// supported by docker-lock's maintainers are returned.
func TestFirstPartyGetAllWrappers(t *testing.T) {
	client := &registry.HTTPClient{}
	wrappers, err := firstparty.GetAllWrappers("", client)
	if err != nil {
		t.Fatal("Could not get wrappers.")
	}
	expectedNumWrappers := 2
	numWrappers := len(wrappers)
	if numWrappers != expectedNumWrappers {
		t.Fatalf("Got '%d' first party wrappers. Want '%d'.",
			numWrappers,
			expectedNumWrappers,
		)
	}
	if _, ok := wrappers[0].(*firstparty.DockerWrapper); !ok {
		t.Fatal("Expected DockerWrapper.")
	}
	if _, ok := wrappers[1].(*firstparty.ACRWrapper); !ok {
		t.Fatal("Expected ACRWrapper.")
	}
}

// TestFirstPartyGetDefaultWrapper ensures that the default wrapper
// is one that can handle images without a prefix.
func TestFirstPartyGetDefaultWrapper(t *testing.T) {
	client := &registry.HTTPClient{}
	wrapper, err := firstparty.GetDefaultWrapper("", client)
	if err != nil {
		t.Fatal("Could not get default wrapper.")
	}
	if _, ok := wrapper.(*firstparty.DockerWrapper); !ok {
		t.Fatal("Expected DockerWrapper.")
	}
}

// TestContribGetAllWrappers ensures that all wrappers maintained by the
// community are returned.
func TestContribGetAllWrappers(t *testing.T) {
	client := &registry.HTTPClient{}
	wrappers, err := contrib.GetAllWrappers(client)
	if err != nil {
		t.Fatal("Could not get wrappers.")
	}
	expectedNumWrappers := 2
	numWrappers := len(wrappers)
	if numWrappers != expectedNumWrappers {
		t.Fatalf("Got '%d' first party wrappers. Want '%d'.",
			numWrappers,
			expectedNumWrappers,
		)
	}
	if _, ok := wrappers[0].(*contrib.ElasticWrapper); !ok {
		t.Fatal("Expected ElasticWrapper.")
	}
	if _, ok := wrappers[1].(*contrib.MCRWrapper); !ok {
		t.Fatal("Expected MCRWrapper.")
	}
}
