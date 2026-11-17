package workspace

import (
	"context"
	"errors"
	"fmt"
	"math"
)

func localObjectCopyRequired(store localObjectStore, next localManifest, total uint64) (uint64, error) {
	need, err := localRequiredSpace(next, total)
	if err != nil {
		return 0, err
	}
	need, err = addLocalObjectStorageCost(need, store)
	if err == nil && store.metadata != nil {
		need, err = addLocalObjectStorageCost(need, *store.metadata)
	}
	return need, err
}

func addLocalObjectStorageCost(need uint64, store localObjectStore) (uint64, error) {
	for _, part := range []struct{ value, factor uint64 }{{store.bytes, 1}, {uint64(len(store.files)), 4096}, {store.paths, 2}, {store.directories, 4096}, {store.directoryPaths, 2}} {
		if part.value > math.MaxUint64/part.factor || part.value*part.factor > math.MaxUint64-need {
			return 0, errors.New("本地对象复制空间估计溢出")
		}
		need += part.value * part.factor
	}
	return need, nil
}

type localCopySpaceProbe func(string) (uint64, uint64, error)

func checkLocalObjectCapacity(ctx context.Context, target string, store localObjectStore, next localManifest, probe localCopySpaceProbe) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	free, total, err := probe(target)
	if err != nil {
		return err
	}
	need, err := localObjectCopyRequired(store, next, total)
	if err != nil {
		return err
	}
	if free < need {
		return fmt.Errorf("本地对象复制空间不足：需要%d字节，可用%d字节", need, free)
	}
	return ctx.Err()
}
