package util

// InterleaveByKey performs round-robin interleaving of items grouped by keyFn.
// Groups are ordered by first appearance. Within each group, order is preserved.
func InterleaveByKey[T any](items []T, keyFn func(T) string) []T {
	if len(items) == 0 {
		return items
	}

	groups := map[string][]T{}
	var order []string
	for _, item := range items {
		k := keyFn(item)
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], item)
	}

	result := make([]T, 0, len(items))
	idx := make(map[string]int, len(order))
	for len(result) < len(items) {
		for _, k := range order {
			i := idx[k]
			if i < len(groups[k]) {
				result = append(result, groups[k][i])
				idx[k] = i + 1
			}
		}
	}
	return result
}
