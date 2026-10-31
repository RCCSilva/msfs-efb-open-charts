package chartfox

import "context"

// Catalog is what the server needs to look charts up and bring them: the airport, its chart list, the detail of a chart
// and its file. Client is the implementation; chartfoxmock has the mock.
type Catalog interface {
	Airport(ctx context.Context, icao string) (Airport, error)
	Charts(ctx context.Context, icao string) ([]Chart, error)
	Chart(ctx context.Context, id string) (Detail, error)
	Download(ctx context.Context, fileURL string) ([]byte, error)
}

var _ Catalog = (*Client)(nil)
