//go:build !unix

package supervisor

import "fmt"

func scanManagedTreePlatform(
	string,
	int,
	int64,
	int64,
	bool,
) (map[string]workspaceEntry, error) {
	return nil, fmt.Errorf("%w: descriptor-rooted traversal unsupported", ErrManagedWorkspace)
}
