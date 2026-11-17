package perforce

import (
	"context"
	"slices"
)

// Sorting is chunked and merged so large inventories observe cancellation.
func localSortedPaths(ctx context.Context, paths map[string]struct{}) ([]string, error) {
	out := make([]string, 0, len(paths))
	for name := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	for start := 0; start < len(out); start += 1024 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		slices.Sort(out[start:min(start+1024, len(out))])
	}
	buffer := make([]string, len(out))
	for width := 1024; width < len(out); width *= 2 {
		for start := 0; start < len(out); start += 2 * width {
			middle, end := min(start+width, len(out)), min(start+2*width, len(out))
			if err := localMergePaths(ctx, buffer[start:end], out[start:middle], out[middle:end]); err != nil {
				return nil, err
			}
		}
		out, buffer = buffer, out
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func localMergePaths(ctx context.Context, out, left, right []string) error {
	i, j := 0, 0
	for index := range out {
		if index%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if j == len(right) || i < len(left) && left[i] <= right[j] {
			out[index] = left[i]
			i++
		} else {
			out[index] = right[j]
			j++
		}
	}
	return nil
}
