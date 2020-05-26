package contrib

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/michaelperel/docker-lock/registry"
)

// TestAllWrappers ensures that all wrappers maintained by the
// community are returned.
func TestAllWrappers(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	client := &registry.HTTPClient{}

	wrappers := AllWrappers(client, "")

	expectedNumWrappers := 2
	numWrappers := len(wrappers)

	if numWrappers != expectedNumWrappers {
		t.Fatalf("got '%d' wrappers, want '%d'",
			numWrappers,
			expectedNumWrappers,
		)
	}

	if _, ok := wrappers[0].(*ElasticWrapper); !ok {
		t.Fatal("expected ElasticWrapper")
	}

	if _, ok := wrappers[1].(*MCRWrapper); !ok {
		t.Fatal("expected MCRWrapper")
	}
}
