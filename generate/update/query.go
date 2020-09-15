package update

import (
	"errors"
	"sync"

	"github.com/safe-waters/docker-lock/generate/parse"
	"github.com/safe-waters/docker-lock/registry"
)

// QueryExecutor queries for digests, caching results of past queries
type QueryExecutor struct {
	WrapperManager *registry.WrapperManager
	cache          map[parse.Image]*cacheResult
	mutex          *sync.Mutex
}

// IQueryExecutor provides an interface for QueryExecutor's exported methods,
// which are used by ImageDigestUpdaters.
type IQueryExecutor interface {
	QueryRegistry(image parse.Image) *QueryResult
}

// QueryResult contains an image with the updated digest and any error
// associated with querying for the digest.
type QueryResult struct {
	*parse.Image
	Err error
}

type cacheResult struct {
	queryResult *QueryResult
	done        chan struct{}
}

// NewQueryExecutor returns a QueryExecutor after validating its fields.
func NewQueryExecutor(
	wrapperManager *registry.WrapperManager,
) (*QueryExecutor, error) {
	if wrapperManager == nil {
		return nil, errors.New("wrapperManager cannot be nil")
	}

	var mutex sync.Mutex

	cache := map[parse.Image]*cacheResult{}

	return &QueryExecutor{
		WrapperManager: wrapperManager,
		cache:          cache,
		mutex:          &mutex,
	}, nil
}

// QueryRegistry queries the appropriate registry for a digest.
func (q *QueryExecutor) QueryRegistry(image parse.Image) *QueryResult {
	q.mutex.Lock()

	result, ok := q.cache[image]
	if ok {
		q.mutex.Unlock()
		<-result.done

		return result.queryResult
	}

	q.cache[image] = &cacheResult{
		queryResult: &QueryResult{
			Image: &parse.Image{
				Name: image.Name,
				Tag:  image.Tag,
			},
		},
		done: make(chan struct{}),
	}
	q.mutex.Unlock()

	wrapper := q.WrapperManager.Wrapper(image.Name)

	digest, err := wrapper.Digest(image.Name, image.Tag)

	q.cache[image].queryResult.Digest = digest
	q.cache[image].queryResult.Err = err

	close(q.cache[image].done)

	return q.cache[image].queryResult
}
