package resources

import (
	"cmp"
	"slices"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/api/resource"
)

// This is largely taken from sorting in kcp's config helpers.

// DefaultWeights are (hopefully) sensible default weights.
var DefaultWeights = [][]schema.GroupVersionKind{
	// First quotas and limits which may need to apply to later resoures.
	{
		{Kind: "ResourceQuota"},
		{Kind: "LimitRange"},
	},

	// Cluster-scoped resources that may be required by other resources
	{
		{Kind: "CustomResourceDefinition"},
		{Kind: "Namespace"},
	},

	// Then everything else
}

// ObjectWeight returns the weight of the object based on the given weights.
//
//nolint:cyclop // It's not that bad, it's just deeply nested
func ObjectWeight(obj *resource.Resource, weights [][]schema.GroupVersionKind) int {
	for weight, gvks := range weights {
		// Ignoring versions entirely.
		objGvk := obj.GetGvk()
		for _, gvk := range gvks {
			switch {
			case gvk.Group != "" && gvk.Kind != "":
				if gvk.Group == objGvk.Group && gvk.Kind == objGvk.Kind {
					return weight
				}
			case gvk.Group != "":
				if gvk.Group == objGvk.Group {
					return weight
				}
			case gvk.Kind != "":
				if gvk.Kind == objGvk.Kind {
					return weight
				}
			}
		}
	}
	return len(weights)
}

// SortObjectsByWeights sorts the given slice of manifests by the given weights.
func SortObjectsByWeights(objects []*resource.Resource, weights [][]schema.GroupVersionKind) []*resource.Resource {
	copied := slices.Clone(objects)
	slices.SortFunc(copied, func(objA, objB *resource.Resource) int {
		weightA := ObjectWeight(objA, weights)
		weightB := ObjectWeight(objB, weights)
		return cmp.Compare(weightA, weightB)
	})
	return copied
}

// GroupObjectsByWeights sorts the given slice of manifests in groups based on the given weights.
// All manifests with the same weight are placed in one group.
func GroupObjectsByWeights(objects []*resource.Resource, weights [][]schema.GroupVersionKind) [][]*resource.Resource {
	copied := SortObjectsByWeights(objects, weights)
	// maximum capacity is everything in weights + catchall
	groups := make([][]*resource.Resource, 0, len(weights)+1)

	curWeight := -1
	for _, obj := range copied {
		weight := ObjectWeight(obj, weights)
		if weight != curWeight {
			curWeight = weight
			groups = append(groups, []*resource.Resource{})
		}

		groups[len(groups)-1] = append(groups[len(groups)-1], obj)
	}

	return groups
}
